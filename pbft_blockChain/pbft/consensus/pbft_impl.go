package consensus

import (
	"blockChain_consensus/pbftChain/block"
	"blockChain_consensus/pbftChain/common"
	"blockChain_consensus/pbftChain/database"
	loglogrus "blockChain_consensus/pbftChain/log_logrus"
	"blockChain_consensus/pbftChain/pbft/contract"
	"crypto/ecdsa"
	"encoding/json"
	"errors"
	"fmt"
	"reflect"
	"sync"
	"time"

	"github.com/syndtr/goleveldb/leveldb"
	"github.com/syndtr/goleveldb/leveldb/storage"
)

type State struct {
	NodeName string
	NodeID   common.NodeID

	ViewID         uint64   //当前视图id
	MsgLogs        *MsgLogs //记录本轮共识产生的log消息
	LastSequenceID int64    //上一轮被共识的request消息的SequenceID
	CurrentStage   Stage    //当前共识阶段

	WorldState      database.Database // 存储世界状态的数据库
	WorldStateMutex sync.RWMutex
}

type MsgLogs struct {
	ReqMsg      []*RequestMsg
	PrepareMsgs map[common.NodeID]VoteMsg
	CommitMsgs  map[common.NodeID]VoteMsg
}

type Stage int

const (
	Idle        Stage = iota // Node is created successfully, but the consensus process is not started yet.
	PrePrepared              // The ReqMsgs is processed successfully. The node is ready to head to the Prepare stage.
	Prepared                 // Same with `prepared` stage explained in the original paper.
	Committed                // Same with `committed-local` stage explained in the original paper.
)

// f: # of Byzantine faulty node
// f = (n­1) / 3
// n = 4, in this case.
const f = 1

// lastSequenceID will be -1 if there is no last sequence ID.
// 根据 视图id 和 上一轮被共识的requestMsg的SequenceID 为本轮共识创建新的共识状态对象State
func CreateState(viewID uint64, lastSequenceID int64, nodeName string, nodeID common.NodeID) *State {
	state := &State{
		NodeName: nodeName,
		NodeID:   nodeID,

		ViewID: viewID,
		MsgLogs: &MsgLogs{
			ReqMsg:      nil,
			PrepareMsgs: make(map[common.NodeID]VoteMsg),
			CommitMsgs:  make(map[common.NodeID]VoteMsg),
		},
		LastSequenceID: lastSequenceID,
		CurrentStage:   Idle,
	}

	if memDB, err := leveldb.Open(storage.NewMemStorage(), nil); err != nil {
		loglogrus.Log.Errorf("当前节点(%s:%x)无法创建内存数据库,err:%v\n", nodeName, nodeID, err)
		return nil
	} else {
		state.WorldState = database.NewSimpleLDB("WorldState", memDB)
	}

	return state

}

// 根据传入的RequestMsg消息,更新本轮共识的State对象,同时返回pre-prepare阶段需要使用的PrePrepareMsg消息
func (state *State) StartConsensus(senderID common.NodeID, senderName string, txSet []block.Transaction, prv *ecdsa.PrivateKey) *PrePrepareMsg {
	// `sequenceID` will be the index of this message.
	sequenceID := time.Now().UnixNano() //PrePrepareMsg消息的sequenceID需要使用当前时间戳为准

	// Find the unique and largest number for the sequence ID
	if state.LastSequenceID != -1 { //要保证本轮需要被共识的request消息的sequenceID大于上一轮达成共识的request消息的sequenceID
		for state.LastSequenceID >= sequenceID {
			sequenceID += 1
		}
	}
	// Change the stage to pre-prepared.
	state.CurrentStage = PrePrepared //更新state对象的共识阶段为Pre-Prepared

	newBlock := block.NewBlock(senderID, txSet)

	return &PrePrepareMsg{
		ViewID:     state.ViewID,
		SequenceID: sequenceID,
		NodeName:   senderName,
		NodeID:     senderID,
		Block:      newBlock,
	}
}

