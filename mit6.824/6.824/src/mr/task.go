package mr

import (
	"fmt"
	"strconv"
	"time"
)

// Task 任务
type Task struct {
	// 任务类型
	Type TaskType
	// 任务编号
	Number TaskNumber

	// 任务开始时间
	StartTime time.Time

	// 任务对应的输入文件名列表
	InputFilenames []string
	// 任务对应的输出文件名列表
	OutputFilenames []string
}

// TaskNumber 任务编号
type TaskNumber int64

// TaskType 任务类型
type TaskType uint

const (
	TaskTypeMap    TaskType = iota // map 任务
	TaskTypeReduce                 // reduce 任务
)

func (t TaskType) String() string {
	switch t {
	case TaskTypeMap:
		return fmt.Sprintf("TaskTypeMap")
	case TaskTypeReduce:
		return fmt.Sprintf("TaskTypeReduce")
	}
	return strconv.Itoa(int(t))
}

// TaskStatus 任务状态
type TaskStatus uint

const (
	_ TaskStatus = iota
	TaskStatusTodo
	TaskStatusDoing
	TaskStatusDone
)
