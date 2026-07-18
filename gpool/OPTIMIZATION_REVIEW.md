# gpool 连接池优化方案（逐项评审稿）

## 1. 文档目的

本文档只描述 `gpool` 连接分配、归还、等待、创建和清理路径的优化思路，不代表所有方案都会直接实施。

建议按本文的编号逐项评审。每一项审核通过后再单独实现、测试和提交，避免一次改动过大，难以判断性能收益或回归来源。

当前建议的评审顺序：

1. 连接状态与容量不变量
2. 并发创建连接的容量预留
3. 归还连接的幂等保护
4. 等待、超时和 context 取消
5. 移除 `UseList` 和链表节点分配
6. 将连接直接交给等待者
7. 将关闭连接移出全局锁
8. 清理连接和连接池关闭机制
9. 生命周期检查方式
10. 分片连接池（可选）
11. 指标与日志策略

## 2. 当前范围和基准

### 2.1 本轮关注范围

本轮主要关注连接池自身的管理成本：

- 获取连接：`Gpool.Get`
- 归还连接：`PoolConn.Close` / `PoolConn.put`
- 空闲连接与使用中连接的状态转换
- 达到 `MaxConns` 后的等待和通知
- 动态创建连接时的容量控制
- 超过 `MaxIdleConns` 后的清理
- 连接生命周期与空闲生命周期检查

暂时不把以下成本混入连接池微基准：

- TCP 连接建立和网络读写
- Thrift 编解码
- 服务端处理耗时
- 业务回调和业务日志
- 网络延迟、丢包和服务端排队

网络 I/O 应通过单独的本机 loopback 端到端基准测量，不能替代连接池微基准。

### 2.2 当前微基准结果

环境：Apple M2、darwin/arm64、Go 1.26.4、`GOMAXPROCS=8`。

热路径日志清理后、删除 `UseList` 前的结果：

| 场景 | 耗时 | 内存分配 |
| --- | ---: | ---: |
| 串行 Get/Close | 约 118 ns/op | 96 B/op，2 allocs/op |
| 8 核、连接充足 | 约 378 ns/op | 96 B/op，2 allocs/op |
| 8 核、争抢 1 个连接 | 约 509 ns/op | 约 180 B/op，3 allocs/op |

删除 `UseList`、改用 `inUse` 计数后的结果：

| 场景 | 耗时 | 内存分配 |
| --- | ---: | ---: |
| 串行 Get/Close | 约 101 ns/op | 48 B/op，1 alloc/op |
| 8 核、连接充足 | 约 338 ns/op | 48 B/op，1 alloc/op |
| 8 核、争抢 1 个连接 | 约 453 ns/op | 约 126 B/op，1 alloc/op |

加入 `creating/closing` 严格容量计数和同步 Close 后的结果：

| 场景 | 耗时 | 内存分配 |
| --- | ---: | ---: |
| 串行 Get/Close | 约 102 ns/op | 48 B/op，1 alloc/op |
| 8 核、连接充足 | 约 329 ns/op | 48 B/op，1 alloc/op |
| 8 核、争抢 1 个连接 | 约 459 ns/op | 约 126 B/op，1 alloc/op |

将 `FreeList` 替换为预分配 FIFO 环形队列后的结果：

| 场景 | 耗时 | 内存分配 |
| --- | ---: | ---: |
| 串行 Get/Close | 约 86 ns/op | 0 B/op，0 allocs/op |
| 8 核、连接充足 | 约 304 ns/op | 0 B/op，0 allocs/op |
| 8 核、争抢 1 个连接 | 约 389 ns/op | 约 71 B/op，低于 1 alloc/op |

连接充足的稳定 Get/Put 已经达到零堆分配。单连接争抢场景的剩余分配来自尚未改造的等待/timer 路径。

加入有界 FIFO waiter 队列、context/timeout 取消和连接直接交接后的结果：

| 场景 | 耗时 | 内存分配 |
| --- | ---: | ---: |
| 串行 Get/Close | 约 89 ns/op | 0 B/op，0 allocs/op |
| 8 核、连接充足 | 约 310 ns/op | 0 B/op，0 allocs/op |
| 8 核、争抢 1 个连接 | 约 655 ns/op | 约 424 B/op，5 allocs/op |

连接充足的主路径仍然保持零堆分配。饱和场景现在提供严格 FIFO、公平交接、有界等待、准确超时和 context 取消，但每个慢路径请求会创建 waiter、结果 channel 和 timer，因此耗时与分配高于旧的空通知方案。后续如果需要继续优化，应单独评审 waiter 安全复用或预分配，避免复用残留结果造成串线。

### 2.3 当前实现的核心问题

#### 性能问题

- 所有连接借还都竞争同一个 `Gpool.mutex`。
- 空闲连接已经使用预分配 FIFO 环形队列，正常 Get/Put 不再产生链表节点分配。
- waiter 慢路径需要创建结果 channel 和 timer。
- 清理路径在全局锁内调用真实连接的 `Close`。
- 连接有效性和生命周期在每次 Get 时检查。

#### 正确性问题

