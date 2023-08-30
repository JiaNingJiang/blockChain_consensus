## 一、`PBFT`服务端

1. **`PBFT`节点的初始化配置项**

`PBFT`节点的主程序代码位于`/pbftServer.go`

- `node_name`：`PBFT`节点的标识符
- `node_addr`：`PBFT`节点的通信地址
- `main_node`：`PBFT`集群中初始主节点的标识名
- `pbft_cluser`：`PBFT`集群所有节点的信息

2. **合约模块**

合约模块代码位于`/pbft/contract`目录下。	

用户需要将新增的合约类首先添加到`/pbft/contract/contract.go`中，接着需要为自定义合约类创建源文件(如`bgp.go`/`common.go`)。

具体设计可以参考`Common`合约类为设计模板，`Common`类实现了两个合约函数：

- `Common:Write`：实现简单的 `k-v`写入功能
- `Common:Read`：实现简单的`k-v`查询功能

3. **关于`TPS`的两个参量**

在`/network/node.go`中，有两个变量会影响共识`TPS`。

- `MinAccumulatedTx`：启动一轮共识最少需要的交易数
- `MaxWaitingTx`：启动一轮共识最多等待的时间

## 二、测试客户端

1. **客户端的初始化配置项**

客户端程序位于`/client/client.go`，其初始化配置项包括:

- `nodeID`：客户端节点的名称标识符
- `nodeUrl`：客户端节点的`url`(用来接收`PBFT`主节点的`reply Msg`)
- `primaryURL`：PBFT集群中主节点的`url`

2. **合约执行与测试案例**

- 如何让`PBFT`集群执行某合约的合约函数,并得到执行结果？

请以`/client/client.go`文件中的`client.CommonWrite()`和`client.CommonRead()`函数作为模板。

这两个函数分别向`PBFT`主节点发送了执行`Common:Write`和`Common:Read`的请求，发送的请求结构体为：

```go
type RequestMsg struct {
	Timestamp  uint64   `json:"timestamp"`   // 当前时间戳
	ClientID   string   `json:"clientID"`	// 客户端名称标识符
	ClientUrl  string   `json:"clientUrl"`  // 客户端接收reply的地址
	Operation  string   `json:"operation"`  // 包含合约名称/合约函数名称
	Args       [][]byte `json:"args"`       // 合约函数参数
	SequenceID int64    `json:"sequenceID"` // client产生消息时不需要填，共识过程会为其添加
}
```
完成共识的客户端请求的执行回执会从`PBFT`集群的主节点回复给客户端`client`, 因为交易执行可能成功也可能失败, 因此客户端有一个变量`successfulTx`专门用于
统计至今为止由`PBFT`集群执行成功的交易的数量。用户可以通过`client.BakcSuccessfulTxCount()`方法随时查看成功执行的交易数量，从而测算TPS。

- 测试案例

测试案例请见：`/network/node_test.go`

单元测试函数`TestFourNode`中提供的是：(4)共识节点 + (1)客户端 的案例。
