# WorkPool 性能测试报告

测试日期：2026-07-23

## 结论摘要

1. 当前 `WorkPool` 更适合作为“有界并发与生命周期管理”组件，而不是极小 no-op 任务的最低开销调度器。固定 `request_id`、容量充足时，一次并发提交、执行和等待约为 **1.46 µs/op、599 B/op、11 allocs/op**。
2. 对 no-op 任务，单 worker 完整 WorkPool 路径约为 **1.15 µs/op、605 B/op、12 allocs/op**；串行创建一个 goroutine 并等待约为 **468 ns/op、224 B/op、4 allocs/op**。WorkPool 付出的额外成本来自有界队列、Task 状态、context 合并、关闭传播、panic 隔离和结果广播，不能只按 goroutine 创建成本理解。
3. 对约 7.13 µs 的 CPU4096 合成任务，WorkPool 的调度成本占比下降。批量吞吐从 1 worker 的 **10.42 µs/op** 改善到 8 worker 的 **4.58 µs/op**、16 worker 的 **3.78 µs/op**。扩展不是线性的，worker 数超过 CPU/负载需要后不会继续等比例收益。
4. 队列不足时不应对 `ErrPoolFull` 热自旋。8 路并发、1 worker 时，每个成功 no-op 任务平均经历 **6.46 次拒绝**，成本上升到 **2.35 µs/op、2.15 KB/op、31 allocs/op**；CPU4096 延迟 profile 中达到约 **38.26 次拒绝/成功任务**。
5. `AddTask` 已接受快路径本身约为 **92.68 ns/op、243 B/op、3 allocs/op**。缺少 `request_id` 时生成 UUID，成本上升到 **466.7 ns/op、339 B/op、6 allocs/op**，约为固定 ID 的 **5.0 倍**。
6. 已完成 Task 的 `WaitRet` 约为 **5.97 ns/op、0 allocs/op**；等待 context 已取消的返回路径约为 **56.36 ns/op、0 allocs/op**。结果广播本身不是瓶颈。
7. 分配 profile 显示约 **72% 的分配对象来自任务执行阶段**，主要是 `context.WithValue`、`context.WithCancel` 和 `context.AfterFunc`；`AddTask` 约占 27%。如果生产负载以微小任务为主，context 组合是第一优化目标。

## 测试对象

本报告测试当前工作区中的新版 `workpool`：

- 使用有界 `chan *Task` 调度，不再依赖 CAS Queue。
- worker 数和默认队列容量都由 `NewWorkPool(poolSize)` 的 `poolSize` 决定。
- `AddTask` 队列满时立即返回 `ErrPoolFull`。
- Task 支持结果、错误、context 等待和多等待者广播。
- worker 支持 panic 隔离、池关闭传播和幂等生命周期。
- 提交请求取消不会取消已经接受的异步任务；Pool 关闭会取消运行中任务。

相关文件：

- `workpool/pool.go`
- `workpool/pool_test.go`
- `workpool/benchmark_test.go`

## 测试环境

| 项目 | 值 |
|---|---|
| 操作系统 | macOS 26.5.2，darwin/arm64 |
| CPU | Apple M2 |
| Go | go1.26.4 |
| `GOMAXPROCS` | 8，benchmark 名称后缀为 `-8` |
| 分支 | `lanwenhong` |
| 基础提交 | `4a7d7b2`，测试对象包含当前未提交的 WorkPool 改动 |
| 日志 | benchmark 中关闭 `lgobase/logger`，不计日志 I/O |

正式 benchmark 使用：

```text
-benchtime=500ms -count=5 -benchmem
```

表格取五轮 `ns/op` 的中位数。延迟 profile 每种配置运行三轮，每轮 8 个 producer、每个 producer 10,000 个任务，共 80,000 个成功任务；表格对每一列分别取三轮中位数。

## 测试方法

### DirectProcess

调用相同的 `Process` 函数，不创建 goroutine、不进入 WorkPool，用作业务计算下限。

### GoroutinePerTask

每次创建一个 goroutine，通过一个有缓冲 channel 等待结果。此 case 是串行“创建并等待”，用于比较单任务调度成本，不代表允许无限 goroutine 的生产设计。

### WorkPoolBatch

创建固定数量 worker，每批最多提交与 worker 数相同的任务，再等待整批完成。这样不会触发 `ErrPoolFull`，测量容量充足时的提交、调度、执行和结果等待总成本。

### ParallelRoundTrip