// 副本节点接收主节点发送的pre-prepare消息,验证消息正确性,更新共识状态,生成prepareMsg
func (state *State) PrePrepare(prePrepareMsg *PrePrepareMsg, nodeIDMap map[common.NodeID]struct{}, nodeID common.NodeID, nodeName string) (*VoteMsg, error) {
	// 1.Verify if v, n(a.k.a. sequenceID), d are correct.
	if !state.verifyMsg(prePrepareMsg.ViewID, prePrepareMsg.SequenceID) { //验证此pre-prepare消息的正确性(sequenceID是否正确)
		loglogrus.Log.Warnf("[PBFT] pre-prepare Msg viewID 或 sequenceID 异常\n")
		return nil, errors.New("pre-prepare message is corrupted")
	}

	// 2.验证区块(数字签名和哈希值)
	rblock := prePrepareMsg.Block

	loglogrus.Log.Infof("当前节点(%s)收到的 pre-prepare Msg 中区块包含的数字签名个数为:%d\n", nodeName, len(rblock.Votes))

	sigValids := make([]bool, len(rblock.Votes))
	for i := 0; i < len(sigValids); i++ {
		sigValids[i] = false
	}
	// 3.所有签名都需要进行验证
	for index, sig := range rblock.Votes {
		for nodeID, _ := range nodeIDMap {
			if ValidateSignature(rblock.BlockID, nodeID, sig) {
				sigValids[index] = true
				break
			}
		}
	}

	// 4.仅有通过验证的数字签名才会被保留
	validSigs := make([]block.Signature, 0)
	for index, sigValid := range sigValids {
		if sigValid {
			validSigs = append(validSigs, rblock.Votes[index])
		}
	}
	rblock.Votes = validSigs

	// 5.Change the stage to pre-prepared.
	state.CurrentStage = PrePrepared //改变共识状态

	return &VoteMsg{
		MsgType:    PrepareMsg,
		ViewID:     state.ViewID,
		SequenceID: prePrepareMsg.SequenceID,
		NodeID:     nodeID,
		Block:      rblock,
	}, nil
}

// 1.验证收到的prepare消息是否正确
// 2.将获取的prepareMsg添加到日志消息池MsgLogs(key为来源NodeID)
// 3.判断收集的prepareMsg是否足够,能否进入下一commit共识阶段
// 4.如果可以，组建commit消息
func (state *State) Prepare(prepareMsg *VoteMsg, nodeIDMap map[common.NodeID]struct{}, prv *ecdsa.PrivateKey, nodeID common.NodeID, nodeName string) (*VoteMsg, error) {
	// 1.验证prepare消息是否正确
	if !state.verifyMsg(prepareMsg.ViewID, prepareMsg.SequenceID) {
		return nil, errors.New("prepare message is corrupted")
	}
	// 2.验证区块(数字签名和哈希值)
	rblock := prepareMsg.Block

	// loglogrus.Log.Infof("当前节点(%s)收到来自节点(%s)(%x)的 prepare Msg 中区块包含的数字签名个数为:%d\n", nodeName, prepareMsg.NodeName, prepareMsg.NodeID, len(rblock.Votes))

	sigValids := make([]bool, len(rblock.Votes))
	for i := 0; i < len(sigValids); i++ {
		sigValids[i] = false
	}
	// 3.所有签名都需要进行验证
	for index, sig := range rblock.Votes {
		for nodeID, _ := range nodeIDMap {
			if ValidateSignature(rblock.BlockID, nodeID, sig) {
				sigValids[index] = true
				break
			}
		}
	}
	// 4.仅有通过验证的数字签名才会被保留
	validSigs := make([]block.Signature, 0)
	for index, sigValid := range sigValids {
		if sigValid {
			validSigs = append(validSigs, rblock.Votes[index])
		}
	}
	rblock.Votes = validSigs
	prepareMsg.Block = rblock

	// for index, sig := range rblock.Votes {
	// 	loglogrus.Log.Infof("当前节点(%s)通过验证的数字签名总共有(%d)条,当前为第%d条: %v\n", nodeName, len(rblock.Votes), index, sig)
	// }

	// Append msg to its logs
	state.MsgLogs.PrepareMsgs[prepareMsg.NodeID] = *prepareMsg //将获取的prepareMsg添加到日志消息池MsgLogs(key为来源NodeID)

	// Print current voting status
	loglogrus.Log.Debugf("当前节点(%s)已搜集的Prepare消息Vote Num:%d", nodeName, len(state.MsgLogs.PrepareMsgs))

	validSigSet := make([]block.Signature, 0)

	if state.prepared(nodeName, rblock.BlockID, nodeIDMap, &validSigSet) { //需要判断是否可以进入下一commit共识阶段,如果可以返回组建的commit消息
		// Change the stage to prepared.
		state.CurrentStage = Prepared //更新共识阶段为prepared(已经完成prepare阶段共识)

		rblock.Votes = make([]block.Signature, 0)
		rblock.Votes = append(rblock.Votes, validSigSet...)

		return &VoteMsg{
			MsgType:    CommitMsg,
			ViewID:     state.ViewID,
			SequenceID: prepareMsg.SequenceID,

			NodeID: nodeID,
			Block:  rblock,
		}, nil
	}

	return nil, nil
}

