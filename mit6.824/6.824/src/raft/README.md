# Raft

## 算法设计

### State

#### 持久化

* currentTerm: 节点当前任期
* votedFor: 节点当前任期投票选出的 Leader Id, 若没有则为 null
* log[]: raft 可复制日志
  * 元素是日志条目 entry, 日志条目包含: index/term/command
  * 日志的初始索引是 1

### 非持久化

* commitIndex: 最近提交的日志条目索引
  * 初始值为 0
  * 单调递增
* lastApplied: 最近应用到可复制状态机中的日志条目索引
  * 初始值为 0
  * 单调递增

#### Leader特有

* nextIndex[]:  表示要发送给每个 Follower 的第一个日志条目索引
  * 初始值是 log[]最大的索引+1
* matchIndex[]: 表示每个 Follower 的日志中与 log[]匹配上的索引
  * 初始值是 0

### RPC

#### AppendEntries RPC

##### arguments 请求参数

* term: 发起者当前任期
* leaderId: 发起请求的领导者 id, 接受者收到后设置 votedFor = leaderId, 方便重定向客户端请求到 leader
* prevLogIndex: 欲复制到 Follower 的日志条目的前一个条目的索引
* prevLogTerm: 欲复制到 Follower 的日志条目的前一个条目的任期
* entries[]: 欲复制到 Follower 的日志条目列表
* leaderCommit: 领导者的 commitIndex

##### results 响应结果

* term: 接受者的 currentTerm, 用于发送者更新自己的 currentTerm
* succus: 表示接受者是否包含匹配 prevLogIndex 和 prevLogTerm 的日志条目

##### 接受者实现

* 若 arguments.term < currentTerm, 则:
  * results.term = currentTerm
  * results.success = false
* 若不包含匹配 arguments.term 和 arguments.Index 的日志条目, 则:
  * results.Term = currentTerm
  * results.success = false
* 若 arguments.entries[] 的条目与 log[] 中的冲突日志条目, 则:
  * 删除 log[] 中该日志条目以及之后的条目
* 追加  arguments.entries[] 中所含不在 log[] 中的日志条目
* 若 arguments.leaderCommit> commitIndex, 则 commitIndex = min(arguments.leaderCommit, log[]中最新条目索引)

#### RequestVote RPC

若 RPC 请求或相应中的 term 大于 currentTerm, 则:* 设置 currentTerm = term

* 转变为 Follower 状态

##### 请求参数

* term: 候选人的任期
* candidateId: 候选人的 id
* lastLogIndex: 最新日志条目索引
* lastLogTerm: 最新日志条目任期

##### 响应结果

* term: 接受者的 currentTerm. 用于候选人更新任期
* voteGranted: 表示是否投票

##### 接受者实现

* 若 arguments.term < currentTerm, 则 results.voteGranted = false
* 若 votedFor 是 null 或 voteFor == arguments.candidateId

  * 若 arguments.lastLogTerm > currentTerm, 则:
    * results.voteGranted = true
    * currentTerm = arguments.term
    * voteFor = arguments.candidateId
  * 若 arguments.lastLogTerm == currentTerm
    * 若 arguments.LastLogIndex >= len(log[]), 则:
      * result.voteGranted = true
      * currentTerm = arguments.term
      * voteFor = arguments.candidateId
    * 否则 result.voteGranted = false
  * 否则 arguments.lastLogTerm < currentTerm, 则:
    * result.voteGranted = fals

### Follower/Candidate/Leader

#### Follower 状态

raft节点启动时, 状态为 `Follower`

随机设置一个在150ms~300ms间的 election timer

##### 若 election timeout 则:

* 转变状态为 `Candidate` 状态

##### 若收到 RequestVote RPC 则:

* 重置 election timer
* 根据 RequestVote RPC 接受者实现处理

##### 若收到 AppendEntries RPC 则:

* 重置 election timer
* 根据 AppendEntries RPC 接受者实现处理

##### 若接收到 Client 请求

重定向请求到 Leader

#### Candidate 状态

##### 选举流程

* 设置 currentTerm += 1
* 为自己投票, 设置 votedFor 为自己的 ID
* 重置 election timer
* 发送 RequestVote RPC 其余所有的 raft 节点
* 若响应为:
  * grantVoted == true, 则 voteCount += 1;
* 若收到的 voteCount大于总节点数的一半, 则:
  * 状态转换为 Leader
  * 若发送 election timeout
    * 开始下一轮选举

##### 若收到 AppendEntries RPC

* 重置 election timer
* 根据 AppendEntries RPC 接受者实现处理

##### 若收到 RequestVote RPC

* 重置 election timer
* 根据 RequestVote RPC 接受者实现处理

#### Leader 状态

##### 进入 Leader 状态时

* 立即并发发送空 AppendEntries RPC 给所有其他的节点, 确立领导权
  * TODO: 应该是什么样的?
* 此时没有选举计时器, 而是有 heartbeat timer 提示闲置时周期性发送空AppendEntries RPC 给所有 Follower, 确保领导权
* 若没有接收到当前任期的 客户端命令, 则不会把以前任期的日志条目复制出去, 只会发送 空 appendEntries RPC

##### 若收到 Client 请求

* 生成 日志条目 entry
* 追加到 log[] 中
* 并发发送 AppendEntries RPC 到各个 Follower 节点
  * 没有成功复制的 follower 会不断发送请求, 直到复制成功
  * 若复制成功, 且最终 matchIndex[] 中大部分的 索引值 >= entry.index, 则 commitIndex = entry.index, 然后更新 lastApplied, 应用日志条目到可复制状态机
  * 发送响应到客户端

##### 若收到 RequestVote RPC

* *若 arguments.term 大于 currentTerm, 则:*
  * 设置 currentTerm = term
  * 转变为 Follower 状态, 并根据 RequestVote RPCAppendEntries RP 的接受者实现处理
* 若 arguments.term 小于 currentTerm, 则:
  * set results.term = currentTerm
  * set results.success = false

##### 若收到 AppendEntries RPC

* 若 arguments.term 大于 currentTerm, 则:
  * 设置 currentTerm = term
  * 转变为 Follower 状态, 并根据 AppendEntries RPC 的接受者实现处理
* 若 arguments.term < currentTerm, 则:
  * set results.term = currentTerm
  * set results.success = false

#### 所有状态

* 若 commitIndex > lastApplied, 则:
  * 应用 log[] 中在 索引区间 (lastApplied, commitIndex] 中的日志条目到 可复制状态机中
  * 设置 lastApplied = commitIndex
* 若 RPC 请求或相应中的 term 大于 currentTerm, 则:
  * 设置 currentTerm = term
  * 转变为 Follower 状态

### 组件设计

> TODO:

### 编码实现

> TODO:

## Part 2A: Leader Election

理解/整理领导选举的流程 --> 思考如何实现 --> 编码实现

## Part 2B: Log

## Part 2C: Pesistence

## Part 2D: Log Compaction

end
