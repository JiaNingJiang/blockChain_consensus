package network

import (
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"strings"
	"sync"
	"time"

	"blockChain_consensus/pbftChain/block"
	"blockChain_consensus/pbftChain/common"
	"blockChain_consensus/pbftChain/crypto"
	loglogrus "blockChain_consensus/pbftChain/log_logrus"
	"blockChain_consensus/pbftChain/pbft/consensus"
	"blockChain_consensus/pbftChain/pool"
	"blockChain_consensus/pbftChain/utils"
)

const (
	MinAccumulatedTx = 20 // 启动一轮共识最少需要的交易数
)

var (
	MaxWaitingTx = 10 * time.Second // 启动一轮共识最多等待的时间
)

type Node struct {
	prvKey *ecdsa.PrivateKey
	pubKey *ecdsa.PublicKey
	NodeID common.NodeID // 本地节点的NodeID(用于区块和交易的节点标识符)

	NodeName      string            // 本地节点的名称(用于共识节点间通信的节点标识符)
	NodeNameTable map[string]string // key=nodeID, value=url  记录其他节点的信息

	NodeInfoTable      map[common.NodeID]consensus.NodeInfo
	NodeInfoTableMutex sync.RWMutex

	View         *View            // 当前视图状态
	CurrentState *consensus.State // 当前共识状态

	CommittedMsgs []*consensus.RequestMsg // 存放所有已经完成pbft共识的request消息(完成commit,执行reply之前)
	MsgBuffer     *MsgBuffer              // 缓存池
	MsgEntrance   chan interface{}        // 与业务二进行通信的管道
	MsgDelivery   chan interface{}
	Alarm         chan bool

	// 区块链相关信息
	CurrentVersion common.Hash // 头区块哈希值
	CurrentHeight  uint64      // 当前区块高度
	TxCount        uint64      // 已上链的交易总数

	WaitingTransactions      []block.Transaction // 等待进行共识的交易(交易由客户端的request生成)
	WaitingTransactionsMutex sync.RWMutex

	LastConsensusFinTime time.Time // 上次共识结束的时间

}

// 消息缓存池
type MsgBuffer struct {
	ReqMsgs        *pool.RequestMsgPool
	PrePrepareMsgs *pool.PrePrepareMsgPool
	PrepareMsgs    *pool.PrepareMsgPool
	CommitMsgs     *pool.CommitMsgPool
	ReplyMsgs      *pool.ReplyMsgPool
}

// 当前视图
type View struct {
	ID          uint64        //视图ID
	PrimaryName string        //主节点nodeName
	PrimaryID   common.NodeID //主节点nodeID
}

const ResolvingTimeDuration = time.Millisecond * 1000 // 1 second.
const f = 1                                           // pbft共识所需的参数f

// 根据NodeID创建一个新节点:
// 1.创建必要的节点资源NodeID/NodeTable/View/CurrentState/CommittedMsgs/MsgBuffer)
// 2.开启必要的处理协程 :
//
//	2.1 go node.dispatchMsg() 接收协程
//	2.2 go node.alarmToDispatcher() 定时器触发共识开启协程,另一种方式触发node.dispatchMsg()
//	2.3 go node.resolveMsg() 消息处理协程
func NewNode(nodeName string) *Node {
	const viewID = 10000000000 // 临时用视图ID

	node := &Node{
		// Hard-coded for test.
		NodeName: nodeName,
		NodeNameTable: map[string]string{
			"MainNode":     "127.0.0.1:1111",
			"ReplicaNode1": "127.0.0.1:1112",
			"ReplicaNode2": "127.0.0.1:1113",
			"ReplicaNode3": "127.0.0.1:1114",
		},

		NodeInfoTable: make(map[common.NodeID]consensus.NodeInfo),

		View: &View{
			ID:          viewID,
			PrimaryName: "MainNode",
		},

		// Consensus-related struct
		CurrentState:  nil,
		CommittedMsgs: make([]*consensus.RequestMsg, 0),
		MsgBuffer: &MsgBuffer{
			ReqMsgs:        pool.NewReqMsgPool(),
			PrePrepareMsgs: pool.NewPPMsgPool(),
			PrepareMsgs:    pool.NewPreMsgPool(),
			CommitMsgs:     pool.NewCommitMsgPool(),
			ReplyMsgs:      pool.NewRyMsgPool(),
		},

		// Channels
		MsgEntrance: make(chan interface{}),
		MsgDelivery: make(chan interface{}),
		Alarm:       make(chan bool),

		LastConsensusFinTime: time.Now(),
	}

	if prv, err := crypto.GenerateKey(); err != nil { // 1.生成私钥
		loglogrus.Log.Errorf("[Network] 当前节点无法无法生成私钥,err:%v\n", err)
		os.Exit(1)
	} else {
		node.prvKey = prv
		node.pubKey = prv.Public().(*ecdsa.PublicKey) // 2.生成公钥
		node.NodeID = crypto.KeytoNodeID(node.pubKey) // 3.生成NodeID

		node.NodeInfoTable[node.NodeID] = consensus.NodeInfo{NodeID: node.NodeID, NodeName: node.NodeName, NodeUrl: node.NodeNameTable[node.NodeName]}
	}

	loglogrus.Log.Infof("[Network] 节点生成成功,NodeName(%s),NodeID(%x),url:%s\n", node.NodeName, node.NodeID, node.NodeNameTable[node.NodeName])

	// Start message dispatcher
	go node.dispatchMsg() //负责从node.MsgEntrance管道读取业务二产生消息存放到对应消息池中

	// Start alarm trigger
	go node.alarmToDispatcher() //定时扫描消息池中的消息,传递给协程3进行共识处理

	// Start message resolver
	go node.resolveMsg()

	// 负责利用来自客户端的请求启动一次共识
	go node.transactionConsensus()

	return node
}