// 1.验证收到的commit消息是否正确
// 2.将获取的commitMsg添加到日志消息池MsgLogs(key为来源NodeID)
// 3.判断收集的commitMsg是否足够,能否进入下一reply共识阶段
// 4.如果可以，组建reply消息,同时还返回reply消息针对的request消息
func (state *State) Commit(commitMsg *VoteMsg, nodeIDMap map[common.NodeID]struct{}, prv *ecdsa.PrivateKey, nodeID common.NodeID, nodeName string) (*ReplyMsg, error) {
	// 1.验证commit消息是否正确
	if !state.verifyMsg(commitMsg.ViewID, commitMsg.SequenceID) {
		return nil, errors.New("commit message is corrupted")
	}
	// 2.验证区块(数字签名和哈希值)
	rblock := commitMsg.Block
	sigValids := make([]bool, len(rblock.Votes))
	for i := 0; i < len(sigValids); i++ {
		sigValids[i] = false
	}
	// 3.所有签名都需要进行验证
	for index, sig := range rblock.Votes {
		for nodeID, _ := range nodeIDMap {
			if ValidateSignature(rblock.BlockID, nodeID, sig) {
				sigValids[index] = true
				break
			}
		}
	}
	// 4.仅有通过验证的数字签名才会被保留
	validSigs := make([]block.Signature, 0)
	for index, sigValid := range sigValids {
		if sigValid {
			validSigs = append(validSigs, rblock.Votes[index])
		}
	}
	rblock.Votes = validSigs
	commitMsg.Block = rblock

	if len(rblock.Votes) < 2*f {
		return nil, errors.New("commit Msg 没有包含足够数量的数字签名")
	}

	// Append msg to its logs
	state.MsgLogs.CommitMsgs[commitMsg.NodeID] = *commitMsg //将获取的commitMsg添加到日志消息池MsgLogs(key为来源NodeID)

	// Print current voting status
	//loglogrus.Log.Debugf("当前节点已搜集的Commit消息Vote Num:%d", len(state.MsgLogs.CommitMsgs))

	validSigSet := make([]block.Signature, 0)
	if state.committed(nodeName, rblock.BlockID, nodeIDMap, &validSigSet) { //需要判断是否可以进入下一reply共识阶段,如果可以返回组建的reply消息

		// 执行区块内的所有交易
		resultSet := make([]ExcuteResult, 0)
		state.WorldStateMutex.Lock()
		for _, tx := range rblock.Transactions {
			res, err := contract.ContractFuncRun(state.WorldState, tx.Contract, tx.Function, tx.Args)
			if err == nil {
				resultSet = append(resultSet, ExcuteResult{tx.Client, tx.TimeStamp, res, "nil"})
			} else {
				resultSet = append(resultSet, ExcuteResult{tx.Client, tx.TimeStamp, res, fmt.Sprintf("%v", err)})
			}

		}
		state.WorldStateMutex.Unlock()

		// TODO:将交易打包成区块,存储到本地

		// Change the stage to prepared.
		state.CurrentStage = Committed              //更新共识阶段为committed(已经完成commit阶段共识)
		state.LastSequenceID = commitMsg.SequenceID //更新State的sequenceID

		rMsg := &ReplyMsg{
			ViewID:    state.ViewID,
			ResultSet: resultSet,
			NodeID:    nodeID,
			NodeName:  nodeName,
		}

		rMsg.DigitalSignature(prv)

		return rMsg, nil
	}

	return nil, nil
}

