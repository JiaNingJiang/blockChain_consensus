package pool

import (
	"blockChain_consensus/pbftChain/pbft/consensus"
	"sync"
)

type CommitMsgPool struct {
	CmMsgPool map[string]consensus.VoteMsg //CommitMsg中包含的交易的txID作为key

	poolMutex sync.RWMutex
}

func NewCommitMsgPool() *CommitMsgPool {
	return &CommitMsgPool{
		CmMsgPool: make(map[string]consensus.VoteMsg),
	}
}

func (cmp *CommitMsgPool) AddCommitMsg(cmMsg consensus.VoteMsg) {
	cmp.poolMutex.Lock()
	defer cmp.poolMutex.Unlock()

	cmp.CmMsgPool[cmMsg.NodeName] = cmMsg
	//fmt.Printf("Commit Msg添加消息池成功: 来源NodeID:%s\n", cmMsg.NodeID)
}

func (cmp *CommitMsgPool) DelCommitMsg(nodeName string) {
	cmp.poolMutex.Lock()
	defer cmp.poolMutex.Unlock()

	if _, ok := cmp.CmMsgPool[nodeName]; ok {
		delete(cmp.CmMsgPool, nodeName)
	}
}

func (cmp *CommitMsgPool) DelAllCommitMsg() {
	cmp.poolMutex.Lock()
	defer cmp.poolMutex.Unlock()

	cmp.CmMsgPool = make(map[string]consensus.VoteMsg)
}

func (cmp *CommitMsgPool) MsgNum() int {
	cmp.poolMutex.RLock()
	defer cmp.poolMutex.RUnlock()
	return len(cmp.CmMsgPool)
}

func (cmp *CommitMsgPool) GetCmMsgByDigest(nodeName string) consensus.VoteMsg {
	cmp.poolMutex.RLock()
	defer cmp.poolMutex.RUnlock()

	return cmp.CmMsgPool[nodeName]
}

func (cmp *CommitMsgPool) GetAllCmMsg() []consensus.VoteMsg {
	cmp.poolMutex.RLock()
	defer cmp.poolMutex.RUnlock()

	result := make([]consensus.VoteMsg, 0)
	for _, msg := range cmp.CmMsgPool {
		result = append(result, msg)
		//fmt.Printf("消息池保存的CommitMsg --- Digest: %s, SequenceID: %d NodeID: %s \n", msg.Digest, msg.SequenceID, msg.NodeID)

	}
	return result
}