// 向其他共识节点发送共识消息
func send(url string, msg []byte) {
	buff := bytes.NewBuffer(msg)
	http.Post("http://"+url, "application/json", buff)
}

// 将传入的共识消息进行json编码,然后广播(http Post)给其他所有的共识节点
func (node *Node) Broadcast(msg interface{}, path string) map[string]error {
	errorMap := make(map[string]error)

	for nodeName, url := range node.NodeNameTable { //遍历node.NodeTable,获取其他共识节点的nodeID和url
		if nodeName == node.NodeName { //跳过本地节点
			continue
		}

		jsonMsg, err := json.Marshal(msg) //对需要进行广播的消息进行json编码
		if err != nil {
			errorMap[nodeName] = err
			continue
		}

		send(url+path, jsonMsg) //发送到其他共识节点的相应目录下(不同目录存放不同阶段的共识消息)
	}

	if len(errorMap) == 0 {
		return nil
	} else {
		return errorMap
	}
}

// 向当前view的主节点发送reply消息
func (node *Node) Reply(msg *consensus.ReplyMsg) error {
	// Print all committed messages.
	for _, value := range node.CommittedMsgs {
		loglogrus.Log.Infof("通过PBFT完成共识, RequestMsg --- clientID:%s 的 Requst Msg:  %d, %s, %d\n", value.ClientID, value.Timestamp, value.Operation, value.SequenceID)
	}

	jsonMsg, err := json.Marshal(msg)
	if err != nil {
		return err
	}

	// zapConfig.SugarLogger.Debugf("目标url:%s", node.NodeTable[node.View.Primary])
	send(node.NodeNameTable[node.View.PrimaryName]+"/reply", jsonMsg) //向当前view的主节点发送reply回复,需要主节点将搜集的reply消息回复给对应的client

	return nil
}