- 创建连接已经通过 `creating` 预留容量，不再允许并发创建突破 `MaxConns`。
- waiter timeout 已统一使用连接池配置的毫秒值。
- 等待过程已经监听 `ctx.Done()`。
- `PoolConn.Close` 已增加原子状态转换，同一次借用只能成功归还一次。
- 清理路径已经改为锁外同步 Close，并删除了重复 Close 的后台清理 goroutine。
- 已增加 `Gpool.Close(ctx)`，统一处理 idle、borrowed、creating、closing 和 waiter。

## 3. 总体设计原则

后续优化应持续满足以下原则：

1. 正确性优先于 benchmark 数字。
2. `MaxConns` 必须是并发环境下的严格约束，而不是近似值。
3. 稳定的 Get/Put 路径应做到零堆分配或接近零堆分配。
4. 网络系统调用不能在全局池锁内执行。
5. 等待必须支持 timeout 和 context 取消。
6. 一个借出的连接只能成功归还一次。
7. 异常路径可以稍慢，正常路径必须简单。
8. 不通过逐请求日志观察池状态，改用原子指标和按需快照。
9. 每一阶段都应可独立测试、独立 benchmark、独立回滚。

## 4. 建议的连接状态模型

### 4.1 状态定义

建议明确区分以下状态：

```text
creating -> borrowed -> idle -> borrowed
                    \-> closing -> closed
idle ----------------> closing -> closed
```

建议状态含义：

- `creating`：已经占用容量槽位，连接工厂仍在执行。
- `borrowed`：连接已经交给调用者。
- `idle`：连接在空闲容器中，可再次借出。
- `closing`：已经从可用集合移除，正在执行网络关闭。
- `closed`：不再属于连接池。

### 4.2 建议的不变量

实现和测试都应围绕以下不变量：

```text
capacityUsed = creating + borrowed + idle + closing
0 <= capacityUsed <= MaxConns
0 <= idle <= MaxIdleConns
```

补充约束：

- 同一个底层连接不能同时处于两个状态。
- 同一个连接不能同时出现在两个容器中。
- `Get` 成功返回时，连接必须处于 `borrowed`。
- `Close/Put` 成功后，连接只能进入 `idle` 或 `closing`。
- 连接工厂失败时，必须释放之前预留的容量槽位。
- 物理 Close 完成后才能从 `closing` 转为 `closed` 并释放严格容量槽位。

是否把 `closing` 计入 `MaxConns` 需要明确决定。

推荐计入。这样可以保证连接正在关闭时不会立即创建替代连接，从而在慢 Close 场景下仍严格限制文件描述符数量。代价是连接关闭期间可用容量恢复稍慢。

## 5. 评审项一：连接状态和容量计数

### 5.1 目标

本项的第一小步已经完成：当前使用

```go
idle.Len() + inUse
```

替代原来的 `FreeList.Len() + UseList.Len()`，`UseList` 已删除。

完整目标仍然是引入明确的容量字段，不再仅通过容器长度间接表示连接总数。

建议字段概念：

```go
capacityUsed int
idleCount   int
inUseCount  int
creating    int
closing     int
```

第一版不一定需要全部作为独立字段保存。可以只保存必要计数，其他值通过已保护的状态推导，但命名和含义必须统一。

### 5.2 收益

- 为严格执行 `MaxConns` 提供基础。
- `UseList` 已移除，稳定借还减少一次链表节点分配。
- 状态统计不再依赖遍历或读取容器长度。
- 更容易写并发不变量测试。

### 5.3 风险

- 多个计数更新顺序错误可能导致永久占用容量。
- 创建失败、关闭失败和 context 取消都必须覆盖计数回滚。
- 如果同时保留旧链表和新计数，过渡期可能出现两套状态不一致。

### 5.4 建议实施方式

当前已删除 `UseList` 和 `FreeList`，使用预分配 idle 环形队列及 mutex 保护的 `inUse/creating/closing` 计数。

### 5.5 验收标准

- 并发测试期间 `capacityUsed` 从不小于 0。
- 并发测试期间 `capacityUsed` 从不大于 `MaxConns`。
- 工厂失败后容量可以继续被其他 Get 使用。
- 重连、过期清理、等待超时后计数不会泄漏。

### 5.6 评审结论

- [x] 接受并完成 `inUse` 第一小步
- [x] 接受并完成 `creating/closing` 容量状态
- [ ] 修改后接受
- [ ] 暂不实施
- 评审备注：

## 6. 评审项二：并发创建的容量预留

### 6.1 当前问题

修改前的流程大致为：

```text
加锁检查连接数量
发现没有达到 MaxConns
解锁
创建网络连接
重新加锁并增加 inUse
```

多个 goroutine 可以同时在解锁前看到容量未满，然后同时创建连接，最终超过 `MaxConns`。

当前已经在解锁执行连接工厂前增加 `creating` 预留，并使用 `idle + inUse + creating + closing` 参与 `MaxConns` 判断。

### 6.2 建议流程

```text
加锁
如果 capacityUsed < MaxConns：
    立即预留一个 creating 槽位
解锁
执行连接工厂
加锁
如果成功：creating -> borrowed
如果失败：释放槽位
解锁
```

伪代码：

