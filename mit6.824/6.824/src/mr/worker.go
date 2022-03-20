package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io"
	"log"
	"net/rpc"
	"os"
	"sort"
)

//
// Map functions return a slice of KeyValue.
//
type KeyValue struct {
	Key   string
	Value string
}

//
// use ihash(key) % NReduce to choose the reduce
// task number for each KeyValue emitted by Map.
//
func ihash(key string) int {
	h := fnv.New32a()
	h.Write([]byte(key))
	return int(h.Sum32() & 0x7fffffff)
}

//
// main/mrworker.go calls this function.
//
func Worker(mapf func(string, string) []KeyValue,
	reducef func(string, []string) string) {

	// Your worker implementation here.
	for {
		var req GetTaskRequest
		var resp GetTaskResponse
		success := call("Coordinator.GetTask", &req, &resp)
		if !success {
			return
		}
		switch resp.Task.Type {
		case TaskTypeMap:
			err := handleMapTask(resp.Task, resp.NReduce, mapf)
			if err != nil {
				fmt.Printf("err: %+v", err)
			}
		case TaskTypeReduce:
			err := handleReduceTask(resp.Task, reducef)
			if err != nil {
				fmt.Printf("err: %+v", err)
			}
		}
	}
}

func handleMapTask(task Task, nReduce int, mapFn func(string, string) []KeyValue) error {
	if len(task.InputFilenames) == 0 {
		return nil
	}
	file, err := os.Open(task.InputFilenames[0])
	if err != nil {
		fmt.Printf("err: %+v", err)
	}
	defer file.Close()

	b, err := io.ReadAll(file)
	if err != nil {
		return err
	}
	kvs := mapFn(file.Name(), string(b))

	// 创建 nReduce 个临时文件
	files := make([]*os.File, 0, nReduce)
	renames := make(map[string]string)
	for i := 0; i < nReduce; i++ {
		filename := fmt.Sprintf("mr-%d-%d", task.Number, i)
		file, err := os.CreateTemp(".", fmt.Sprintf("%s-*", filename))
		renames[file.Name()] = filename
		if err != nil {
			return err
		}
		files = append(files, file)
	}
	defer func() {
		for i := range files {
			files[i].Close()
		}
	}()

	// 使用 ihash(key) 将每个 kv 追加到对应的文件里
	for _, kv := range kvs {
		num := ihash(kv.Key) % nReduce
		file := files[num]
		enc := json.NewEncoder(file)
		err := enc.Encode(&kv)
		if err != nil {
			return err
		}
	}

	// 执行完毕后重命名所有临时文件
	for old, new := range renames {
		err := os.Rename(old, new)
		if err != nil {
			return err
		}
		task.OutputFilenames = append(task.OutputFilenames, new)
	}

	// 发送 completeTask RPC 给 coordinator
	var req = CompleteTaskRequest{
		Task: task,
	}
	var resp CompleteTaskResponse
	success := call("Coordinator.CompleteTask", &req, &resp)
	if !success {
		return nil
	}

	return nil
}

func handleReduceTask(task Task, reduceFn func(string, []string) string) error {
	outputfilename := fmt.Sprintf("mr-out-%d", task.Number)
	outputfile, err := os.CreateTemp(".", fmt.Sprintf("%s-*", outputfilename))
	if err != nil {
		return err
	}
	defer outputfile.Close()

	intermediate := make([]KeyValue, 0)
	for i := range task.InputFilenames {
		file, err := os.Open(task.InputFilenames[i])
		if err != nil {
			return err
		}
		dec := json.NewDecoder(file)
		for {
			var kv KeyValue
			if err := dec.Decode(&kv); err != nil {
				break
			}
			intermediate = append(intermediate, kv)
		}
		file.Close()
	}
	sort.Sort(ByKey(intermediate))

	//
	// call Reduce on each distinct key in intermediate[],
	// and print the result to mr-out-0.
	//
	i := 0
	for i < len(intermediate) {
		j := i + 1
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		output := reduceFn(intermediate[i].Key, values)

		// this is the correct format for each line of Reduce output.
		fmt.Fprintf(outputfile, "%v %v\n", intermediate[i].Key, output)

		i = j
	}

	// 重命名
	err = os.Rename(outputfile.Name(), outputfilename)
	if err != nil {
		return err
	}

	// 发送 completeTask RPC 给 coordinator
	task.OutputFilenames = append(task.OutputFilenames, outputfilename)
	var req = CompleteTaskRequest{
		Task: task,
	}
	var resp CompleteTaskResponse
	success := call("Coordinator.CompleteTask", &req, &resp)
	if !success {
		return nil
	}

	return nil
}

// for sorting by key.
type ByKey []KeyValue

// for sorting by key.
func (a ByKey) Len() int           { return len(a) }
func (a ByKey) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a ByKey) Less(i, j int) bool { return a[i].Key < a[j].Key }

//
// example function to show how to make an RPC call to the coordinator.
//
// the RPC argument and reply types are defined in rpc.go.
//
func CallExample() {

	// declare an argument structure.
	args := ExampleArgs{}

	// fill in the argument(s).
	args.X = 99

	// declare a reply structure.
	reply := ExampleReply{}

	// send the RPC request, wait for the reply.
	// the "Coordinator.Example" tells the
	// receiving server that we'd like to call
	// the Example() method of struct Coordinator.
	ok := call("Coordinator.Example", &args, &reply)
	if ok {
		// reply.Y should be 100.
		fmt.Printf("reply.Y %v\n", reply.Y)
	} else {
		fmt.Printf("call failed!\n")
	}
}

//
// send an RPC request to the coordinator, wait for the response.
// usually returns true.
// returns false if something goes wrong.
//
func call(rpcname string, args interface{}, reply interface{}) bool {
	// c, err := rpc.DialHTTP("tcp", "127.0.0.1"+":1234")
	sockname := coordinatorSock()
	c, err := rpc.DialHTTP("unix", sockname)
	if err != nil {
		log.Fatal("dialing:", err)
	}
	defer c.Close()

	err = c.Call(rpcname, args, reply)
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}
