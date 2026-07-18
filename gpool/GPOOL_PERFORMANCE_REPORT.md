# gpool 性能测试报告

## 1. 报告摘要

本报告评估 `gpool` 当前工作区版本在以下两个层面的性能：

1. 连接池内部管理成本：不包含网络 I/O，只测量 `Get/Close`、空闲环形队列、容量计数和 waiter 路径。
2. 真实端到端 RPC：通过 localhost TCP、Apache Thrift Framed Transport 和 Binary Protocol 执行真实 `Example.Add` RPC。

核心结论：

- 连接充足时，gpool 内部 `Get/Close` 约为串行 `89 ns/op`，稳定保持 `0 B/op、0 allocs/op`。
- 对最小 Thrift RPC，连接充足的 gpool 相对每个 worker 独占连接的吞吐成本约为 `3%~4%`。
- 32 并发、32 条池连接时，端到端吞吐约为 `127,322~134,282 QPS`，具体数值因统计方式不同而略有差异。
- 连接不足后，主要成本不再是连接池内部的纳秒级操作，而是等待可用连接：最大 8 条连接时 Get P50 约 `231 µs`；最大 1 条连接时 Get P50 约 `871 µs`。
- 每次 RPC 重新建立 TCP 连接约为池化串行调用的 `3.2 倍`，并产生约 `17 KB、97 allocs/op`，持续高频运行还会耗尽客户端临时 TCP 端口。
- CPU profile 中约 `92%` 的采样落在网络系统调用，gpool 代码自身不是主要 CPU 消耗点。
- 当前结果不支持继续引入 waiter/channel 预分配、连接池分片或无锁化。现阶段更应关注线上 `MaxConns` 配置、Get 等待分位数和连接池运行指标。

## 2. 测试对象

### 2.1 仓库状态

- 仓库：`lgobase`
- 分支：`lanwenhong`
- 基础提交：`8144f5b`
- 测试对象：包含当前工作区尚未提交的 gpool 优化代码
- 测试日期：2026-07-18

### 2.2 已包含的连接池优化

当前被测实现包含：

- 删除 Get/Close 热路径逐请求日志。
- 删除 `UseList`，改为显式 `inUse` 计数。
- 使用 `creating` 和 `closing` 保证 `MaxConns` 是严格容量限制。
- 将真实连接 `Close` 移出全局池锁，并在物理 Close 返回后释放容量。
- 将 `FreeList` 替换为预分配 FIFO 环形队列。
- 使用有界 FIFO waiter 队列，支持 context 取消和准确 timeout。
- 归还连接时优先直接交给最早等待的 waiter。
- 增加 `PoolConn.Close` 原子重复归还保护。
- 增加 `Gpool.Close(ctx)` 完整关闭流程。

相关实现：

- `gpool/gpool.go`
- `gpool/idle_queue.go`
- `gpool/waiter_queue.go`
- `gpool/pool_internal_test.go`
- `gpool/pool_benchmark_test.go`
- `gpool/network_benchmark_test.go`

## 3. 测试环境

| 项目 | 值 |
| --- | --- |
| CPU | Apple M2 |
| 操作系统 | Darwin 25.5.0，arm64 |
| Go | go1.26.4 darwin/arm64 |
| GOMAXPROCS | 8 |
| Thrift | github.com/apache/thrift v0.22.0 |
| pprof | github.com/google/pprof v0.0.0-20260709232956-b9395ee17fa0 |
| 网络 | 同进程 localhost TCP，`127.0.0.1` 随机端口 |
| Transport | Thrift Framed Transport |
| Protocol | Thrift Binary Protocol |

测试过程中关闭业务日志、UUID 生成和逐请求 context 构造。服务端和客户端运行在同一进程，因此结果主要用于比较不同连接管理方式，而不是模拟跨机器生产网络延迟。

## 4. 测试方法

### 4.1 连接池微基准

微基准使用 fake connection，不执行 TCP 或 Thrift 操作，测量：