func (node *Node) transactionConsensus() {
	cycle := time.NewTicker(2 * time.Second)

	for {
		select {
		case <-cycle.C:
			if node.CurrentState != nil && node.CurrentState.CurrentStage != consensus.Idle { // 不是并行的共识,因此一旦检测到当前节点已经在共识了,就不能再启动一轮新的共识
				continue
			}

			if len(node.WaitingTransactions) >= MinAccumulatedTx { // 情况一：主节点搜集到了来自客户端足够数量的请求,可以组装一个区块开始共识
				node.WaitingTransactionsMutex.Lock()
				loglogrus.Log.Infof("当前节点(%s)收集的交易数量为(%d),可以构成一个区块\n", node.NodeName, len(node.WaitingTransactions))
				prePrepareMsg := node.CurrentState.StartConsensus(node.NodeID, node.NodeName, node.WaitingTransactions, node.prvKey) //根据获取的客户端request消息,产生新一轮共识需要使用的prePrepareMsg和State对象(将request消息记录到State对象中)
				if prePrepareMsg != nil {
					// 附加自己对区块的数字签名
					prePrepareMsg.Block.DigitalSignature(prePrepareMsg.Block.BlockID, node.prvKey)
					node.Broadcast(prePrepareMsg, "/preprepare") //向其他节点广播新产生的pre-prepare
					loglogrus.Log.Infof("当前节点id:(%s) 已完成 Pre-prepare阶段\n", node.NodeName)
				}

				// 清空收集的交易
				node.WaitingTransactions = make([]block.Transaction, 0)
				node.WaitingTransactionsMutex.Unlock()

			} else if node.CurrentState != nil && len(node.WaitingTransactions) != 0 && time.Now().Sub(node.LastConsensusFinTime) > MaxWaitingTx { // 情况二: 主节点迟迟得不到足够数量的客户端请求,因此在规定时间间隔后主动启动一次共识

				node.WaitingTransactionsMutex.Lock()
				loglogrus.Log.Infof("当前节点(%s)收集的交易数量为(%d),不足以构成一个区块,但是到达了规定的共识间隔,因此主动发起一轮共识\n", node.NodeName, len(node.WaitingTransactions))
				prePrepareMsg := node.CurrentState.StartConsensus(node.NodeID, node.NodeName, node.WaitingTransactions, node.prvKey) //根据获取的客户端request消息,产生新一轮共识需要使用的prePrepareMsg和State对象(将request消息记录到State对象中)
				if prePrepareMsg != nil {
					// 附加自己对区块的数字签名
					prePrepareMsg.Block.DigitalSignature(prePrepareMsg.Block.BlockID, node.prvKey)
					node.Broadcast(prePrepareMsg, "/preprepare") //向其他节点广播新产生的pre-prepare
					loglogrus.Log.Infof("当前节点id:(%s) 已完成 Pre-prepare阶段\n", node.NodeName)
				}
				// 清空收集的交易
				node.WaitingTransactions = make([]block.Transaction, 0)
				node.WaitingTransactionsMutex.Unlock()
			} else { // 情况三: 既没有收集到足够数量的请求,也没有达到规定的时间间隔
				//loglogrus.Log.Infof("当前节点(%s)收集的交易数量为(%d),不足以构成一个区块\n", node.NodeName, len(node.WaitingTransactions))
			}
		default:
			continue
		}
	}
}

// 主节点根据获取的客户端requestMsg,开启新一轮pbft共识(即产生pre-prepare消息并广播给其他共识节点)
func (node *Node) GetReq(reqMsg *consensus.RequestMsg) error {
	loglogrus.Log.Infof("接收的Requst Msg: ClientID: %s, Timestamp: %d, Operation: %s\n", reqMsg.ClientID, reqMsg.Timestamp, reqMsg.Operation)

	// Create a new state for the new consensus.
	if node.CurrentState == nil {
		err := node.createStateForNewConsensus() //初次运行，主节点需要为当前视图创建一个State对象
		if err != nil {
			return err
		}
	}

	// 必须搜集到足够数量的客户端Request 或者达到了时间上限 才行开启一轮共识
	// 根据客户端的request组装交易
	op := strings.Split(reqMsg.Operation, "::")
	newTx := block.NewTransaction(reqMsg.ClientID, node.NodeID, 0, node.CurrentVersion, op[0], op[1], reqMsg.Args)

	txHash := newTx.Hash() // 计算交易哈希值
	newTx.DigitalSignature(txHash, node.prvKey)
	newTx.TimeStamp = reqMsg.Timestamp

	node.WaitingTransactionsMutex.Lock()
	node.WaitingTransactions = append(node.WaitingTransactions, *newTx)
	node.WaitingTransactionsMutex.Unlock()

	node.MsgBuffer.ReqMsgs.DelReqMsg(reqMsg.Hash()) //主节点删除消息池中的requestMsg(因为包含此请求的交易已经被存储)

	return nil
}

