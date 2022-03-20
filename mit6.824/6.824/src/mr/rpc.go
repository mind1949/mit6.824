package mr

//
// RPC definitions.
//
// remember to capitalize all names.
//

import (
	"os"
	"strconv"
)

//
// example to show how to declare the arguments
// and reply for an RPC.
//

type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.

// GetTaskRequest
// GetTask RPC 请求
type GetTaskRequest struct{}

// GetTaskResponse
// GetTask RPC 响应
type GetTaskResponse struct {
	Task
	NReduce int
}

// CompleteTaskRequest
// CompleteTask 请求
type CompleteTaskRequest struct {
	Task
}

// CompleteTaskResponse
// CompleteTask 响应
type CompleteTaskResponse struct{}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string {
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