- 串行 `Gpool.Get/PoolConn.Close`。
- 8 核并行、连接数量充足。
- 8 核并行、池中只有一条连接，强制进入 waiter。

微基准用于定位连接池本身的锁、容器和分配成本，不能代表真实 RPC 延迟。

### 4.2 真实 RPC 服务

复用生成的 `Example` Thrift 服务：

```thrift
service Example {
    i32 add(1: i32 a, 2: i32 b)
}
```

服务端 handler 只执行：

```go
func (*networkBenchmarkService) Add(
    _ context.Context,
    a, b int32,
) (int32, error) {
    return a + b, nil
}
```

服务启动特点：

- 使用 `127.0.0.1:0` 自动选择可用端口。
- 同步完成 `Listen` 后再开始 benchmark，不使用固定 Sleep。
- benchmark 结束后关闭 gpool 和 Thrift server。
- 服务端不打印日志，不生成 UUID，不执行额外业务逻辑。

### 4.3 真实 RPC 场景

| 场景 | 说明 |
| --- | --- |
| Dedicated/Serial | 一条客户端连接串行复用，不执行 gpool Get/Close |
| Pool/Serial | 一条 gpool 连接串行复用 |
| Dedicated/Parallel | 32 个 worker，每个 worker 独占一条连接 |
| Pool/ParallelAmple | 32 个 worker，gpool 最大和空闲连接均为 32 |
| Pool/ParallelMax8 | 32 个 worker，gpool 最大连接数为 8 |
| Pool/ParallelMax1 | 32 个 worker，gpool 最大连接数为 1 |
| DialEach/Serial | 每次请求都执行 TCP Open、RPC、TCP Close |

### 4.4 指标定义

- Go benchmark `ns/op`：并行场景下表示聚合完成一个操作所需的摊销时间，不是单请求延迟。
- QPS：`1s / ns/op`，或固定负载测试中的成功请求数除以墙钟时间。
- Get latency：进入 `pool.Get` 到获得连接的时间。
- RPC latency：获得连接后，到 Thrift RPC 返回的时间。
- Total latency：Get、RPC 和 PoolConn.Close 的总时间。
- B/op、allocs/op：客户端每次成功 RPC 的堆分配。

## 5. 连接池微基准

### 5.1 当前结果

运行三轮，每轮 `benchtime=2s`：

| 场景 | 三轮结果 | 中位数 | 内存分配 |
| --- | --- | ---: | ---: |
| 串行 Get/Close | 88.98 / 90.14 / 89.09 ns | 89.09 ns/op | 0 B，0 allocs |
| 并行、连接充足 | 309.8 / 307.6 / 332.5 ns | 309.8 ns/op | 0 B，0 allocs |
| 并行、单连接饱和 | 658.0 / 644.8 / 662.3 ns | 658.0 ns/op | 424 B，5 allocs |

解释：

- 串行和连接充足的并行路径都没有堆分配。
- 单连接饱和路径的分配来自 waiter、结果 channel 和 timer。
- waiter 分配只发生在池已满且没有可用连接的慢路径。
- 如果生产环境连接长期充足，继续预分配 waiter/channel 对整体性能收益很低。

### 5.2 优化演进

| 实现阶段 | 串行 | 连接充足并行 | 单连接争抢 | 串行分配 |
| --- | ---: | ---: | ---: | ---: |
| 清理日志后，保留 UseList | 约 118 ns | 约 378 ns | 约 509 ns | 96 B，2 allocs |
| 删除 UseList | 约 101 ns | 约 338 ns | 约 453 ns | 48 B，1 alloc |
| 严格 creating/closing 计数 | 约 102 ns | 约 329 ns | 约 459 ns | 48 B，1 alloc |
| FreeList 改为 FIFO 环形队列 | 约 86 ns | 约 304 ns | 约 389 ns | 0 B，0 allocs |
| FIFO waiter、P0 正确性完成 | 约 89 ns | 约 310 ns | 约 658 ns | 0 B，0 allocs |