// 主节点验证来自各个副本节点的reply消息正确
func (state *State) Reply(relpyMsg *ReplyMsg) (string, common.NodeID, []ExcuteResult, error) {

	if !ValidateSignature(relpyMsg.Hash(), relpyMsg.NodeID, relpyMsg.Signature) {
		loglogrus.Log.Warnf("[PBFT] reply Msg 中包含的交易无法通过数字签名验证\n")
		return "", common.NodeID{}, nil, errors.New("reply message is invalid")
	} else {
		return relpyMsg.NodeName, relpyMsg.NodeID, relpyMsg.ResultSet, nil
	}

}

// 传入的参数分别为待检测消息包含的 viewID,sequenceID,数字摘要(检查sequenceID是否正确,request消息的摘要是否正确)
func (state *State) verifyMsg(viewID uint64, sequenceID int64) bool {
	// Wrong view. That is, wrong configurations of peers to start the consensus.
	if state.ViewID != viewID {
		return false
	}

	// Check if the Primary sent fault sequence number. => Faulty primary.
	// TODO: adopt upper/lower bound check.
	if state.LastSequenceID != -1 {
		if state.LastSequenceID >= sequenceID { //待验证的消息不能是已经完成共识的消息(每一条request消息都有唯一的序列号sequenceID, state.LastSequenceID是上一条已经被确认的request消息的序列号)
			return false
		}
	}

	return true
}