使用 `testing.B.RunParallel`，本机为 8 路并发。每个并发调用提交一个 no-op 任务并等待结果；队列满时执行 `runtime.Gosched()` 后重试，同时统计 `rejects/op`。该 case 模拟同步调用方在快速失败 API 上重试的行为。

### LatencyProfile

固定 8 个 producer，每个 producer 串行提交并等待自己的任务，记录从首次尝试提交到 Task 完成的端到端延迟。队列满时 `Gosched` 重试，因此延迟包含拒绝和重新提交成本。

### CPU4096

每个任务执行 4,096 轮整数移位与异或运算。该工作负载在直接调用时约为 7.13 µs/op，用于观察任务执行时间增加后 worker 的并发扩展。

## 基础调度成本

| 工作负载 | 执行方式 | ns/op | B/op | allocs/op |
|---|---|---:|---:|---:|
| NoOp | 直接调用 | 8.676 | 8 | 0 |
| NoOp | 每任务一个 goroutine，串行等待 | 468.3 | 224 | 4 |
| NoOp | WorkPool，1 worker | 1,151 | 605 | 12 |
| CPU4096 | 直接调用 | 7,130 | 15 | 1 |
| CPU4096 | 每任务一个 goroutine，串行等待 | 10,755 | 231 | 5 |
| CPU4096 | WorkPool，1 worker | 10,422 | 615 | 13 |

No-op 场景主要在测调度框架本身，因此 WorkPool 比单次 goroutine 创建慢约 2.46 倍，并多出 8 次分配。这个结果不表示生产代码应改成无限制地启动 goroutine：WorkPool 提供的是并行上限、队列上限、关闭、错误返回、panic 隔离和等待广播。

CPU4096 场景下，业务计算已经占据主要时间；WorkPool 单 worker 与每任务 goroutine 的差距缩小到约 3%。任务越重，框架的固定成本占比越低。

## Worker 数与批量吞吐

`ns/op` 是聚合完成一个任务的摊销时间，值越低越好。

### NoOp

| Workers | ns/op | 约合 tasks/s | B/op | allocs/op |
|---:|---:|---:|---:|---:|
| 1 | 1,151 | 868,800 | 605 | 12 |
| 2 | 1,298 | 770,400 | 606 | 12 |
| 4 | 1,150 | 869,600 | 606 | 12 |
| 8 | 1,064 | 939,800 | 607 | 12 |
| 16 | 1,129 | 885,700 | 607 | 12 |

No-op 任务没有可并行的有效计算，worker 增加主要改变 goroutine 唤醒和 channel 调度。8 worker 在本机略优，16 worker 开始回退。微小任务不应通过盲目增加 worker 来优化。

### CPU4096

| Workers | ns/op | 约合 tasks/s | 相对 1 worker | B/op | allocs/op |
|---:|---:|---:|---:|---:|---:|
| 1 | 10,422 | 95,950 | 1.00× | 615 | 13 |
| 2 | 10,124 | 98,780 | 1.03× | 616 | 13 |
| 4 | 7,854 | 127,320 | 1.33× | 616 | 13 |
| 8 | 4,580 | 218,340 | 2.28× | 616 | 13 |
| 16 | 3,784 | 264,270 | 2.75× | 616 | 13 |

CPU 任务能够从多个 worker 获益，但扩展明显低于线性：任务仍需经过 AddTask 锁、channel、context 构造、完成 channel 和 runtime 调度。本机为异构 Apple M2，worker 数和物理核心性能并非简单一一对应。

## 并发 Round-Trip 与队列饱和

8 路并发 no-op 任务：

| Workers / Queue | ns/op | 约合 tasks/s | rejects/op | B/op | allocs/op |
|---:|---:|---:|---:|---:|---:|
| 1 / 1 | 2,351 | 425,350 | 6.460 | 2,150 | 31 |
| 8 / 8 | 1,455 | 687,290 | 0 | 599 | 11 |
| 32 / 32 | 1,535 | 651,470 | 0 | 599 | 11 |

结论：

- worker 与并发度匹配时没有拒绝，吞吐最好。
- 只有 1 个 worker 时，调用方热重试使每个成功任务额外产生约 6.46 个被拒绝 Task；时间增加约 62%，内存增加到 3.59 倍，分配次数增加到 2.82 倍。
- 从 8 worker 增加到 32 worker 没有收益，反而慢约 5.5%。容量应按实际并发和任务类型配置，不应以“越大越安全”为原则。

## AddTask 与 WaitRet 微基准

### AddTask