```go
gp.mu.Lock()
if gp.capacityUsed >= gp.maxConns {
    gp.mu.Unlock()
    return gp.wait(ctx)
}
gp.capacityUsed++
gp.creating++
gp.mu.Unlock()

conn, err := gp.create(ctx)

gp.mu.Lock()
gp.creating--
if err != nil {
    gp.capacityUsed--
    gp.signalCapacityAvailable()
    gp.mu.Unlock()
    return nil, err
}
gp.inUseCount++
gp.mu.Unlock()
return conn, nil
```

### 6.3 需要覆盖的异常

- 连接工厂返回错误。
- 连接工厂返回 `(nil, nil)`。
- context 在拨号期间取消。
- 创建成功后连接立即被判断为不可用。
- 连接池在创建期间被关闭。

### 6.4 验收测试

使用一个可阻塞的 fake factory：

1. `MaxConns=4`。
2. 同时启动 100 个 Get。
3. 阻塞所有创建过程。
4. 断言实际进入 factory 的并发数不超过 4。
5. 放开 factory 后归还全部连接。
6. 断言所有状态计数恢复一致。

### 6.5 评审结论

- [x] 接受并完成
- [ ] 修改后接受
- [ ] 暂不实施
- 评审备注：

## 7. 评审项三：归还操作幂等化

### 7.1 修改前的问题

`PoolConn.Close` 当前可以被调用多次。

删除 `UseList` 后，第二次归还会再次执行 `inUse--` 和 `idle.Push`，仍可能造成：

- 同一个底层连接在空闲容器中出现多次。
- 两个调用者同时拿到同一个连接。
- 链表长度和真实连接数不一致。
- 清理时重复关闭同一个连接。

### 7.2 当前方案

给每次借出增加明确状态或 generation。

低风险版本：

```go
const (
    connIdle uint32 = iota
    connBorrowed
    connClosing
    connClosed
)
```

归还时执行 CAS：

```go
if !atomic.CompareAndSwapUint32(&pc.state, connBorrowed, connIdle) {
    return ErrAlreadyReturned
}
```

当前实际状态为 `idle/borrowed/returning/closing/closed`。`Close(ctx)` 通过 CAS 将 `borrowed` 转换为 `returning`，失败时静默返回，以保持现有无返回值 API 兼容；随后根据归还结果进入 `idle`、再次直接交接为 `borrowed`，或者进入 `closing/closed`。

该保护基于连接所有权约定：调用方执行 `Close` 后不能继续使用该 `PoolConn`；状态会在连接下一次合法借出时重新变为 `borrowed`。

### 7.3 API 行为需要评审

重复 Close 有三种可选行为：

1. 返回明确错误。
2. 静默忽略，保持幂等。
3. debug 构建 panic，生产环境返回错误。

推荐第 1 种，但当前 `PoolConn.Close(ctx)` 没有返回值。如果保持 API 兼容，可以内部静默忽略并增加原子计数指标；下一主版本再考虑返回错误。

### 7.4 验收测试

- [x] 同一个连接连续 Close 两次，只能进入空闲池一次。
- [x] 100 个 goroutine 同时 Close 同一个连接，只能一个成功归还。
- [x] 重复 Close 后容量与 idle/inUse 计数保持一致。
- [x] race detector 通过。

### 7.5 评审结论

- [x] 接受并完成
- [ ] 修改后接受
- [ ] 暂不实施
- 评审备注：保持 `PoolConn.Close(ctx)` 签名不变，重复归还静默忽略。

## 8. 评审项四：等待、超时和 context 取消

### 8.1 修改前的问题

- 每次循环使用 `time.After`，产生新的 timer。
- 剩余超时计算把毫秒数直接转换成 `time.Duration`，实际按纳秒解释。
- 不监听 `ctx.Done()`。
- WaitNotify 只表示“可能有连接”，不携带连接。
- 通知可能被新进入的 Get 抢占，等待者不一定公平。
- channel 满时通知被丢弃，依赖调用者重新检查状态。

### 8.2 最小修复方案

在不重做容器前，先使用单个 timer 和绝对 deadline：

```go
deadline := time.Now().Add(timeout)
timer := time.NewTimer(timeout)
defer timer.Stop()

for {
    select {
    case <-ctx.Done():
        return nil, ctx.Err()
    case <-timer.C:
        return nil, ErrPoolTimeout
    case <-gp.waitNotify:
        // 加锁并重新检查条件
    }

    remaining := time.Until(deadline)
    if remaining <= 0 {
        return nil, ErrPoolTimeout
    }
    resetTimer(timer, remaining)
}
```

### 8.3 推荐的最终方案

后续改为“等待连接结果”而不是“等待空通知”：

```go
type waitResult[T any] struct {
    conn *PoolConn[T]
    err  error
}
```

每个 waiter 拥有一个容量为 1 的结果 channel。Put 优先把连接直接交给队首 waiter；没有 waiter 时才进入空闲容器。

等待队列只在池已满时使用，因此允许慢路径产生少量分配。正常有空闲连接的路径仍应零分配。

当前已按此方案实现：