FIFO waiter 版本在饱和路径上增加了独立结果 channel、timer、取消处理和公平交接，因此饱和微基准慢于旧的空通知机制；换来的能力包括严格 FIFO、有界等待、准确 timeout、context 取消和取消/交付竞态安全。

## 6. 真实 TCP/Thrift 吞吐结果

运行命令使用 `benchtime=2s、count=3`，下表取三轮中位数：

| 场景 | ns/op | 约合 QPS | B/op | allocs/op | 最大打开连接数 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 独占连接，串行 | 25,105 | 39,833 | 694 | 22 | 1 |
| gpool，串行 | 25,993 | 38,472 | 758 | 26 | 1 |
| 独占连接，32 worker | 7,213 | 138,638 | 692 | 22 | 32 |
| gpool 连接充足，32 连接 | 7,447 | 134,282 | 757 | 26 | 32 |
| gpool 最大 8 连接 | 9,669 | 103,423 | 1,182 | 32 | 8 |
| gpool 最大 1 连接 | 28,150 | 35,524 | 1,182 | 32 | 1 |
| 每请求重新建连，串行 | 83,753 | 11,940 | 17,080 | 97 | 1 |

### 6.1 连接充足时的 gpool 成本

串行：

```text
独占连接：25,105 ns/op
gpool：   25,993 ns/op
差值：      888 ns/op，约 +3.5%
```

32 worker 并行：

```text
独占连接：7,213 ns/op
gpool：   7,447 ns/op
差值：      234 ns/op，约 +3.2%
```

这与内部微基准中约 `310 ns/op` 的并行连接池管理成本处于同一数量级。对当前最小 RPC，gpool 不是主要端到端瓶颈。

连接充足时，gpool 调用比独占连接多约 `64~65 B/op、4 allocs/op`。连接池自身微基准仍为零分配；这里的差异属于完整 Thrift 调用路径和 Go 逃逸行为的组合结果。

### 6.2 连接数不足的影响

与 32 条连接的充足场景相比：

- 最大 8 条连接时，聚合操作成本从 `7,447 ns` 上升到 `9,669 ns`，QPS 下降约 `23%`。
- 最大 1 条连接时，聚合操作成本上升到 `28,150 ns`，QPS 下降约 `74%`。
- 受限场景增加约 `425 B/op、6 allocs/op`，与 waiter 慢路径微基准的分配基本吻合。

### 6.3 每请求建连的成本

与 gpool 串行复用一条连接相比：

| 指标 | gpool 串行 | 每请求建连 | 倍数 |
| --- | ---: | ---: | ---: |
| 时间 | 25.99 µs | 83.75 µs | 3.2 倍 |
| 内存 | 758 B/op | 17,080 B/op | 22.5 倍 |
| 分配 | 26 allocs/op | 97 allocs/op | 3.7 倍 |

持续时长型的每请求建连 benchmark 在约四万次请求后出现：

```text
dial tcp 127.0.0.1:<port>: connect: can't assign requested address
```

原因是大量客户端 socket 进入 TIME_WAIT，耗尽本机临时端口。因此正式对照使用固定 `1000x`，而不是按时长无限迭代。这也说明高频 RPC 必须复用连接。

## 7. 32 并发端到端延迟

每个场景运行三轮；每轮 32 个 worker，每个 worker 执行 1,000 次 RPC，总计 32,000 次。下表取三轮中位数：

| 场景 | QPS | Total P50 | Total P95 | Total P99 | Get P50 | Get P95 | Get P99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 独占连接 | 132,955 | 213 µs | 465 µs | 739 µs | 0 | 0 | 0 |
| gpool 连接充足 | 127,322 | 209 µs | 507 µs | 921 µs | 1.9 µs | 16 µs | 98 µs |
| gpool 最大 8 连接 | 101,143 | 304 µs | 417 µs | 578 µs | 231 µs | 314 µs | 439 µs |
| gpool 最大 1 连接 | 35,317 | 899 µs | 1.006 ms | 1.132 ms | 871 µs | 976 µs | 1.101 ms |

### 7.1 连接充足

