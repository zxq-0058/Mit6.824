 参考：
 （1）https://blog.csdn.net/ligen1112/article/details/120931874
 
讲座的笔记：
（1）https://github.com/chaozh/MIT-6.824/issues/2

先上网看一下rpc的使用栗子。->rpc.go要求我们定义


Coordinator需要提供rpc的handler函数，当worker发出rpc请求时，可以进行请求处理


微服务中很重要的一个内容就是RPC 远程过程调用（Remote Procedure Call，缩写为 RPC）是一个计算机通信协议，他的主要作用是允许运行于一台计算机的程序调用另一台计算机的子程序，而程序员无需额外地为这个交互作用编程
https://zhuanlan.zhihu.com/p/575618324
![](https://pic2.zhimg.com/80/v2-8eb289c6f009b9ae56e0ed54057592c5_1440w.webp)


对于客户端来讲，首先时连接

Mapreduce的过程：
Map function: Each input key value pair (k1, v1) is processed by a map function. The map function will output a list of (k2, v2), where k2 is the intermediate key.
Reduce function: Each (k2, list of v2)) is passed to a reduce function, the reduce function usually merge the list of v2 to a smaller set of values list(v2) as the final output.

三个关键组件的实现：
All the three files that need to add my own codes are:

src/mr/coordinator.go to add the logic to maintain the status
src/mr/worker.go to add the logic to execute each map/reduce job.
src/mr/rpc.go to add the RPC calls argument and reply contract.



首先先进行代码的阅读：`mrseq.go`
这里我们使用的是单步调试.感觉整个过程下来非常好理解.


任务：worker从coordinator那里取出任务并完成任务，随后写入临时文件。

任务：worker完成任务，并将完成任务的信息发送到coordinator。（这个过程也是通过调用coordinator的处理函数，coordinator随后将其任务管理的元信息进行改写）


我现在的疑惑点是reduce中的Taskid有什么用？
说白了我没有明白map在将中间结果写出去是X-Y分别应该指定什么值？（这个问题很关键）
X的话好理解，对应的是map的taskid，这个直接通过reply中的taskid指定即可。
那么Y呢, Y的范围应该是[1, nReduce]，是不是我们当前的所有的inter都直接写入一个固定的Y呢？其实不是，跟具体的key相关，所以需要用到hash


How to use make in golang to create a 2-dim array?
In Go, you can use the built-in make function to create a 2-dimensional array. Here's an example of how to create a 2D array of integers:

go
Copy code
// create a 2D array of size 3x4
rows, cols := 3, 4
arr := make([][]int, rows)
for i := range arr {
    arr[i] = make([]int, cols)
}
In the example above, we first create a slice of length rows, where each element of the slice is a slice of integers of length cols. We then use a for loop to initialize each of these inner slices to the zero value for integers (which is 0).

If you want to initialize the array with some other value, you can modify the initialization loop accordingly. For example, to initialize the array to all 1s:

go
Copy code
// create a 2D array of size 3x4 initialized to 1
rows, cols := 3, 4
arr := make([][]int, rows)
for i := range arr {
    arr[i] = make([]int, cols)
    for j := range arr[i] {
        arr[i][j] = 1
    }
}


当worker向coor请求任务时，有以下若干情况：
（1）map任务队列不为空，此时从Map任务队列中取出一个任务并返回
（2）map任务队列为空，但是Map没有全部完成（这里显然是需要写一个判断函数的），此时只能让worker等待（TaskType为Waiting）
（3）



// 对于我们的TaskManager，所管理的任务只有两种类型，要不然是MapType，要不然是ReduceType，这二者在Make之初都是Initialized状态，随后worker请求任务时会通过调用Emit函数改为Processing状态并增加开始时间
// WaitType, KillType不在我们的Meta的管理的范围之内


修改 const debugEnabled = false, 把false改成true

接着在 master_rpc.go修改

debug("RegistrationServer: accept error", err) 在error 后加%v

debug("RegistrationServer: accept error %v", err)

最后运行 go test -run Sequential 如果成功了出现下面的样子


如果有worker中途crash掉了（在该实验说明中，指该worker处理时间超过10s仍未向master报告任务处理完成），master虽然可以在10s后把该任务再分配给其他来要任务的worker，但是之前的worker可能写到一半的中间结果或者最后结果文件就残留了不正确的答案，如何处理？实验文档中给出了一个解决方法是worker写结果时到文件时先写入临时文件；待向master汇报后，由master来修改临时文件名为中间结果/最后结果的文件名，并标记任务完成。