- waiter 使用侵入式 FIFO 双向队列，队列节点就是 waiter 本身，取消时可以 O(1) 删除。
- 每个 waiter 使用容量为 1 的结果 channel；结果可以是直接交付的连接，也可以是已经预留的创建槽位。
- 等待超时严格使用 `time.Duration(gp.TimeOut) * time.Millisecond`，与连接池原有 timeout 配置一致，并且整个等待过程只创建一个 timer。
- `ctx.Done()`、timer 和 Put 交付发生竞争时，在池锁内决定唯一结果；如果连接已经交付，则交付结果胜出，避免丢失连接。
- `MaxWaiters` 是新增可选配置。显式配置大于 0 时按配置限制；旧配置未提供或值不大于 0 时，默认使用 `max(1, MaxConns)`。
- 旧的 `GpoolInit` 函数签名保持不变，同样默认 `MaxWaiters=max(1, maxconns)`。
- 队列达到上限时立即返回 `ErrPoolWaitQueueFull`；到达连接池 timeout 时返回 `ErrPoolWaitTimeout`。

### 8.4 公平性

建议等待者采用 FIFO：

- 避免新请求长期抢占旧请求。
- 延迟分布更稳定。
- 方便定义 timeout 行为。

代价是需要维护 waiter 队列，并处理 waiter 已取消但仍在队列中的情况。

### 8.5 验收测试

- [x] context 取消后在可控时间内返回 `context.Canceled`。
- [x] timeout 到期后返回统一的 `ErrPoolWaitTimeout`。
- [x] 等待时间使用连接池配置的 timeout，不会明显超时。
- [x] 取消 waiter 后不会吞掉下一次连接归还。
- [x] 高并发等待完成后 waiter 数量回到 0。
- [x] 取消与连接交付竞态测试通过 race detector。

### 8.6 评审结论

- [x] 接受并完成
- [ ] 修改后接受
- [ ] 暂不实施
- 评审备注：兼容旧配置，未配置 `MaxWaiters` 时默认等于 `MaxConns`；waiter timeout 与连接池 timeout 相同。

## 9. 评审项五：移除 UseList 和链表节点分配

### 9.1 当前成本

删除 `UseList` 前，一次完整借还包含：

```text
FreeList.Remove
UseList.PushBack   // 分配 list.Element
UseList.Remove
FreeList.PushBack // 分配 list.Element
```

当时实测稳定产生：

```text
96 B/op
2 allocs/op
```

删除 `UseList`、但仍保留 `FreeList` 的中间阶段，一次完整借还为：

```text
FreeList.Remove
inUse++
inUse--
FreeList.PushBack // 分配 list.Element
```

该阶段实测稳定产生：

```text
48 B/op
1 alloc/op
```

当前 `FreeList` 也已替换为预分配 FIFO 环形队列，连接充足时一次完整借还为：

```text
idle.Pop
inUse++
inUse--
idle.Push
```

当前实测稳定产生：

```text
0 B/op
0 allocs/op
```

### 9.2 为什么 UseList 可以考虑移除

原来的 `UseList` 主要承担两个职责：

- 通过长度参与连接总数计算。
- 保存借出的连接节点，归还时再移除。

如果引入明确的 `inUseCount` 和连接状态，通常不需要容器保存所有借出连接。借出的连接已经由调用者持有，池只需要记录计数。

例外情况：如果未来要求强制遍历并关闭所有借出连接，仍需要额外注册表。但强制关闭正在被业务使用的连接本身需要单独定义语义，不建议仅为这个潜在需求保留高频链表分配。

### 9.3 空闲容器候选方案

#### 方案 A：预分配环形队列（推荐作为第一版）

特点：

- 可以保持 FIFO，语义接近当前 `container/list`。
- 可以优先取最早空闲的连接，方便执行 `MaxIdleConnLife`。
- 预分配后稳定路径零分配。
- 继续使用同一把 mutex，改造范围可控。

代价：

- 需要自己实现 head、tail、size。
- 扩容和配置变更需要明确定义。

当前按 `MaxIdleConns` 预分配，不在热路径扩容。

#### 方案 B：slice 栈

特点：实现最简单，LIFO 复用缓存局部性好。

问题：最老的空闲连接可能长期停留在底部，`MaxIdleConnLife` 无法只依靠 Get 时检查，需要额外 reaper。

#### 方案 C：`chan *PoolConn`

特点：等待和直接交接天然简单，支持 context select。

问题：动态创建、严格容量计数、过期检查和关闭池的状态协调仍需要额外同步；channel 自身也有锁，性能需要基准验证。

#### 方案 D：自定义侵入式链表

把 next/prev 指针直接放入 `PoolConn`，可以零分配并保持 FIFO。

问题：实现复杂度和指针状态风险高于环形队列，不建议作为第一版。

### 9.4 推荐决策

当前进度：

- [x] 删除 `UseList`。
- [x] 使用显式 `inUse` 计数。
- [x] 使用预分配 FIFO 环形队列保存 idle 连接。
- [x] 保留单 mutex，先隔离验证“去分配”的收益。

不要在同一个改动中同时做分片或无锁化。

### 9.5 目标流程

Get：

```text
加锁
idle queue 非空：pop -> idle-- -> borrowed++
解锁
返回连接
```

当前 Put：

```text
加锁
如果 idle < MaxIdleConns：push -> borrowed-- -> idle++
否则：淘汰最老 idle，保留刚归还连接，oldest -> closing
解锁
必要时在锁外 Close
```

