package mr

import (
	"fmt"
	"io/ioutil"
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

func panic_on(cond bool) {
	if cond {
		panic("assertion failed")
	}
}

// Coordinator的状态: Map阶段、Reduce阶段，AllDone阶段
type CoordinatorState int

const (
	MapPhase = iota
	ReducePhase
	AllDone
)

// A mutex for tasks meta-info, namely for TasksMetaManager
var mu sync.Mutex

type Coordinator struct {
	TaskChannelMap    chan *Task        // map的任务队列
	TaskChannelReduce chan *Task        // reduce的任务队列
	UniqueTaskId      int               // 分配任务号，调用完GenerateTaskId(自动加1)
	ReducerNum        int               // reduce 的数量
	CurrentState      CoordinatorState  // 当前的状态：Map阶段 || Reduce阶段 || AllDone阶段
	TasksManager      *TasksMetaManager // 任务元信息管理
}

type TaskState int

const (
	Initialized TaskState = iota // tasks which is initialized but has not been distributed
	Processing                   // tasks which have been distributed but have not received response from worker
	Done                         // tasks which are done
)

type TaskInfo struct {
	TaskPtr   *Task
	State     TaskState
	StartTime time.Time
}

type TasksMetaManager struct {
	MetaMap map[int]*TaskInfo // id ----> TaskInfo
}

func (t *TasksMetaManager) AddTaskMeta(task *Task) {
	tinfo := &TaskInfo{
		TaskPtr:   task,
		State:     Initialized,
		StartTime: time.Time{},
	}
	t.MetaMap[task.TaskId] = tinfo
}

func (t *TasksMetaManager) EmitTask(task *Task) {
	/*
		TODO: Add some Protective checking
	*/
	tinfo, _ := t.MetaMap[task.TaskId]
	tinfo.State = Processing
	tinfo.StartTime = time.Now()
}

func (t *TasksMetaManager) MapTasksDone() bool {
	// To check whether All the map tasks is Done
	// Note that at this point, the MetaMap should not contain any info of Reduce Task
	// Since only when all the Map tasks is done can we call MakeReduceTasks()
	for _, tinfo := range t.MetaMap {
		fmt.Println(tinfo)
		if tinfo.State != Done {
			return false
		}
	}
	return true
}

func (t *TasksMetaManager) ReduceTasksDone() bool {
	// Check whether all the reduce tasks are done
	for _, tinfo := range t.MetaMap {
		if tinfo.TaskPtr.TaskType == ReduceType && tinfo.State != Done {
			return false
		}
	}
	return true
}

// Your code here -- RPC handlers for the worker to call.
//
// an example RPC handler.
//
// the RPC argument and reply types are defined in rpc.go.
//

func (c *Coordinator) GenerateTaskId() int {
	// TODO: 考不考虑0?
	ret := c.UniqueTaskId
	c.UniqueTaskId++
	return ret
}

func (c *Coordinator) Example(args *ExampleArgs, reply *ExampleReply) error {
	// 这个函数在收到来自worker的rpc请求之后进行答复（将答复放在ExampleReply里面）
	reply.Y = args.X + 1
	return nil
}

func (c *Coordinator) Reply(args *ExampleArgs, reply *Task) error {
	*reply = *<-c.TaskChannelMap
	return nil
}

func (c *Coordinator) TaskDone(task *Task, reply *ExampleReply) error {
	/*
		Process the task completion request from the worker and modify the task metadata managed by us accordingly.
	*/
	taskID := task.TaskId
	tinfo, ok := c.TasksManager.MetaMap[taskID]
	if !ok {
		return fmt.Errorf("Invalid Task ID: %v", taskID)
	}
	tinfo.State = Done
	return nil
}

func (c *Coordinator) MakeMapTasks(files []string) {
	for _, v := range files {
		id := c.GenerateTaskId()
		task := Task{
			TaskType:   MapType,
			InputFile:  []string{v},
			TaskId:     id,
			ReducerNum: c.ReducerNum,
			ReduceId:   -1, // for map task, reduce id is useless
		}

		c.TasksManager.AddTaskMeta(&task)
		c.TaskChannelMap <- &task
	}

	defer close(c.TaskChannelMap)
	fmt.Println("done making map Tasks")
}

//
// start a thread that listens for RPCs from worker.go
// 启动一个监听来自worker的rpc请求的线程
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
	go http.Serve(l, nil) // handler可能需要改为我们实现的handler
}