- gpool QPS 相比独占连接低约 `4.2%`。
- Get P50 只有约 `1.9 µs`。
- Get P99 约 `98 µs`，说明 32 worker 同时竞争全局 mutex 时存在尾延迟，但整体吞吐下降仍只有约 4%。
- Total P50 与独占连接接近，差异处在本机调度波动范围内。

### 7.2 最大 8 条连接

- 32 个 worker 竞争 8 条连接，Get P50 约 `231 µs`，已经成为端到端延迟的主要部分。
- RPC P50 反而下降到约 `66 µs`，因为同时在服务端执行的请求从 32 个被限制为最多 8 个。
- Total P99 低于连接充足场景，这是并发限制带来的削峰效果，不代表吞吐更高；QPS 仍下降约 `21%`。

### 7.3 最大 1 条连接

- 所有请求串行经过同一条连接。
- Get P50 约 `871 µs`，占 Total P50 的约 97%。
- RPC 本身的 P50 约 `24 µs`，绝大多数时间用于等待连接。
- 此场景证明连接池饱和后的主要问题是容量，而不是 waiter 数据结构的几百纳秒管理成本。

## 8. 连接数量与正确性校验

真实 RPC harness 使用：

```text
workers = 8
MaxConns = 2
MaxIdleConns = 2
总 RPC = 200
```

已验证：

- 所有 RPC 返回值正确。
- 实际成功创建连接数不超过 2。
- benchmark 中观测到的最大同时打开连接数分别严格等于配置的 32、8、1。
- 连接充足的稳定 benchmark 期间没有创建或关闭连接。
- `Gpool.Close(ctx)` 完成后，成功创建数与物理关闭数一致。
- waiter、inUse、creating、closing 最终均恢复到 0。
- 真实 RPC harness 使用 race detector 通过。

## 9. pprof 分析

profile 采集场景：

```text
BenchmarkGpoolNetworkRPC/Pool/ParallelAmple
32 worker
32 条池连接
benchtime=3s
```

由于服务端和客户端在同一进程，profile 同时包含两端的运行成本。所有 delay profile 数值都是各 goroutine 阻塞时间之和，不等于墙钟时间，也不能直接与 QPS 相除。

### 9.1 CPU profile

CPU profile：

```text
Duration:      3.40s
Total samples: 18.74s
```

主要 flat CPU：

| 节点 | Flat CPU | 占比 |
| --- | ---: | ---: |
| `syscall.rawsyscalln` | 17.30s | 92.32% |
| `runtime.kevent` | 0.76s | 4.06% |
| `runtime.usleep` | 0.21s | 1.12% |
| `runtime.pthread_cond_wait` | 0.11s | 0.59% |
| `gpool.getConnFromIdle` | 0.01s | 0.05% |

结论：CPU 样本几乎全部消耗在 socket read/write 和事件等待相关的系统调用中。`getConnFromIdle` 自身的 flat CPU 只有约 `0.05%`。

`getConnFromIdle` 的 cumulative CPU 约为 `7.15%`，其中主要来自 `TFramedTransport.IsOpen` 的下游调用，而不是环形队列操作本身。这说明后续若仍要减少锁内工作，优先评审 `IsOpen()` 检查，而不是更换 idle 容器。

### 9.2 Mutex profile

mutex delay 总计约 `10.52s`，主要归因于 `sync.Mutex.Unlock` 所释放的等待者：

| 调用路径 | Cumulative delay |
| --- | ---: |
| `Gpool.Get` | 6.07s |
| `PoolConn.Close/put` | 4.33s |

这证明全局 mutex 在 32 worker 并发下存在竞争。不过端到端 benchmark 中，连接充足 gpool 相对独占连接只下降约 3%~4%，因此 mutex 竞争目前不是引入分片的充分理由。

### 9.3 Block profile

block delay 的主要来源：