waiter 直接交接和重复归还校验均已完成。

### 9.6 验收指标

- [x] 串行 Get/Close 达到 `0 allocs/op`。
- [x] 连接充足的并行 Get/Close 达到 `0 allocs/op`。
- [x] 功能和 race 测试通过。
- [x] 串行耗时相对 118 ns/op 有可测量下降，当前约 86 ns/op。
- [x] 保持 FIFO，`MaxIdleConnLife` 顺序语义不变。

### 9.7 评审结论

- [x] 接受并完成环形队列方案
- [ ] 选择 slice 栈
- [ ] 选择 channel
- [ ] 选择侵入式链表
- [ ] 暂不实施
- 评审备注：

## 10. 评审项六：直接把连接交给等待者

### 10.1 修改前的流程

```text
Put 将连接加入 idle 环形队列
Put 发送空通知
waiter 被唤醒
waiter 重新抢 mutex
waiter 再从 idle 环形队列获取连接
```

这个过程包含额外的容器操作、锁竞争和抢占窗口。

### 10.2 当前流程

```text
Put 发现存在 waiter
从等待队列移除最早 waiter
将 PoolConn 直接写入 waiter.result
连接保持 borrowed 状态，仅所有者发生转移
```

直接交接时不需要经过 idle 状态，也不需要进入空闲容器。

### 10.3 取消竞争

需要处理以下竞争：

```text
waiter context 取消
Put 同时准备交付连接
```

当前规则：

- 在池锁内决定 waiter 是否仍有效。
- waiter 结果 channel 容量为 1；交付在池锁内完成，但每个 waiter 只交付一次，因此发送不会阻塞。
- 如果发送前发现 waiter 已取消，将连接交给下一个 waiter 或放回 idle。
- waiter 只能由取消路径或交付路径之一成功完成。

waiter 使用受池锁保护的 `queued/delivered/canceled` 状态。

### 10.4 收益

- 饱和场景减少一次 idle 容器 push/pop。
- 降低 mutex 竞争。
- FIFO waiter 延迟更稳定。
- 连接池满载时吞吐提升最明显。

### 10.5 风险

- waiter 取消和交付竞态较复杂。
- 如果在锁内向无缓冲 channel 发送，可能造成死锁，因此必须避免。
- 需要防止已超时 waiter 吞掉连接。

### 10.6 验收测试

- [x] 多个 waiter 按 FIFO 顺序获得连接。
- [x] waiter 取消后，剩余 waiter 仍能完成。
- [x] 连接总数和状态计数始终一致。
- [x] 取消和交付并发竞争通过 race detector。
- [ ] 进一步降低饱和路径分配。当前约 655 ns/op、424 B/op、5 allocs/op；这是有界 FIFO、独立结果 channel 和单 timer 的成本，后续优化需要单独评审安全复用方案。

### 10.7 评审结论

- [x] 接受并完成
- [ ] 修改后接受
- [ ] 暂不实施
- 评审备注：优先保证 FIFO、公平性、取消正确性和等待队列有界；连接充足路径继续保持零分配。

## 11. 评审项七：将 Close 移出全局锁

### 11.1 当前问题

修改前，超过 `MaxIdleConns` 时，Put 会在持有 `Gpool.mutex` 的情况下调用 `Gc.Close()`。

真实网络 Close 可能包含系统调用、TLS shutdown 或 transport 清理。只要 Close 变慢，整个池的 Get 和 Put 都会停顿。

当前已经采用锁外同步关闭：连接从 idle 环形队列摘除后计入 `closing`，底层 `Conn.Close()` 返回后才执行 `closing--` 并释放容量。

### 11.2 建议流程

锁内只做状态变化：

```text
borrowed/idle -> closing
从可用容器移除
更新计数
收集到局部 closeList
```

解锁后：

```text
逐个执行 Gc.Close()
重新加锁
closing -> closed
释放容量槽位
通知等待创建容量的 waiter
解锁
```

如果采用后台关闭 worker，应使用有界队列，不能无限堆积。

### 11.3 同步关闭还是后台关闭

#### 锁外同步关闭（推荐第一版）

- 代码简单。
- 不阻塞整个池，只阻塞当前归还者。
- 不需要额外 goroutine 生命周期管理。

#### 后台关闭

- Put 延迟更低。
- 需要有界队列、关闭池时 drain、错误处理和 worker 退出。
- 当前实现已经存在重复 Close 和 goroutine 无法退出问题，不建议继续沿用现状。

### 11.4 验收测试

构造一个会阻塞 Close 的 fake connection：

- 一个 goroutine 触发清理并阻塞在 Close。
- 其他 goroutine 仍能从已有 idle 连接 Get/Put。
- 同一个连接只执行一次物理 Close。
- Close 完成前严格容量计数不超限。

### 11.5 评审结论

- [x] 接受并完成锁外同步关闭
- [ ] 采用后台有界关闭队列
- [ ] 暂不实施
- 评审备注：

## 12. 评审项八：清理机制和关闭整个连接池

### 12.1 修改前的问题

- 原有永久 purge goroutine 已随同步 Close 改造删除。
- 没有 `Gpool.Close/Shutdown`。
- 反复初始化会累积 goroutine。
- 空闲连接无法统一关闭。
- 正在创建连接时关闭池的行为没有定义。