//
// main/mrcoordinator.go calls Done() periodically to find out
// if the entire Task has finished.
//
func (c *Coordinator) Done() bool {
	ret := c.CurrentState == AllDone
	// 判断是否完成任务
	// Your code here.
	return ret
}

//
// create a Coordinator.
// main/mrcoordinator.go calls this function.
// nReduce is the number of reduce tasks to use.
//
func MakeCoordinator(files []string, nReduce int) *Coordinator {
	t := &TasksMetaManager{
		MetaMap: make(map[int]*TaskInfo),
	}
	c := Coordinator{
		TaskChannelMap:    make(chan *Task, len(files)),
		TaskChannelReduce: make(chan *Task, nReduce),
		UniqueTaskId:      0,
		ReducerNum:        nReduce,
		CurrentState:      MapPhase,
		TasksManager:      t,
	}
	c.MakeMapTasks(files)

	c.server() // 监听来自worker的请求（注意到server函数内部有一个goroutine处理，因server()函数会返回但是该线程依旧运行）
	return &c
}

func (c *Coordinator) DistributeTask(args ExampleArgs, reply *Task) error {
	mu.Lock()
	defer mu.Unlock()
	if c.CurrentState == MapPhase {
		if len(c.TaskChannelMap) > 0 {
			// 说明当前还有map任务没有被处理
			*reply = *<-c.TaskChannelMap
			c.TasksManager.EmitTask(reply) // 这里主要是开启计时并将任务的状态改为Processing
			return nil
		} else {
			// 说明当前所有的map任务都已经分发出去，但是可能worker还没完成
			reply.TaskType = WaitType
			if c.TasksManager.MapTasksDone() {
				// if all the MapTasks are Done, then move to The Reduce Phase
				c.Move2NextState()
			}
		}
	}

	if c.CurrentState == ReducePhase {
		if len(c.TaskChannelReduce) > 0 {
			*reply = *<-c.TaskChannelReduce
			c.TasksManager.EmitTask(reply)
			return nil
		} else {
			// 说明当前reduce任务都已经分发出去，但是可能worker还没完成
			reply.TaskType = WaitType
			if c.TasksManager.ReduceTasksDone() {
				// if all the MapTasks are Done, then move to The AllDone Phase
				c.Move2NextState()
			}
		}
	}

	if c.CurrentState == AllDone {
		reply.TaskType = KillType
	}
	return nil
}

func (c *Coordinator) MakeReduceTasks() {
	/*
		This method will only be called after the MapPhase has completed!
	*/
	nReduce := c.ReducerNum
	for i := 0; i < nReduce; i++ {
		id := c.GenerateTaskId()
		cwd, _ := os.Getwd()
		filenames, _ := findFilesWithPrefix(cwd, i)
		task := Task{
			TaskType:   ReduceType,
			InputFile:  filenames,
			TaskId:     id,
			ReducerNum: c.ReducerNum,
			ReduceId:   i + 1,
		}
		fmt.Printf("taskid%d , 创建编号为%d\n", id, task.ReduceId)
		c.TasksManager.AddTaskMeta(&task)
		c.TaskChannelReduce <- &task
	}

	defer close(c.TaskChannelReduce)
	fmt.Println("Done making reduce tasks")
}

func (c *Coordinator) Move2NextState() {
	/*
		State transition:
		(1) MapPhase ---> ReducePhase
		(2) ReducePhase ---> AllDone
	*/
	switch c.CurrentState {
	case MapPhase:
		c.CurrentState = ReducePhase
		c.MakeReduceTasks()
	case ReducePhase:
		c.CurrentState = AllDone
	default:
		err := fmt.Sprintf("Invalid state transition! Current state: %v", c.CurrentState)
		panic(err)
	}
}

func findFilesWithPrefix(dirPath string, Y int) ([]string, error) {
	var filenames []string

	files, err := ioutil.ReadDir(dirPath)
	if err != nil {
		return nil, err
	}

	for _, file := range files {
		if strings.HasPrefix(file.Name(), "mr-tmp-") {
			parts := strings.Split(file.Name(), "-")
			if len(parts) == 4 {
				X, err := strconv.Atoi(parts[2])
				if err == nil && X >= 0 && strings.HasSuffix(parts[3], strconv.Itoa(Y)) {
					filenames = append(filenames, file.Name())
				}
			}
		}
	}

	return filenames, nil
}
