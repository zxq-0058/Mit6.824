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
// 参数和答复都需要我们另外再定义
type ExampleArgs struct {
	X int
}

type ExampleReply struct {
	Y int
}

// Add your RPC definitions here.

type TaskType int

// 任务的类型
const (
	MapType = iota
	ReduceType
	WaitType
	KillType
)

type Task struct {
	TaskType   TaskType // 任务类型
	InputFile  []string // 需要处理的文件
	TaskId     int      // 任务编号
	ReducerNum int      // reduce数量
	ReduceId   int      // 当任务类型是Reduce的时候，需要reduce task的编号，范围为[0, ReduceNum)
}

// Cook up a unique-ish UNIX-domain socket name
// in /var/tmp, for the coordinator.
// Can't use the current directory since
// Athena AFS doesn't support UNIX-domain sockets.
func coordinatorSock() string { // 说白了就是返回一个临时的文件名
	s := "/var/tmp/824-mr-"
	s += strconv.Itoa(os.Getuid())
	return s
}