### 12.2 当前生命周期

```text
open -> closing -> closed
```

当前 API：

```go
func (gp *Gpool[T]) Close(ctx context.Context) error
```

当前语义：

1. 标记池不再接受新的 Get。
2. 唤醒所有 waiter，并返回 `ErrPoolClosed`。
3. 阻止新的连接创建。
4. 关闭所有 idle 连接。
5. 等待正在 creating 的连接结束并立即关闭。
6. 是否等待 borrowed 连接归还，由 context 控制。
7. context 到期后返回，但池仍保持 closing，后续归还连接直接关闭。
8. 所有资源释放后进入 closed。

idle 连接由一次后台关闭任务处理，保证 `Close(ctx)` 的 context 到期后可以及时返回；物理关闭仍会继续。后续再次调用 `Close` 会等待同一个关闭过程，不会重复关闭 idle。borrowed 连接采用温和关闭，归还时同步物理关闭。

### 12.3 借出连接的处理策略

可选方案：

- 温和关闭：等待借出连接自然归还，推荐默认。
- 强制关闭：遍历注册表并关闭借出连接，需要额外 API 和明确风险。

如果移除 `UseList`，默认只实现温和关闭。若业务确实需要强制关闭，再单独引入 borrowed registry，不要让低频管理需求污染高频 Get/Put。

### 12.4 验收测试

- [x] Close 后新 Get 立即返回 `ErrPoolClosed`。
- [x] waiter 全部被唤醒。
- [x] idle 连接全部且只关闭一次。
- [x] borrowed 连接归还后立即关闭，不重新进入 idle。
- [x] 创建中的连接完成后立即关闭。
- [x] 重复调用 Close 安全且幂等。
- [x] context 到期后关闭继续进行，后续 Close 可以继续等待。
- [x] 不遗留 purge goroutine。

### 12.5 评审结论

- [x] 接受并完成
- [ ] 修改后接受
- [ ] 暂不实施
- 评审备注：采用温和关闭 borrowed；不增加 borrowed registry，不支持强制中断正在使用的连接。

## 13. 评审项九：连接生命周期检查

### 13.1 当前行为

每次从空闲列表取连接时都会：

- 调用 `time.Now()`。
- 检查 `MaxConnLife`。
- 检查 `MaxIdleConnLife`。
- 调用 `IsOpen()`。

这部分目前不是最大瓶颈，但在去掉链表分配后，占比会变高。

### 13.2 建议优化

创建连接时预先计算：

```go
expireAt = createdAt.Add(MaxConnLife)
```

归还连接时计算：

```go
idleExpireAt = returnedAt.Add(MaxIdleConnLife)
```

Get 时只需要比较 deadline，而不是反复做差值和读取多个配置字段。

使用 `time.Time` 可以保留 Go 的 monotonic clock 信息，避免系统时间回拨影响生命周期判断。

### 13.3 IsOpen 策略

`IsOpen()` 只能说明本地 transport 状态，通常不能证明远端连接仍可用。

可选策略：

- 保留轻量本地 IsOpen 检查。
- RPC 失败时标记连接 broken，归还时直接关闭。
- 不在每次 Get 时主动 ping，避免额外网络往返。

### 13.4 后台 reaper

只有在选择 LIFO 空闲栈，或者必须主动按时释放长期空闲连接时，才需要后台 reaper。

若采用 FIFO 环形队列，可以在 Get/Put 时从最老端批量清理到期连接，先避免引入周期 goroutine。

### 13.5 验收测试

- 系统墙钟变化不影响 monotonic 生命周期判断。
- `MaxConnLife=0` 和 `MaxIdleConnLife=0` 不产生额外过期行为。
- 过期连接不会交给业务。
- 过期连接只关闭一次。

### 13.6 评审结论

- [ ] 接受 deadline 方案
- [ ] 保持当前方案
- [ ] 引入后台 reaper
- 评审备注：

## 14. 评审项十：分片连接池（可选）

### 14.1 适用条件

只有满足以下条件后才考虑分片：

- 已经消除稳定路径分配。
- 已经缩短全局锁临界区。
- 已经完成真实网络 I/O 基准。
- profile 仍显示 mutex 是端到端性能瓶颈。

真实 RPC 耗时通常远大于 100~500 ns，因此微基准中的锁退化不一定会成为业务瓶颈。

### 14.2 可能方案

按固定 shard 数拆分：

```go
type shard[T any] struct {
    mu   sync.Mutex
    idle idleQueue[T]
    // counters
}
```

Get 使用原子 round-robin 或低成本随机选择 shard，必要时尝试其他 shard。

### 14.3 难点

- 全局 `MaxConns` 如何在 shard 之间严格协调。
- 某个 shard 空闲而另一个 shard 有 waiter 时如何平衡。
- 连接生命周期和关闭池需要跨 shard 聚合。
- shard 数过大时，少量连接会分布不均。

### 14.4 推荐判断

第一轮不实施。先完成单池零分配和等待路径优化，再根据 CPU profile 决定。

### 14.5 评审结论