| 节点 | Delay 占比 | 解释 |
| --- | ---: | --- |
| `runtime.selectgo` | 84.29% | Thrift server 为每条连接启动的 stop watcher 等待 |
| `sync.Mutex.Lock` | 7.71% | 包含 gpool 和运行时其他 mutex 等待 |
| `runtime.chanrecv1` | 5.37% | 测试和服务关闭阶段的 channel 等待 |
| `sync.WaitGroup.Wait` | 2.63% | benchmark/服务清理等待 |

block profile 主要反映 Thrift server 的连接管理和 benchmark 清理，不代表客户端单次 RPC 延迟。需要分析 gpool 时应使用 `focus` 或 `list` 进一步缩小范围。

示例命令：

```bash
pprof -top /tmp/gpool-network-ample.cpu
pprof -top /tmp/gpool-network-ample.mutex
pprof -top /tmp/gpool-network-ample.block
pprof -list 'Gpool.*Get' /tmp/gpool-network-ample.cpu
```

## 10. 性能判断

### 10.1 当前值得保留的设计

- FIFO idle 环形队列：稳定主路径零分配。
- 单 mutex：实现简单，在真实最小 RPC 下只带来约 3%~4% 吞吐成本。
- FIFO waiter：虽然饱和微基准分配较高，但提供公平性、准确 timeout 和取消安全。
- 严格 creating/closing 计数：保证物理 Close 完成前不释放容量。
- 温和 Pool Close：避免为低频关闭操作重新引入 UseList 或 borrowed registry。

### 10.2 当前不建议实施

#### waiter/channel/timer 预分配

只优化已经饱和的慢路径。真实测试显示连接数不足时等待时间达到数百微秒，而 waiter 自身只有约 658ns 管理成本。降低几次分配无法解决容量不足造成的主要延迟。

#### 连接池分片

分片会增加：

- 全局 MaxConns 协调。
- waiter FIFO 公平性处理。
- 跨分片空闲连接不平衡。
- Pool Close 和连接生命周期复杂度。

真实 RPC 中连接充足 gpool 只比独占连接慢约 3%~4%，当前收益不足以覆盖这些复杂性。

#### 无锁队列

当前 idle 环形队列已经零分配。瓶颈主要在网络系统调用和连接容量，无锁化不会显著改善端到端结果。

## 11. 配置建议

### 11.1 MaxConns

建议根据同时在途 RPC 数量配置，而不是只根据平均 QPS：

```text
预估并发连接数 ≈ 峰值 QPS × RPC P95/P99 延迟
```

再根据服务端容量增加适当余量。不要盲目把 MaxConns 调得很大：本次 MaxConns=8 场景虽然吞吐下降，但服务端在途请求减少后 P99 更稳定，说明连接数还承担客户端限流作用。

### 11.2 MaxIdleConns

- 稳定低延迟优先时，可配置为接近 `MaxConns`，避免归还时物理 Close 和后续重新建连。
- 如果服务实例很多、后端连接预算有限，应根据长期实际并发设置，而不是简单等于 MaxConns。
- 当前初始化会真实打开 `MaxIdleConns` 条连接，不只是分配结构体；配置过大会增加启动时间和瞬时连接压力。

### 11.3 MaxWaiters

- 未配置时默认 `max(1, MaxConns)`。
- 若业务存在短时突发并允许排队，可显式配置为峰值并发与 MaxConns 的差值，并预留余量。
- MaxWaiters 过大只会把拒绝转换成长尾等待，必须与请求 timeout 和业务 SLO 一起设置。

### 11.4 Timeout

- waiter timeout 当前与连接池 `TimeOut` 配置一致。
- 应保证 pool timeout 不超过上层 RPC deadline。
- 线上需要分别统计 waiter timeout、context cancel 和 queue full，避免所有错误都归类为 RPC timeout。

## 12. 建议增加的运行指标

建议提供线程安全的 `Stats()` 快照，以及少量原子累计指标：

```text
当前 idle
当前 inUse
当前 creating
当前 closing
当前 waiters
wait timeout 总数
wait queue full 总数
connection create success/failure
physical close 总数与耗时
direct handoff 总数
Get wait P50/P95/P99
```

这些指标比继续优化微基准更能指导生产配置。特别需要关注：