| 路径 | ns/op | B/op | allocs/op | 说明 |
|---|---:|---:|---:|---|
| 接受，已有 request_id | 92.68 | 243 | 3 | 不启动 worker，只测成功入队 |
| 接受，自动生成 request_id | 466.7 | 339 | 6 | 包含 UUID 生成 |
| 队列满拒绝 | 78.82 | 240 | 3 | 当前在容量判断前已经创建 Task 和完成 channel |

自动生成 request_id 比复用现有 ID 慢约 5.0 倍，增加 96 B 和 3 次分配。生产请求通常应在入口建立 request_id 并沿 context 传递。

队列满虽然返回很快，但仍分配约 240 B、3 个对象。这解释了热重试场景的分配放大。后续若优化，可在保证 AddTask/Kill 原子边界的前提下先预留队列容量，再创建 Task；不能用一次不受保护的 `len(channel)` 判断代替。

### WaitRet

| 路径 | ns/op | B/op | allocs/op |
|---|---:|---:|---:|
| Task 已完成 | 5.973 | 0 | 0 |
| Task 未完成，等待 context 已取消 | 56.36 | 0 | 0 |

完成 channel 的关闭广播支持多个等待者，同时保持零分配；当前无需优化 `WaitRet`。

## 固定负载延迟分位数

每种配置为 8 producer、80,000 个成功任务。延迟包括队列满重试、入队、执行和 `WaitRet`。

### NoOp

| Workers | QPS | rejects/op | p50 | p95 | p99 |
|---:|---:|---:|---:|---:|---:|
| 1 | 416,343 | 6.0132 | 8.417 µs | 69.791 µs | 125.208 µs |
| 4 | 874,545 | 0.0126 | 3.541 µs | 33.125 µs | 57.333 µs |
| 8 | 653,383 | 0 | 2.250 µs | 55.917 µs | 117.791 µs |

### CPU4096

| Workers | QPS | rejects/op | p50 | p95 | p99 |
|---:|---:|---:|---:|---:|---:|
| 1 | 85,588 | 38.2587 | 71.917 µs | 227.875 µs | 352.792 µs |
| 4 | 339,847 | 0.1941 | 20.167 µs | 54.333 µs | 94.708 µs |
| 8 | 310,685 | 0 | 10.292 µs | 93.125 µs | 175.458 µs |

观察：

- 1 worker 下重试次数与尾延迟同时放大，CPU4096 每个成功任务约发生 38 次拒绝。
- 8 worker 没有队列拒绝且 p50 最低，但本机 p95/p99 和聚合 QPS 不如 4 worker。8 个 producer、8 个 worker 会让更多 goroutine 在完成与再次提交之间频繁切换；4 worker 的有界排队反而平滑了调度。
- 该结果说明“无拒绝”不等于“尾延迟最优”。最终 worker 数应在目标机器上用真实任务测试。

## Pool 生命周期成本

以下包含 `NewWorkPool + Run + Kill + Join`，不执行任务：

| Workers | ns/op | B/op | allocs/op |
|---:|---:|---:|---:|
| 1 | 1,400 | 856 | 18 |
| 8 | 11,673 | 2,333 | 60 |
| 32 | 39,461 | 7,202 | 205 |

生命周期成本随 worker 数近似增长。WorkPool 应作为长生命周期组件复用，不应为每个请求创建和销毁。即便 32 worker 初始化关闭只有约 39.5 µs，它也会产生约 7.2 KB 和 205 次分配，并触发大量 goroutine 调度。

## CPU 与内存 Profile

Profile 场景：`ParallelRoundTrip/Workers8`，持续 3 秒，结果约为 1.538 µs/op、599 B/op、11 allocs/op。

CPU flat sample 主要落在 runtime：

| 热点 | flat 占比 |
|---|---:|
| `runtime.usleep` | 37.91% |
| `runtime.pthread_cond_wait` | 34.11% |
| `runtime.pthread_cond_signal` | 18.53% |

三者合计约 90.5%，说明 no-op 场景主要受 goroutine 休眠、唤醒和 select 调度影响。WorkPool 自身函数的 flat CPU 占比较低，因此当前数据不支持为了 no-op 微基准引入复杂无锁结构。

按分配对象数统计：

| 分配热点 | flat 占比 | cumulative 占比 |
|---|---:|---:|
| `context.WithValue` | 18.52% | 18.52% |
| `WorkPool.AddTask` | 18.04% | 27.46% |
| `context.AfterFunc` | 17.40% | 17.40% |
| `WorkPool.execute` | 11.27% | 72.40% |
| `context.WithoutCancel` | 9.42% | 9.42% |
| `context.withCancel` | 9.19% | 9.19% |