// GetPrePrepare can be called when the node's CurrentState is nil.
// Consensus start procedure for normal participants.
// 副本节点根据从主节点获取的pre-prepare消息,创建针对此pre-prepareMsg包含的requestMsg的State对象,同时产生prepare消息,最后将prepareMsg广播发送给其他共识节点
func (node *Node) GetPrePrepare(prePrepareMsg *consensus.PrePrepareMsg) error {
	loglogrus.Log.Infof("接收的Pre-prepare Msg: SenderName: %s, SenderID: %x, BlockID: %x\n", prePrepareMsg.NodeName,
		prePrepareMsg.NodeID, prePrepareMsg.Block.BlockID)

	if node.CurrentState == nil {
		err := node.createStateForNewConsensus() //初次运行，副本节点需要为当前视图创建一个State对象
		if err != nil {
			return err
		}
	}

	nodeIDMap := make(map[common.NodeID]struct{})
	node.NodeInfoTableMutex.RLock()
	for nodeID, _ := range node.NodeInfoTable {
		nodeIDMap[nodeID] = struct{}{}
	}
	node.NodeInfoTableMutex.RUnlock()

	prePareMsg, err := node.CurrentState.PrePrepare(prePrepareMsg, nodeIDMap, node.NodeID, node.NodeName) //处理本次传入的pre-prepare消息,产生prepare阶段消息,同时将pre-prepare消息中的request消息记录到State对象中
	if err != nil {
		return err
	}

	if prePareMsg != nil {
		// Attach node ID to the message
		prePareMsg.NodeName = node.NodeName //追加自己的NodeID添加到prepareMsg消息中

		// 附加上自己的数字签名
		prePareMsg.Block.DigitalSignature(prePareMsg.Block.BlockID, node.prvKey)

		loglogrus.Log.Infof("当前节点id:%s 已完成 Pre-prepare阶段\n", node.NodeName)
		node.Broadcast(prePareMsg, "/prepare")                              //将prepareMsg广播发送给其他共识节点
		node.MsgBuffer.PrePrepareMsgs.DelPPMsg(prePrepareMsg.Block.BlockID) //副本节点删除消息池中的pre-prepare消息(因为此消息已经被State对象记录)
	}

	return nil
}

// 节点收集其他共识节点发送的prepareMsg,一旦收到的prepareMsg 数目大于等于 2*f 就可以组建commit消息,并广播给其他共识节点
func (node *Node) GetPrepare(prepareMsg *consensus.VoteMsg) error {
	loglogrus.Log.Infof("当前节点(%s) 接收的Prepare Msg: NodeName: %s\n", node.NodeName, prepareMsg.NodeName)

	nodeIDMap := make(map[common.NodeID]struct{})
	node.NodeInfoTableMutex.RLock()
	for nodeID, _ := range node.NodeInfoTable {
		nodeIDMap[nodeID] = struct{}{}
	}
	node.NodeInfoTableMutex.RUnlock()

	commitMsg, err := node.CurrentState.Prepare(prepareMsg, nodeIDMap, node.prvKey, node.NodeID, node.NodeName) //处理本次传入的prepare消息
	if err != nil {
		return err
	}

	if commitMsg != nil { //如果prepare共识阶段目标已达成,将组建的commit消息广播发送给其他共识节点
		// Attach node ID to the message
		commitMsg.NodeName = node.NodeName

		// 附加上自己的数字签名
		commitMsg.Block.DigitalSignature(commitMsg.Block.BlockID, node.prvKey)

		loglogrus.Log.Infof("当前节点Name:%s 已完成 Prepare阶段", node.NodeName)
		node.Broadcast(commitMsg, "/commit")
		node.MsgBuffer.PrepareMsgs.DelAllPreMsg() //当前节点删除消息池中的所有prepare消息(因为消息已经被State对象记录)
		return utils.MSGENOUGH
	}

	return nil
}

