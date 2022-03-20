# 要求

* 因为所有的进程都跑在一台机器上, 所以显然所有进程共享同一个文件系统, 单当进程跑在不同机器上时会使用类似 GFS(google file system) 的全局文件系统
* 通过 go 的 plugine 机制实现 map 与 reduce 的可拔插

## 流程

* worker 请求 coordinator 发送一个任务
* coordinator 收到请求后, 查找没有开始的任务, 并发送相应的文件名给 worker

## Worker

* job完成后的退出机制:
  * 方案1: 当 worker通过 call 函数远程调用 coordinator 失败时, 就认为 coordinator 已经完成 job 并退出, 此时 worker 直接退出
  * 方案2: coordinator 发送一个任务给worker 让 worker 退出
* 防止出现部分写入的文件被使用:
  * 先将输出写入到临时文件中, 当任务完成了再通过os.Rename 原子化地对文件重命名

### Map

* 在 map 阶段创建 nReduce个文件, 并将中间数据使用 ihash(key) % nReduce 函数计算属于第x个 reduce task, 并放在第 x 个文件中 , 以便于 nReduce个 reduce 任务消费.
* 中间文件的命名惯例: mr-X-Y, 其中 X 表示Map 任务编号, Y 表示 Reduce 任务编号

### Reduce

* 通过MakeCoordinator()函数指定 reduce 任务的数量为 nReduce
* 第 x 个 reduce 任务生成的数据放在文件 mr-out-x 中
* 每行数据的格式化方式: fmt.Fprintf(ofile, "%v%v\n", intermediate[i].Key, output)

## Coordinate

* 使用 Done 方法表示 job 是否执行完毕
* 对每一个分发出去的任务等待十秒, 十秒后没有完成就重新发起执行

# 实现

## 流程

* worker 发送 GetTask RPC, 请求 coordinator 分配任务
* coordinator 收到 GetTaskRequestTas RPC 后, 查找待处理的任务并发送给 worker
* worker 处理完任务后, 发送 CompleteTask RPC,  告知 coordinator 任务 i 处理完成
* coordinator 接到 CompleteTask RPC 后将任务 i 标记为已完成
* coordinator 周期性的检查已经开始但还未完成的任务, 若任务超时(10s)则将任务标记为待执行状态, 待接收到 RequestTask RPC 时, 将任务分配出去

## 组件设计

### Worker

* 请求任务
* 执行任务
* 告知任务完成

### Coordinator

* 分配任务
* 管理任务
  * 重发超时任务
  * 标记完成任务
  * map 任务执行完成后, 生成 reduce 任务(pending 状态, 只有当所有的 map 任务都执行完成后才能开始分配. 因为要聚合所有 map 任务才能得到完整的 reduce 任务)

### RPC

#### GetTask

参数:

* 空参数

响应:

* TaskType(任务类型)
* TaskNumber(任务编号,在命名输出文件时会用到)
* InputFilenames(输入的文件名列表)
* NReduce(reduce任务的数量)

实现:

worker无任务处理时, 发送RPC

coordinator 接收到RPC 时, 查找未处理任务并分配

TaskType 包含三种类型: TaskTypeMap / TaskTypeReduce / TaskTypeExit

#### CompleteTask

参数:

* TaskType(任务类型)
* TaskNumber(任务编号)
* OutputFilenames(输出的文件名列表)

响应:

* 空参数