- [ ] 后续实施
- [ ] 暂不实施（推荐）
- 评审备注：

## 15. 评审项十一：指标和日志策略

### 15.1 原则

连接池正常 Get/Put 不输出逐请求日志。

需要观察的状态通过原子计数或锁内快照提供：

```go
type PoolStats struct {
    CapacityUsed int64
    Idle         int64
    InUse        int64
    Creating     int64
    Closing      int64
    Waiters      int64
    GetTotal     uint64
    WaitTotal    uint64
    WaitTimeout  uint64
    CreateTotal  uint64
    CreateErrors uint64
    ClosedTotal  uint64
}
```

建议 API：

```go
func (gp *Gpool[T]) Stats() PoolStats
```

### 15.2 保留的日志

建议保留：

- 创建连接失败。
- 等待超时，但需要考虑采样或限速。
- 配置非法。
- 池状态不变量被破坏。
- 关闭池失败。

建议删除或不默认输出：

- 每次 Get 来源。
- 每次 Put 后长度。
- 每次 waiter 唤醒。
- 每个连接创建成功。
- 每个连接正常清理成功。

### 15.3 高频异常日志限速

当服务端不可用时，连接创建失败和等待超时可能形成日志风暴。建议在 logger 层或 gpool 层增加：

- 每地址限速。
- 相同错误按时间窗口聚合。
- 输出 suppressed count。

这项可以独立于连接池数据结构改造。

### 15.4 评审结论

- [ ] 接受 Stats API
- [ ] 接受异常日志限速
- [ ] 暂不实施
- 评审备注：

## 16. 测试计划

### 16.1 单元测试

需要新增可控制行为的 fake connection：

- 可阻塞 Create。
- 可阻塞 Close。
- 可切换 IsOpen。
- 原子统计 Create/Open/Close 次数。
- 可配置创建失败和关闭失败。

核心测试：

1. 初始化连接数正确。
2. Get/Put 状态转换正确。
3. 并发创建不超过 MaxConns。
4. 创建失败释放容量。
5. 重复 Put 不重复入池。
6. context 取消等待。
7. timeout 上界正确。
8. FIFO waiter 顺序。
9. waiter 取消与 Put 竞争。
10. MaxIdleConns 清理。
11. MaxConnLife 清理。
12. MaxIdleConnLife 清理。
13. Close 不在池锁内阻塞其他请求。
14. 每个底层连接只关闭一次。
15. Pool Close 后所有资源最终释放。

### 16.2 Race 测试

至少覆盖：

```bash
go test -race ./gpool -run 'TestGpoolConcurrent'
```

并行基准也用于扩大竞态窗口，但 race benchmark 的数值不用于性能比较。

### 16.3 微基准

保留当前场景并逐步增加：

- Serial Get/Close。
- Parallel available。
- Parallel one connection。
- Parallel pool size 4/16/64。
- Create success/failure。
- Replace broken connection。
- Wait timeout/cancel。
- Purge overflow idle connections。
- Pool Close。

每个基准报告：

- ns/op
- B/op
- allocs/op
- aggregate ops/s

固定机器上使用 `benchstat` 比较每个阶段前后结果。

### 16.4 本机网络 I/O 基准

微基准稳定后，启动本机 Thrift echo 服务，分别测量：

- 不使用池，每次建立 TCP 连接。
- 使用池，连接充足。
- 使用池，连接数小于并发数。
- 连接定期失效和重建。
- 服务端主动断开。

报告：

- QPS
- 平均延迟
- P50/P95/P99/P999
- timeout/error rate
- 实际创建连接数
- 最大同时连接数
- CPU 和内存 profile

网络基准不能使用当前会无限加深的 context 链，也不能把 UUID 生成和逐请求日志混入计时区间。

### 16.5 本机真实 TCP/Thrift 实测结果

环境：Apple M2、darwin/arm64、Go 1.26.4、`GOMAXPROCS=8`。服务端与客户端运行在同一进程，通过 `127.0.0.1` 随机端口通信，使用 Framed Transport、Binary Protocol 和 `Example.Add(i32, i32)`。服务端 handler 只执行整数加法，不输出日志。测试实现位于 `network_benchmark_test.go`，通过 `performance` build tag 显式启用。

吞吐 benchmark 运行三轮，每轮 `benchtime=2s`，下表取中位数。并行场景的 `ns/op` 是聚合吞吐成本，不是单请求延迟：

| 场景 | ns/op | 约合 QPS | B/op | allocs/op | 最大连接数 |
| --- | ---: | ---: | ---: | ---: | ---: |
| 独占连接，串行 | 25,105 | 39,833 | 694 | 22 | 1 |
| gpool，串行 | 25,993 | 38,472 | 758 | 26 | 1 |
| 独占连接，32 worker | 7,213 | 138,638 | 692 | 22 | 32 |
| gpool 连接充足，32 连接 | 7,447 | 134,282 | 757 | 26 | 32 |
| gpool 受限为 8 连接 | 9,669 | 103,423 | 1,182 | 32 | 8 |
| gpool 受限为 1 连接 | 28,150 | 35,524 | 1,182 | 32 | 1 |
| 每请求重新建连，串行固定 1,000 次 | 83,753 | 11,940 | 17,080 | 97 | 1 |

