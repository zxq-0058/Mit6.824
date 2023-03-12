package mr

import (
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/ioutil"
	"log"
	"net/rpc"
	"os"
	"sort"
	"time"
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

func DoMap(mapf func(string, string) []KeyValue, map_task Task) {
	filenames := map_task.InputFile
	task_id := map_task.TaskId
	nReduce := map_task.ReducerNum

	var intermediate []KeyValue
	for _, filename := range filenames {
		content, err := ioutil.ReadFile(filename)
		if err != nil {
			log.Fatalf("cannot read %v: %v", filename, err)
		}
		kva := mapf(filename, string(content))
		intermediate = append(intermediate, kva...)
	}

	HashKV := make([][]KeyValue, nReduce)
	for _, kva := range intermediate {
		bucket_id := ihash(kva.Key) % nReduce
		HashKV[bucket_id] = append(HashKV[bucket_id], kva)
	}

	for i := 0; i < nReduce; i++ {
		intermediate_output := fmt.Sprintf("mr-tmp-%d-%d", task_id, i)
		file, err := os.Create(intermediate_output)
		if err != nil {
			log.Fatalf("Error creating file %s: %v", intermediate_output, err)
		}

		encoder := json.NewEncoder(file)
		for _, kva := range HashKV[i] {
			if err := encoder.Encode(&kva); err != nil {
				log.Fatalf("Error encoding KeyValue: %v", err)
			}
		}

		if err := file.Close(); err != nil {
			log.Fatalf("Error closing file %s: %v", intermediate_output, err)
		}
	}

}

// DoReduce performs the reduce phase of MapReduce
// by processing intermediate files and calling reduce function on key groups
func DoReduce(reducef func(string, []string) string, reduce_task Task) {
	// Initialize an empty slice to hold intermediate key-value pairs
	intermediate := []KeyValue{}

	// Loop through each intermediate file for this reduce task
	for _, filename := range reduce_task.InputFile {
		// Open the file
		file, err := os.Open(filename)
		if err != nil {
			log.Fatalf("cannot open %v", filename)
		}
		defer file.Close()

		// Create a JSON decoder to decode the key-value pairs from the file
		decoder := json.NewDecoder(file)
		for {
			var kv KeyValue
			err := decoder.Decode(&kv)
			if err != nil {
				break
			}
			// Append the decoded key-value pair to the intermediate slice
			intermediate = append(intermediate, kv)
		}
	}

	// Sort all intermediate key-value pairs by key
	sort.Slice(intermediate, func(i, j int) bool {
		return intermediate[i].Key < intermediate[j].Key
	})

	// Create a temporary file to hold the reduce output
	dir, _ := os.Getwd()
	tempFile, err := ioutil.TempFile(dir, "mr-tmp-*")
	if err != nil {
		log.Fatal("Failed to create temp file", err)
	}

	// Process the intermediate key-value pairs
	i := 0
	for i < len(intermediate) {
		j := i + 1
		// Find the end of the current key group
		for j < len(intermediate) && intermediate[j].Key == intermediate[i].Key {
			j++
		}
		values := []string{}
		for k := i; k < j; k++ {
			values = append(values, intermediate[k].Value)
		}
		// Call the reduce function on the current key group and write the output to the temporary file
		output := reducef(intermediate[i].Key, values)
		fmt.Fprintf(tempFile, "%v %v\n", intermediate[i].Key, output)
		i = j
	}
	tempFile.Close()

	// Rename the temporary file to the final output file
	oname := fmt.Sprintf("mr-out-%d", reduce_task.ReduceId)
	os.Rename(tempFile.Name(), oname)
}

func TaskDone(task *Task) {
	ok := call("Coordinator.TaskDone", task, nil)
	if !ok {
		fmt.Printf("call Fail!")
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

	err = c.Call(rpcname, args, reply) // 远程调用master的rpc服务函数并获得reply
	if err == nil {
		return true
	}

	fmt.Println(err)
	return false
}

func Worker(mapf func(string, string) []KeyValue, reducef func(string, []string) string) {
	for {
		// Request a task from the coordinator.
		args := ExampleArgs{}
		task := Task{}
		if !call("Coordinator.DistributeTask", &args, &task) {
			log.Fatal("Failed to request task from coordinator")
		}

		// Execute the task based on its type.
		switch task.TaskType {
		case MapType:
			DoMap(mapf, task)
		case ReduceType:
			DoReduce(reducef, task)
		case WaitType:
			time.Sleep(500 * time.Millisecond)
			continue
		case KillType:
			return
		default:
			log.Fatalf("Invalid task type: %v", task.TaskType)
		}

		// Notify the coordinator that the task is done.
		if !call("Coordinator.TaskDone", &task, &struct{}{}) {
			log.Fatalf("Failed to notify coordinator that task is done: %v", task)
		}
	}
}
