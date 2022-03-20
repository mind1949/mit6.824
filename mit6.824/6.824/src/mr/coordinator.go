package mr

import (
	"log"
	"net"
	"net/http"
	"net/rpc"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Coordinator struct {
	// 输入的文件名
	files []string
	// reduce任务的数量
	nReduce int

	mu sync.RWMutex

	// 当前处理的任务类型
	taskType TaskType
	// 待处理任务
	todo []Task
	// 已经分配出去的任务
	doing []Task
	// 已经完成的任务
	done []Task

	intermediateFilenames []string

	doneC chan struct{}
}

// Your code here -- RPC handlers for the worker to call.

func (c *Coordinator) GetTask(req *GetTaskRequest, resp *GetTaskResponse) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	if len(c.todo) == 0 {
		return nil
	}

	// 从 todo 队列取一个任务
	task := c.todo[0]
	*resp = GetTaskResponse{
		Task:    task,
		NReduce: c.nReduce,
	}

	// 将task 从 todo 队列 转到 doing 队列
	c.todo = delete(c.todo, 0)
	task.StartTime = time.Now()
	c.doing = append(c.doing, task)

	return nil
}

func (c *Coordinator) CompleteTask(req *CompleteTaskRequest, resp *CompleteTaskResponse) error {
	c.mu.Lock()
	defer c.mu.Unlock()

	for i := 0; i < len(c.doing); i++ {
		task := c.doing[i]
		if task.Number != req.Task.Number ||
			task.Type != req.Task.Type {
			continue
		}

		task.OutputFilenames = req.Task.OutputFilenames
		// 将任务从 doing 队列转到 done 队列
		c.doing = delete(c.doing, i)
		c.done = append(c.done, task)
		// 任务没有全部完成
		if len(c.todo)+len(c.doing) != 0 {
			return nil
		}

		// 任务全部完成
		switch c.taskType {
		case TaskTypeMap:
			done := c.done
			todo, err := mapTasks2ReduceTasks(done)
			if err != nil {
				return err
			}

			for i := range done {
				c.intermediateFilenames = append(c.intermediateFilenames, done[i].OutputFilenames...)
			}
			c.todo = todo
			c.done = c.done[:0]
			c.taskType = TaskTypeReduce
		case TaskTypeReduce:
			for _, filename := range c.intermediateFilenames {
				os.Remove(filename)
			}
			close(c.doneC)
		}
		return nil
	}

	return nil
}

func mapTasks2ReduceTasks(input []Task) (output []Task, err error) {
	m := make(map[TaskNumber]Task)
	for i := 0; i < len(input); i++ {
		filenames := input[i].OutputFilenames
		for _, filename := range filenames {
			s := strings.Split(filename, "-")
			num, err := strconv.Atoi(s[len(s)-1])
			if err != nil {
				return nil, err
			}
			taskNumber := TaskNumber(num)
			task, ok := m[taskNumber]
			if !ok {
				task = Task{
					Type:   TaskTypeReduce,
					Number: taskNumber,
				}
			}
			task.InputFilenames = append(task.InputFilenames, filename)
			m[taskNumber] = task
		}
	}
	for _, task := range m {
		output = append(output, task)
	}
	return output, nil
}

func delete(queue []Task, idx int) []Task {
	if len(queue) <= idx {
		return queue
	}

	last := len(queue) - 1
	queue[idx], queue[last] = queue[last], queue[idx]
	queue = queue[:last]
	return queue
}

// loopReIssue
// 管理超时任务
func (c *Coordinator) loopReIssue() error {
	ticker := time.NewTicker(3 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.doneC:
			return nil
		case now := <-ticker.C:
			c.reIssueElapsedTasks(now)
		}
	}
}

// reIssueElapsedTasks
// 管理超时的任务, 放入到 todo 队列
func (c *Coordinator) reIssueElapsedTasks(now time.Time) {
	c.mu.Lock()
	defer c.mu.Unlock()

	timeout := 10 * time.Second
	for i := 0; i < len(c.doing); i++ {
		task := c.doing[i]
		timeoutTime := task.StartTime.Add(timeout)
		if timeoutTime.After(now) {
			continue
		}

		c.doing = delete(c.doing, i)
		c.todo = append(c.todo, task)
	}
}

//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//
func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	reply.Y = args.X + 1
	return nil
}

//
// start a thread that listens for RPCs from worker.go
//
func (c *Coordinator) server() {
	rpc.Register(c)
	rpc.HandleHTTP()
	//l, e := net.Listen("tcp", ":1234")
	sockname := coordinatorSock()
	os.Remove(sockname)
	l, e := net.Listen("unix", sockname)
	if e != nil {
		log.Fatal("listen error:", e)
	}
	go http.Serve(l, nil)
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire job has finished.
//
func (c *Coordinator) Done() bool {
	<-c.doneC
	return true
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	todo := make([]Task, 0, len(files))
	for i := range files {
		todo = append(todo, Task{
			Type:           TaskTypeMap,
			Number:         TaskNumber(i),
			InputFilenames: []string{files[i]},
		})
	}

	c := Coordinator{
		files:   files,
		nReduce: nReduce,

		todo: todo,

		doneC: make(chan struct{}),
	}

	// Your code here.

	c.server()
	go c.loopReIssue()
	return &c
}