- Get wait P95/P99 是否持续上升。
- wait queue full 是否出现。
- 创建连接数是否在稳定流量下持续增加。
- closing 是否长期不归零。
- MaxConns 是否经常被完全占用。

## 13. 测试限制

本报告不应直接当作生产容量结论，存在以下限制：

1. 服务端和客户端在同一进程，共享 CPU 和 Go scheduler。
2. 使用 localhost，不包含真实网卡、交换机、跨机延迟、丢包和拥塞。
3. RPC payload 只有两个 int32 和一个 int32 返回值，属于最小请求。
4. 未启用 TLS。
5. 只测试 Framed Transport + Binary Protocol。
6. benchmark 使用类型化 client 调用，没有包含反射版 `ThriftCall` 和业务日志。
7. 没有混入服务端业务逻辑、数据库或缓存访问。
8. 服务主动断开、重启和丢包属于正确性/恢复测试，尚未计入稳定性能数字。
9. pprof 同时包含同进程的客户端和服务端，不能直接视为客户端独立 profile。

如果需要更接近生产环境，应在两台机器上使用真实服务协议、payload、TLS 和日志策略重复测试。

## 14. 复现命令

### 14.1 连接池微基准

```bash
go test ./gpool \
  -run '^$' \
  -bench '^BenchmarkGpoolGetClose$' \
  -benchmem \
  -benchtime=2s \
  -count=3
```

### 14.2 真实 RPC 正确性夹具

```bash
go test -tags=performance ./gpool \
  -run '^TestGpoolNetworkRPCHarness$' \
  -count=1

go test -race -tags=performance ./gpool \
  -run '^TestGpoolNetworkRPCHarness$' \
  -count=1
```

### 14.3 真实 RPC 吞吐

```bash
go test -tags=performance ./gpool \
  -run '^$' \
  -bench '^BenchmarkGpoolNetworkRPC$' \
  -benchmem \
  -benchtime=2s \
  -count=3
```

### 14.4 每请求重新建连

必须使用固定次数，避免耗尽临时 TCP 端口：

```bash
go test -tags=performance ./gpool \
  -run '^$' \
  -bench '^BenchmarkGpoolNetworkRPCDialEach$' \
  -benchmem \
  -benchtime=1000x \
  -count=3
```

### 14.5 固定并发延迟

```bash
go test -tags=performance ./gpool \
  -run '^TestGpoolNetworkRPCLatency$' \
  -count=3 \
  -v
```

### 14.6 Profile 采集

```bash
go test -tags=performance ./gpool \
  -run '^$' \
  -bench '^BenchmarkGpoolNetworkRPC/Pool/ParallelAmple$' \
  -benchtime=3s \
  -count=1 \
  -cpuprofile=/tmp/gpool-network-ample.cpu \
  -mutexprofile=/tmp/gpool-network-ample.mutex \
  -blockprofile=/tmp/gpool-network-ample.block

pprof -top /tmp/gpool-network-ample.cpu
pprof -top /tmp/gpool-network-ample.mutex
pprof -top /tmp/gpool-network-ample.block
```

## 15. 最终结论

当前 gpool 的连接充足路径已经达到零分配，真实最小 Thrift RPC 中只引入约 3%~4% 的吞吐成本。CPU profile 显示绝大多数时间消耗在网络系统调用，而不是连接池队列或状态计数。

当 MaxConns 低于同时在途请求数时，Get 等待迅速上升到数百微秒甚至毫秒级。此时优化 waiter 的几百纳秒管理成本不能改变主要矛盾，应优先调整连接容量、请求并发和服务端承载策略。

因此当前推荐：

1. 保持现有单 mutex、FIFO idle 环形队列和 FIFO waiter 设计。
2. 暂不实施 waiter/channel 预分配、分片或无锁化。
3. 增加 Stats 和线上 Get 等待分位数指标。
4. 根据峰值同时在途 RPC，而不是平均 QPS，配置 MaxConns。
5. 在真实服务协议、payload 和跨机环境中进行下一轮容量测试。