// 节点收集其他共识节点发送的commitMsg,一旦收到的commitMsg 数目大于等于 2*f 就可以组建reply消息,发送给主节点
func (node *Node) GetCommit(commitMsg *consensus.VoteMsg) error {
	loglogrus.Log.Infof("当前节点(%s) 接收的Commit Msg: NodeName: %s\n", node.NodeName, commitMsg.NodeName)

	nodeIDMap := make(map[common.NodeID]struct{})
	node.NodeInfoTableMutex.RLock()
	for nodeID, _ := range node.NodeInfoTable {
		nodeIDMap[nodeID] = struct{}{}
	}
	node.NodeInfoTableMutex.RUnlock()

	replyMsg, err := node.CurrentState.Commit(commitMsg, nodeIDMap, node.prvKey, node.NodeID, node.NodeName) //处理本次传入的commit消息
	if err != nil {
		return err
	}

	if replyMsg != nil { //如果commit共识阶段目标已达成,将组建的reply消息广播发送主节点
		// if committedMsg == nil { //committedMsg就是reply消息针对的request消息
		// 	return errors.New("committed message is nil, even though the reply message is not nil")
		// }

		// // Save the last version of committed messages to node.
		// node.CommittedMsgs = append(node.CommittedMsgs, committedMsg) //将完成共识的request消息添加到node.CommittedMsgs消息池中

		// 清空State对象本次共识所缓存的除了commitLog外的log消息(node.CommittedMsgs已经记录了完成共识的request消息)
		node.CurrentState.MsgLogs.ReqMsg = nil
		node.CurrentState.MsgLogs.PrepareMsgs = make(map[common.NodeID]consensus.VoteMsg)
		node.CurrentState.MsgLogs.CommitMsgs = make(map[common.NodeID]consensus.VoteMsg)

		loglogrus.Log.Infof("当前节点id:(%s) 已完成 Commit阶段", node.NodeName)

		node.Reply(replyMsg) //发送reply消息给主节点

		if node.NodeName != node.View.PrimaryName { //副本节点已经完成共识

			node.CurrentState.CurrentStage = consensus.Idle //重新等待下一次共识开始
		}

		node.MsgBuffer.CommitMsgs.DelAllCommitMsg() //当前节点删除消息池中的所有commit消息(因为消息已经被State对象记录)
		return utils.MSGENOUGH
	}

	return nil
}

func (node *Node) GetReply(msg *consensus.ReplyMsg) (string, common.NodeID, []consensus.ExcuteResult, error) {

	for _, res := range msg.ResultSet {
		loglogrus.Log.Infof("接收的Result Msg: %v by %s\n", res, msg.NodeName)
	}

	return node.CurrentState.Reply(msg) //处理本次传入的 reply 消息

	//replicaNodeName, replicaNodeID, replicaNodeResult, err := node.CurrentState.Reply(msg) //处理本次传入的 reply 消息

}

// 为需要进行共识的客户端的request提供 State对象(包含当前viewID/SequenceID)
func (node *Node) createStateForNewConsensus() error {
	// Check if there is an ongoing consensus process.
	if node.CurrentState != nil {
		return errors.New("another consensus is ongoing")
	}

	// Get the last sequence ID
	//
	var lastSequenceID int64
	if len(node.CommittedMsgs) == 0 {
		lastSequenceID = -1
	} else {
		lastSequenceID = node.CommittedMsgs[len(node.CommittedMsgs)-1].SequenceID
	}

	// Create a new state for this new consensus process in the Primary
	node.CurrentState = consensus.CreateState(node.View.ID, lastSequenceID, node.NodeName, node.NodeID)

	return nil
}

// 消息接收与调度
func (node *Node) dispatchMsg() {
	for {
		select {
		case msg := <-node.MsgEntrance: //将获取的消息存到对应的消息池中
			err := node.routeMsg(msg)
			if err != nil {
				loglogrus.Log.Errorln(err)
				// TODO: send err to ErrorChannel
			}
		case <-node.Alarm: //定期启动新一轮共识(结合当前消息池和共识状态),通过从消息池中取出消息进行共识
			err := node.routeMsgWhenAlarmed()
			if err != nil {
				loglogrus.Log.Errorln(err)
				// TODO: send err to ErrorChannel
			}
		}
	}
}