// 判断当前节点是否已经完成了prepare共识阶段,可以进入下一commit阶段
// 1.检查State对象是否有作为共识目标的request消息(MsgLogs.ReqMsg是否有)
// 2.检查当前节点是否已经收集到了足够的 prepareMsg (至少需要收集到2f条其他节点的prepare消息)
func (state *State) prepared(nodeName string, blockID common.Hash, nodeIDMap map[common.NodeID]struct{}, validSigs *[]block.Signature) bool {
	// if state.MsgLogs.ReqMsg == nil {
	// 	return false
	// }

	if len(state.MsgLogs.PrepareMsgs) < 2*f {
		loglogrus.Log.Infof("[pbft] 当前节点(%s)没有收集到足够数量(当前数量:%d  目标数量:%d)的 Prepare Msg\n", nodeName, len(state.MsgLogs.PrepareMsgs), 2*f)
		return false
	}

	loglogrus.Log.Infof("当前节点(%s)收集到足够数量的 Prepare Msg,数量为:%d\n", nodeName, len(state.MsgLogs.PrepareMsgs))

	// 1.确保这 2*f条消息中包含的区块哈希值是一样的(这样才是同一个区块)
	blockHash := common.Hash{}
	sameBlockCount := 0

	sigSet := make([]block.Signature, 0) // 获取所有的数字签名(会存在重复)

	for _, vMsg := range state.MsgLogs.PrepareMsgs {

		//loglogrus.Log.Infof("当前节点(%s)收到来自节点(%s)(%x)的区块,包含数字签名:%v\n", nodeName, vMsg.NodeName, vMsg.NodeID, vMsg.Block.Votes)

		sigSet = append(sigSet, vMsg.Block.Votes...)

		if reflect.DeepEqual(blockHash, common.Hash{}) {
			blockHash, _ = vMsg.Block.Hash() // 重新在本地计算一次区块的哈希值
		}
		if reflect.DeepEqual(blockHash, vMsg.Block.BlockID) { // 发现同一个区块,sameBlockCount++
			sameBlockCount++
		}
	}
	if sameBlockCount < 2*f {
		loglogrus.Log.Infof("[pbft] 当前节点(%s) 搜集的 prepare Msg 集合中包含的区块不一致 \n", nodeName)
		return false
	}

	// loglogrus.Log.Infof("[PBFT] 当前节点(%s) 搜集的 数字签名总数:%d\n", nodeName, len(sigSet))
	// for index, sig := range sigSet {
	// 	loglogrus.Log.Infof("[PBFT] 当前节点(%s) 搜集的第(%d)个数字签名:%v\n", nodeName, index, sig)
	// }

	// 2.删除重复的签名
	singleSigSet := make([]block.Signature, 0)
	for _, sig := range sigSet { // 遍历所有数字签名
		isDup := false
		for _, singleSig := range singleSigSet { // 遍历已有的唯一签名
			if reflect.DeepEqual(sig, singleSig) { // 该签名已经存在了
				isDup = true
				break
			}
		}
		if !isDup { // 仅当该签名从未出现才会进行存储
			singleSigSet = append(singleSigSet, sig)
		}
	}

	// for index, sig := range singleSigSet {
	// 	loglogrus.Log.Infof("[PBFT] 当前节点(%s) 有效的第(%d)个数字签名:%v\n", nodeName, index, sig)
	// }

	// // 3.总的有效数字签名个数需要 > 2*f (包含自己的数字签名)
	// sigValids := make([]bool, len(singleSigSet))
	// for i := 0; i < len(sigValids); i++ {
	// 	sigValids[i] = false
	// }

	// for index, sig := range singleSigSet {
	// 	for nodeID, _ := range nodeIDMap {
	// 		if ValidateSignature(blockID, nodeID, sig) {
	// 			sigValids[index] = true
	// 			break
	// 		}
	// 	}
	// }
	// // 4.仅有通过验证的数字签名才会被保留
	// validSigSet := make([]block.Signature, 0)
	// for index, sigValid := range sigValids {
	// 	if sigValid {
	// 		validSigSet = append(validSigSet, singleSigSet[index])
	// 	}
	// }

	// TODO: 一个bug,主节点不知道为什么只能收到两条有效数字签名
	if len(singleSigSet) > 2*f {
		*validSigs = singleSigSet
		return true
	} else {
		loglogrus.Log.Infof("[pbft] 当前节点(%s) 无法从 prepare Msg 集合中搜集到针对区块(%x)的足够数量(当前数量:%d  目标数量:%d)的数字签名 \n", nodeName, blockHash, len(singleSigSet), 2*f+1)
		return false
	}

}

// 判断当前节点是否已经完成了commit共识阶段,可以进入下一reply阶段
// 1.判断是否已经完成prepare阶段
// 2.判断收集的commitMsg消息数是否足够(至少需要收集到2f条其他节点的commit消息)
func (state *State) committed(nodeName string, blockID common.Hash, nodeIDMap map[common.NodeID]struct{}, validSigs *[]block.Signature) bool {
	if !state.prepared(nodeName, blockID, nodeIDMap, validSigs) {
		return false
	}

	if len(state.MsgLogs.CommitMsgs) < 2*f {
		return false
	}

	return true
}

// 传入消息进行json编码并计算hash值返回
func digest(object interface{}) (string, error) {
	msg, err := json.Marshal(object)

	if err != nil {
		return "", err
	}

	return Hash(msg), nil
}