按分配字节统计，`AddTask` cumulative 约占 40.0%，`execute` cumulative 约占 59.6%。执行阶段为每个任务组合请求 value、pool 取消信号和 request_id，是当前主要分配来源。

## 配置与使用建议

1. **不要热自旋重试 `ErrPoolFull`**：快速失败后应选择立即返回、降级、统计丢弃，或者使用有界退避。后台关键任务不能忽略 AddTask error。
2. **worker 数按任务类型设置**：CPU 密集任务可从 `GOMAXPROCS` 附近开始压测；I/O 等待型任务可以更高，但必须同时观察服务端容量、超时和尾延迟。
3. **队列容量应与 worker 数解耦**：当前兼容构造器让两者都等于 `poolSize`。后续可以增量增加 `NewWorkPoolWithOptions(workerSize, queueSize)`，保留现有 `NewWorkPool(int)` 行为。
4. **入口统一生成 request_id**：避免每次 AddTask 单独生成 UUID，也保证跨组件日志关联一致。
5. **WorkPool 长生命周期复用**：在服务启动时 Run，服务关闭时 Kill/Join，不要按请求创建。
6. **监控指标**：至少记录接受数、`ErrPoolFull` 数、队列深度、运行任务数、任务耗时 p95/p99、panic 数和关闭时取消任务数。

## 后续优化优先级

### P1：降低队列满路径分配

当前拒绝前已经创建 Task、完成 channel 和 context wrapper。可以设计原子 admission reservation，在确认容量后再构造 Task。必须继续保证：

- AddTask 与 Kill 不能交错产生关闭后滞留任务；
- 并发提交不能突破 QueueSize；
- reservation 失败或提交失败时正确归还容量。

### P1：减少每任务 context 组合

当前执行阶段使用 `WithValue + WithCancel + AfterFunc` 合并提交上下文 value 与 pool 取消。这部分约占 72% 的分配对象。可评估一个只读组合 context：

- `Deadline/Done/Err` 委托给 pool context；
- `Value` 优先读取 Task/request context；
- request_id 在 AddTask 时一次性固化。

优化必须保持请求结束不取消已接受后台任务、Pool Kill 能取消运行任务的现有生产语义。

### P2：独立 QueueSize

延迟 profile 中 4 worker + 有界排队优于 8 worker，说明执行并行度与吸收瞬时突发的队列深度不是同一个参数。建议用新增构造器或 Options 扩展，不修改现有生产调用。

### 暂不建议

- 为 no-op benchmark 改回自制无锁 CAS Queue；runtime 调度与 context 分配才是当前主要成本。
- 直接使用 `sync.Pool` 复用 Task；Task 支持多等待者，生命周期边界复杂，应先完成 admission 和 context 优化。
- 无限扩大 worker 或 queue；这会隐藏上游过载并放大关闭耗时和尾延迟。

## 可复现命令

```bash
# 编译性能测试
go test -tags=performance ./workpool -run '^$'

# 完整微基准
go test -tags=performance ./workpool -run '^$' \
  -bench '^Benchmark' -benchmem -benchtime=500ms -count=5

# 固定负载 QPS 与延迟分位数
go test -tags=performance ./workpool \
  -run '^TestWorkPoolLatencyProfile$' -count=3 -v

# CPU 和分配 profile
go test -tags=performance ./workpool -run '^$' \
  -bench '^BenchmarkWorkPoolParallelRoundTrip/Workers8$' \
  -benchtime=3s -count=1 \
  -cpuprofile=/tmp/workpool.cpu -memprofile=/tmp/workpool.mem

go tool pprof -top /tmp/workpool.cpu
go tool pprof -top -alloc_objects /tmp/workpool.mem
go tool pprof -top -alloc_space /tmp/workpool.mem

# 性能测试代码的竞态验证
go test -race -tags=performance ./workpool \
  -run '^TestWorkPoolLatencyProfile$' -count=1
```

## 适用范围与限制

- 这是单机微基准，不包含网络、Redis、数据库、磁盘和真实业务日志。
- benchmark 关闭了 lgobase 日志。生产环境开启高频 Debug/Info 会增加时间和分配。
- CPU4096 是合成整数计算，不能替代真实任务的锁竞争、缓存局部性和 I/O 等待。
- Go benchmark 的并发 `ns/op` 是聚合吞吐摊销值，不等于单任务延迟；单任务延迟应看 profile 的 p50/p95/p99。
- Apple M2 是异构 CPU，worker 扩展结果不能直接外推到 Linux x86 或服务器 ARM。
- 绝对值会随 Go 版本、`GOMAXPROCS`、电源状态和后台负载变化；配置决策应在目标生产机器上复测。
