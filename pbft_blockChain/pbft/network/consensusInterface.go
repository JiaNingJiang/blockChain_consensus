package network

import (
	"encoding/json"
	"net/http"
	"time"

	loglogrus "blockChain_consensus/pbftChain/log_logrus"
	"blockChain_consensus/pbftChain/pbft/consensus"

	"github.com/gin-gonic/gin"
)

// 共识节点类
type ConsensusNode struct {
	url  string
	node *Node
}

// 创建一个共识节点(包含Node对象和Server对象)
func NewServer(nodeID string) *ConsensusNode {
	node := NewNode(nodeID)                                           //创建Node对象
	consensusNode := &ConsensusNode{node.NodeNameTable[nodeID], node} //创建Server对象

	router := consensusNode.InitRouter() //返回一个gin路由器

	s := &http.Server{
		Addr:           node.NodeNameTable[nodeID],
		Handler:        router,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	// 启动共识节点的Server对象,监听server.url,获取其他共识节点的消息并提供服务
	loglogrus.Log.Infof("ConsensusNode(%s) will be started at %s...\n", node.NodeName, consensusNode.url)
	go s.ListenAndServe()

	go func() { // 指定时间间隔后向其他节点广播自己的NodeID等信息
		time.Sleep(time.Second)
		// 向其他节点发送自己的的NodeID
		nodeInfoMsg := consensus.NodeInfo{
			NodeID:   node.NodeID,
			NodeName: node.NodeName,
			NodeUrl:  node.NodeNameTable[node.NodeName],
		}

		node.Broadcast(nodeInfoMsg, "/nodeInfo")
	}()

	return consensusNode
}

func (cn *ConsensusNode) InitRouter() *gin.Engine {

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	gin.SetMode("release")

	consensus := r.Group("")

	consensus.POST("/nodeInfo", GetNodeInfo(cn)) // 接收其他节点的标识符(NodeID)
	consensus.POST("/req", GetReq(cn))           // 接收客户端的req
	consensus.POST("/preprepare", GetPrePrepare(cn))
	consensus.POST("/prepare", GetPrepare(cn))
	consensus.POST("/commit", GetCommit(cn))
	consensus.POST("/reply", GetReply(cn))

	return r
}

func GetNodeInfo(cn *ConsensusNode) func(*gin.Context) {
	return func(c *gin.Context) {
		var msg consensus.NodeInfo
		err := json.NewDecoder(c.Request.Body).Decode(&msg)
		if err != nil {
			loglogrus.Log.Errorf("NodeInfo Msg json解码失败,err:%v", err)
			return
		}
		//loglogrus.Log.Infof("当前节点(%s)获取的NodeInfo Msg:  NodeID:(%x) NodeName:(%s) NodeUrl:(%s)\n", cn.node.NodeName, msg.NodeID, msg.NodeName, msg.NodeUrl)

		cn.node.NodeInfoTableMutex.Lock()
		cn.node.NodeInfoTable[msg.NodeID] = msg
		cn.node.NodeInfoTableMutex.Unlock()
	}
}

// 主节点接收client的request消息,进行json解码,解码后的消息输入到node.MsgEntrance管道
func GetReq(cn *ConsensusNode) func(*gin.Context) {
	return func(c *gin.Context) {
		var msg consensus.RequestMsg
		err := json.NewDecoder(c.Request.Body).Decode(&msg)
		if err != nil {
			loglogrus.Log.Errorf("Request Msg json解码失败,err:%v", err)
			return
		}
		loglogrus.Log.Infof("当前节点(%s)获取的Request Msg:  ClientID: %s, Timestamp: %d, Operation: %s\n", cn.node.NodeName, msg.ClientID, msg.Timestamp, msg.Operation)
		cn.node.MsgEntrance <- &msg
	}
}

// 副本节点接收主节点的Pre-Prepare消息,进行json解码,解码后的消息输入到node.MsgEntrance管道
func GetPrePrepare(cn *ConsensusNode) func(*gin.Context) {
	return func(c *gin.Context) {
		var msg consensus.PrePrepareMsg
		err := json.NewDecoder(c.Request.Body).Decode(&msg)
		if err != nil {
			loglogrus.Log.Errorf("Pre-Prepare Msg json解码失败,err:%v", err)
			return
		}
		loglogrus.Log.Infof("当前节点(%s)获取的Pre-prepare Msg:  TxID:(%x), SequenceID: %d, BlockID: (%x), TxCount:(%d)\n", cn.node.NodeName,
			msg.Block.BlockID, msg.SequenceID, msg.Block.BlockID, len(msg.Block.Transactions))

		cn.node.MsgEntrance <- &msg
	}
}

// 接收其他节点的Prepare消息,进行json解码,解码后的消息输入到node.MsgEntrance管道
func GetPrepare(cn *ConsensusNode) func(*gin.Context) {
	return func(c *gin.Context) {
		var msg consensus.VoteMsg
		err := json.NewDecoder(c.Request.Body).Decode(&msg)
		if err != nil {
			loglogrus.Log.Errorf("Prepare Msg json解码失败,err:%v", err)
			return
		}
		loglogrus.Log.Infof("当前节点(%s)获取的Prepare Msg:  NodeName:(%s), NodeID:(%x), BlockID:(%x), SequenceID:(%d)\n", cn.node.NodeName,
			msg.NodeName, msg.NodeID, msg.Block.BlockID, msg.SequenceID)

		cn.node.MsgEntrance <- &msg
	}
}

// 接收其他节点的commit消息,进行json解码,解码后的消息输入到node.MsgEntrance管道
func GetCommit(cn *ConsensusNode) func(*gin.Context) {
	return func(c *gin.Context) {
		var msg consensus.VoteMsg
		err := json.NewDecoder(c.Request.Body).Decode(&msg)
		if err != nil {
			loglogrus.Log.Errorf("Commit Msg json解码失败,err:%v", err)
			return
		}
		loglogrus.Log.Infof("当前节点(%s)获取的Commit Msg: NodeName:(%s), NodeID:(%x), BlockID:(%x), SequenceID:(%d)\n", cn.node.NodeName,
			msg.NodeName, msg.NodeID, msg.Block.BlockID, msg.SequenceID)
		cn.node.MsgEntrance <- &msg
	}
}

// 主节点接收其他共识节点的reply消息,进行json解码,解码后的消息直接调用node.GetReply()进行处理(这里就是打印)
func GetReply(cn *ConsensusNode) func(*gin.Context) {
	return func(c *gin.Context) {
		var msg consensus.ReplyMsg
		err := json.NewDecoder(c.Request.Body).Decode(&msg)
		if err != nil {
			loglogrus.Log.Errorf("Reply Msg json解码失败,err:%v", err)
			return
		}
		//zapConfig.SugarLogger.Debugf("当前节点%s获取的Reply Msg: %s by %s\n", cn.node.NodeID, msg.Result, msg.NodeID)
		cn.node.MsgEntrance <- &msg
		//cn.node.GetReply(&msg)
	}
}