// 将不同类型的消息添加到各自的消息池中
func (node *Node) routeMsg(msg interface{}) []error {
	switch msg.(type) {
	case *consensus.RequestMsg: //收到客户端的request消息

		reqMsg, ok := msg.(*consensus.RequestMsg)

		if ok {
			node.MsgBuffer.ReqMsgs.AddReqMsg(*reqMsg)
		}
		//zapConfig.SugarLogger.Debugf("requestMsgPool --- clientID:%s,req:%v", reqMsg.ClientID, node.MsgBuffer.ReqMsgs.GetReqMsgByClientID(reqMsg.ClientID))
	case *consensus.PrePrepareMsg: //收到主节点的pre-prepare消息

		ppMsg, ok := msg.(*consensus.PrePrepareMsg)
		if ok {
			node.MsgBuffer.PrePrepareMsgs.AddPPMsg(*ppMsg)
		}
		//zapConfig.SugarLogger.Debugf("PrePrepareMsgPool --- digest:%s,req:%v", ppMsg.Digest, node.MsgBuffer.PrePrepareMsgs.GetPPMsgByDigest(ppMsg.Digest))
	case *consensus.VoteMsg: //收到其他节点的PrepareMsg/CommitMsg消息
		loglogrus.Log.Infof("[network] 当前节点(%s)接收到 VoteMsg , MsgType:%v ,来源节点(%s)\n", node.NodeName, msg.(*consensus.VoteMsg).MsgType, msg.(*consensus.VoteMsg).NodeName)
		if msg.(*consensus.VoteMsg).MsgType == consensus.PrepareMsg { //收到PrepareMsg消息

			preMsg, ok := msg.(*consensus.VoteMsg)
			if ok {
				node.MsgBuffer.PrepareMsgs.AddPreMsg(*preMsg)
			}

		} else if msg.(*consensus.VoteMsg).MsgType == consensus.CommitMsg { //收到CommitMsg消息
			cmMsg, ok := msg.(*consensus.VoteMsg)
			if ok {
				node.MsgBuffer.CommitMsgs.AddCommitMsg(*cmMsg)
			}
		}

	case *consensus.ReplyMsg:

		ryMsg, ok := msg.(*consensus.ReplyMsg)
		if ok {
			node.MsgBuffer.ReplyMsgs.AddRyMsg(*ryMsg)
		}
		//zapConfig.SugarLogger.Debugf("ReplyMsgPool --- clientID:%s,req:%v", ryMsg.ClientID, node.MsgBuffer.ReplyMsgs.GetRyMsgByClientID(ryMsg.NodeID))
	}

	return nil
}

// 定时扫描所有的消息池,根据当前共识状态决定是否处理
func (node *Node) routeMsgWhenAlarmed() []error {

	if node.CurrentState == nil || node.CurrentState.CurrentStage == consensus.Idle { //还未开始共识

		// 主节点负责读取request消息池
		if node.MsgBuffer.ReqMsgs.MsgNum() != 0 {
			msgs := node.MsgBuffer.ReqMsgs.GetAllReqMsg()

			for i := 0; i < node.MsgBuffer.ReqMsgs.MsgNum(); i++ {
				node.MsgBuffer.ReqMsgs.DelReqMsg(msgs[i].Hash())
			}

			node.MsgDelivery <- msgs
		}
		// 副本节点负责读取pre-prepare消息池
		if node.MsgBuffer.PrePrepareMsgs.MsgNum() != 0 {

			msgs := node.MsgBuffer.PrePrepareMsgs.GetAllPPMsg()

			for i := 0; i < node.MsgBuffer.PrePrepareMsgs.MsgNum(); i++ {
				node.MsgBuffer.PrePrepareMsgs.DelPPMsg(msgs[i].Block.BlockID)
				//zapConfig.SugarLogger.Debugf("消息扫描  PrePrepareMsg ---  Digest: %s, SequenceID: %d\n", msgs[i].Digest, msgs[i].SequenceID)
			}
			node.MsgDelivery <- msgs
		}

	} else { //正处于pbft共识阶段中
		switch node.CurrentState.CurrentStage {
		case consensus.PrePrepared: //刚刚完成pbft的pre-prepare阶段,需要进入prepare阶段,取出MsgBuffer.PrepareMsgs缓存池中的全部消息，输入到node.MsgDelivery管道

			if node.MsgBuffer.PrepareMsgs.MsgNum() >= 2*f { //必须保证当前节点收集到了至少 2*f 个节点的PrepareMsg

				//zapConfig.SugarLogger.Debugf("消息扫描 len of PrepareMsg:%d", node.MsgBuffer.PrepareMsgs.MsgNum())
				msgs := node.MsgBuffer.PrepareMsgs.GetAllPreMsg()

				for i := 0; i < node.MsgBuffer.PrepareMsgs.MsgNum(); i++ {
					node.MsgBuffer.PrepareMsgs.DelPreMsg(msgs[i].NodeName)
					//zapConfig.SugarLogger.Debugf("消息扫描  PrepareMsg ---  Digest: %s, SequenceID: %d NodeID: %s \n", msgs[i].Digest, msgs[i].SequenceID, msgs[i].NodeID)
				}

				node.MsgDelivery <- msgs
			}

		case consensus.Prepared: //刚刚完成pbft的prepare阶段,需要进入commit阶段,取出MsgBuffer.CommitMsgs缓存池中的全部消息，输入到node.MsgDelivery管道

			if node.MsgBuffer.CommitMsgs.MsgNum() >= 2*f { //必须保证当前节点收集到了至少 2*f 个节点的CommitMsg
				msgs := node.MsgBuffer.CommitMsgs.GetAllCmMsg()

				for i := 0; i < node.MsgBuffer.CommitMsgs.MsgNum(); i++ {
					node.MsgBuffer.CommitMsgs.DelCommitMsg(msgs[i].NodeName)
					//zapConfig.SugarLogger.Debugf("消息扫描  CommitMsg ---  Digest: %s, SequenceID: %d NodeID: %s \n", msgs[i].Digest, msgs[i].SequenceID, msgs[i].NodeID)

				}

				node.MsgDelivery <- msgs
			}
		case consensus.Committed: //完成了pbft的commit阶段,需要进入reply阶段,取出MsgBuffer.ReplyMsgs缓存池中的全部消息，输入到node.MsgDelivery管道
			//zapConfig.SugarLogger.Debugf("当前共识状态为:consensus.Committed,消息数为%d", node.MsgBuffer.ReplyMsgs.MsgNum())

			if node.MsgBuffer.ReplyMsgs.MsgNum() >= f+1 { //必须保证当前主节点收集到了至少 f+1 个节点的ReplyMsg(可以是主节点自己的reply)
				msgs := node.MsgBuffer.ReplyMsgs.GetAllRyMsg()

				for i := 0; i < node.MsgBuffer.ReplyMsgs.MsgNum(); i++ {
					node.MsgBuffer.ReplyMsgs.DelRyMsg(msgs[i].NodeID)
					//zapConfig.SugarLogger.Debugf("消息扫描  ReplyMsg --- Reply Msg: %s by %s\n", msgs[i].Result, msgs[i].NodeID)

				}

				node.MsgDelivery <- msgs
			}
		}
	}

	return nil
}