32 并发固定负载延迟测试运行三轮，每轮 32,000 次 RPC，下表取三轮中位数：

| 场景 | QPS | total P50 | total P95 | total P99 | Get P50 | Get P95 | Get P99 |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 独占连接 | 132,955 | 213 µs | 465 µs | 739 µs | 0 | 0 | 0 |
| gpool 连接充足 | 127,322 | 209 µs | 507 µs | 921 µs | 1.9 µs | 16 µs | 98 µs |
| gpool 受限为 8 连接 | 101,143 | 304 µs | 417 µs | 578 µs | 231 µs | 314 µs | 439 µs |
| gpool 受限为 1 连接 | 35,317 | 899 µs | 1.006 ms | 1.132 ms | 871 µs | 976 µs | 1.101 ms |

结论：

- 对最小 Thrift RPC，连接充足时 gpool 相对独占连接的吞吐成本约为 3%~4%，主路径不是当前端到端瓶颈。
- 连接受限后，主要成本来自等待可用连接；`MaxConns=8` 时 Get P50 已达到约 231 µs，远高于连接池自身的纳秒级管理成本。
- 每请求重新建连的串行耗时约为池化串行的 3.2 倍，并产生约 17 KB、97 次分配；持续运行还会耗尽本机临时 TCP 端口，因此该场景必须使用固定迭代次数。
- `MaxConns=8` 的 total P99 低于连接充足场景，是限制服务端同时在途请求带来的削峰结果，不代表吞吐更高；其 QPS 仍下降约 21%。
- 当前结果不支持为了连接充足路径引入分片或无锁结构。优先根据线上并发和 Get 等待分位数配置 `MaxConns`。

复现命令：

```bash
go test -tags=performance ./gpool -run '^$' \
  -bench '^BenchmarkGpoolNetworkRPC$' -benchmem -benchtime=2s -count=3

go test -tags=performance ./gpool -run '^$' \
  -bench '^BenchmarkGpoolNetworkRPCDialEach$' -benchmem -benchtime=1000x -count=3

go test -tags=performance ./gpool \
  -run '^TestGpoolNetworkRPCLatency$' -count=3 -v
```

## 17. 分阶段提交建议

建议每个阶段一个独立提交或 PR：

### 阶段 A：正确性地基

- [x] 增加 fake connection 测试工具。
- [x] 增加容量和状态不变量测试。
- [x] 增加 creating reservation。
- [x] 增加重复归还保护。

### 阶段 B：等待路径

- [x] 修复 duration 单位。
- [x] 支持 context 取消。
- [x] 单 timer。
- [x] waiter 计数和取消测试。

### 阶段 C：零分配容器

- [x] 引入显式 in-use 计数。
- [x] 删除 UseList。
- [x] FreeList 替换为预分配环形队列。
- [x] 连接充足的稳定路径达到 `0 allocs/op`。

### 阶段 D：直接交接和锁外 Close

- [x] FIFO waiter 队列。
- [x] Put 直接交接连接。
- [x] Close 移出全局锁。
- [x] 删除重复后台 Close。

### 阶段 E：生命周期

- [x] 增加 Pool Close。
- [x] 清理 goroutine 可退出。
- deadline 生命周期检查。

### 阶段 F：端到端验证

- [x] 本机真实 TCP/Thrift benchmark。
- CPU、mutex、block profile。
- [x] 判断是否需要分片：当前端到端结果不支持增加分片复杂度。

## 18. 总体验收目标

功能目标：

- `MaxConns` 在并发创建下严格有效。
- Get 等待可以 timeout 和 context cancel。
- PoolConn 重复归还安全。
- 每个底层连接只关闭一次。
- Pool 可以完整关闭且不泄漏 goroutine。
- 所有并发测试通过 race detector。

性能目标：

- 连接充足时 Get/Put 达到 `0 allocs/op`。
- 串行借还当前约 89 ns/op，继续保持零分配。
- 8 核连接充足场景当前约 310 ns/op，继续保持零分配。
- 单连接饱和场景当前约 655 ns/op、424 B/op、5 allocs/op；后续在不破坏取消/交付正确性的前提下优化 waiter 分配。
- 日志开启时正常 Get/Put 不产生逐请求日志。

工程目标：

- 保持现有公开 API 尽量兼容。
- 每个阶段可以独立回滚。
- 不用一次性“无锁化”增加不可控复杂度。
- 性能结论必须同时有微基准和端到端基准支持。

## 19. 需要优先确认的设计选择

开始实现前，建议优先审核并确认以下选择：

1. `closing` 是否占用 `MaxConns` 槽位。本文推荐占用。
2. 重复 `PoolConn.Close` 是静默忽略还是返回错误。为兼容现有 API，本文推荐第一阶段静默忽略并记录指标。
3. 空闲容器选择环形 FIFO、slice LIFO 还是 channel。本文推荐第一阶段使用预分配环形 FIFO。
4. Pool Close 是否等待 borrowed 连接。本文推荐默认等待，由 context 控制上限。
5. waiter 是否严格 FIFO。本文推荐 FIFO。
6. 是否需要强制关闭借出连接。本文建议第一版不支持。
7. 是否立即做分片。本文建议暂不实施。