// (被定时器触发)循环从node.MsgDelivery管道中取出消息,进行分类处理
func (node *Node) resolveMsg() {
	for {
		// Get buffered messages from the dispatcher.
		msgs := <-node.MsgDelivery

		switch msgs.(type) {
		case []consensus.RequestMsg: //client发送的request消息

			// for _, v := range msgs.([]*consensus.RequestMsg) {
			// 	zapConfig.SugarLogger.Debugf("准备处理RequestMsg ---  ClientID: %s, Operation: %s, SequenceID: %d\n", v.ClientID, v.Operation, v.SequenceID)
			// }
			var tempMsgs []consensus.RequestMsg = msgs.([]consensus.RequestMsg)

			errs := node.resolveRequestMsg(tempMsgs) // 主节点根据此request消息产生pre-prepare消息并广播给其他共识节点
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
				}
				// TODO: send err to ErrorChannel
			}
		case []consensus.PrePrepareMsg: //pbft一阶段产生的PrePrepareMsg消息
			var tempMsgs []consensus.PrePrepareMsg = msgs.([]consensus.PrePrepareMsg)

			errs := node.resolvePrePrepareMsg(tempMsgs) // 副本节点根据此Pre-PrepareMsg消息,生成共识处理器以及prepareMsg并进行广播
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
				}
				// TODO: send err to ErrorChannel
			}
		case []consensus.VoteMsg: //pbft二三阶段产生的PrepareMsg消息和CommitMsg消息

			var tempMsgs []consensus.VoteMsg = msgs.([]consensus.VoteMsg)
			if len(tempMsgs) == 0 {
				break
			}

			//判断voteMsg到底是PrepareMsg消息还是CommitMsg消息,分类处理
			if tempMsgs[0].MsgType == consensus.PrepareMsg {
				errs := node.resolvePrepareMsg(tempMsgs) // 节点根据收集到的prepareMsg,生成commitMsg进行广播
				if len(errs) != 0 {
					for _, err := range errs {
						fmt.Println(err)
					}
					// TODO: send err to ErrorChannel
				}
			} else if tempMsgs[0].MsgType == consensus.CommitMsg {
				errs := node.resolveCommitMsg(tempMsgs) // 节点根据收集到的commitMsg,生成replyMsg发送给主节点(主节点自己也会给自己发送一份)
				if len(errs) != 0 {
					for _, err := range errs {
						fmt.Println(err)
					}
					// TODO: send err to ErrorChannel
				}
			}
		case []consensus.ReplyMsg: //pbft完成共识的reply消息
			errs := node.resolveReplyMsg(msgs.([]consensus.ReplyMsg))
			if len(errs) != 0 {
				for _, err := range errs {
					fmt.Println(err)
				}
				// TODO: send err to ErrorChannel
			}

		}
	}
}

// 每经过ResolvingTimeDuration时长,产生node.Alarm信号,被动开启新一轮共识
func (node *Node) alarmToDispatcher() {
	for {
		time.Sleep(ResolvingTimeDuration)
		node.Alarm <- true
	}
}

// 处理传入的所有RequestMsg,为每一条RequestMsg开启新一轮pbft共识(产生pre-prepare消息并广播给其他共识节点)
func (node *Node) resolveRequestMsg(msgs []consensus.RequestMsg) []error {
	errs := make([]error, 0)

	// Resolve messages
	for _, reqMsg := range msgs {
		err := node.GetReq(&reqMsg)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}

// 处理传入的所有Pre-PrepareMsg,根据Pre-Prepare Msg组建Prepare Msg并广播给其他共识节点
func (node *Node) resolvePrePrepareMsg(msgs []consensus.PrePrepareMsg) []error {
	errs := make([]error, 0)

	// Resolve messages
	for _, prePrepareMsg := range msgs {
		err := node.GetPrePrepare(&prePrepareMsg)
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}

// 处理传入的所有PrepareMsg,根据Prepare Msg组建Commit Msg并广播给其他共识节点
func (node *Node) resolvePrepareMsg(msgs []consensus.VoteMsg) []error {
	errs := make([]error, 0)
	// Resolve messages
	for _, prepareMsg := range msgs {
		err := node.GetPrepare(&prepareMsg) // 一次传入一个prepareMsg,当凑够2f条时返回utils.MSGENOUGH
		if err == utils.MSGENOUGH {         //需要获取的共识消息数已达到要求
			break
		}
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}

// 处理传入的所有Commit Msg,根据Commit Msg组建Reply Msg并广播给其他共识节点
func (node *Node) resolveCommitMsg(msgs []consensus.VoteMsg) []error {
	errs := make([]error, 0)
	// Resolve messages
	for _, commitMsg := range msgs {
		err := node.GetCommit(&commitMsg)
		if err == utils.MSGENOUGH { //需要获取的共识消息数已达到要求
			break
		}
		if err != nil {
			errs = append(errs, err)
		}
	}

	if len(errs) != 0 {
		return errs
	}

	return nil
}

// 处理传入的所有Reply Msg,将其全部发送给client(这里就直接打印就可以了)
func (node *Node) resolveReplyMsg(msgs []consensus.ReplyMsg) []error {

	executeResultMap := make(map[common.NodeID][]consensus.ExcuteResult) // 区分从每个节点接收到的reply
	finalResMap := make(map[consensus.ExcuteResult]int)                  // 统计具有相同执行结果result的reply

	// Resolve messages
	for _, replyMsg := range msgs {
		_, replicaNodeID, replicaNodeResultSet, err := node.GetReply(&replyMsg)
		if err == nil {
			executeResultMap[replicaNodeID] = replicaNodeResultSet
		}
	}

	for nodeID, receiptSet := range executeResultMap { // 遍历每个节点区块内的交易执行回执
		succTx := 0
		for _, receipt := range receiptSet { // 遍历指定节点的所有交易执行回执
			if receipt.Error == "nil" {
				succTx++
			}
		}
		//loglogrus.Log.Infof("来自节点(%x) 的区块回执: %v\n", nodeID, receiptSet)
		loglogrus.Log.Infof("来自节点(%x) 的有效交易回执数量: %v\n", nodeID, succTx)
	}

	for _, resSet := range executeResultMap { // 遍历每个节点的reply
		for _, res := range resSet { // 遍历每个relpy的执行结果
			finalResMap[res]++
		}

	}

	for res, count := range finalResMap {
		if count >= f+1 {
			loglogrus.Log.Infof("客户端请求执行成功,执行结果:%v\n", res.Result)
			// TODO:执行成功.向客户端回复执行结果
		} else {
			loglogrus.Log.Infof("客户端请求执行失败\n")
			// TODO:没有足够数量的合法reply,说明执行失败,向客户端回复失败信息
		}
	}

	node.MsgBuffer.ReplyMsgs.DelAllRyMsg()          //完成一轮共识,可以清空消息池
	node.CurrentState.CurrentStage = consensus.Idle //主节点结束本次pbft,重新回到初始状态
	return nil
}
