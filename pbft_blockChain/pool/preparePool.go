package pool

import (
	"pbft_blockchain/pbft/consensus"
	"sync"
)

type PrepareMsgPool struct {
	PreMsgPool map[string]consensus.VoteMsg //PrepareMsg中包含的交易的txID作为key

	poolMutex sync.RWMutex
}

func NewPreMsgPool() *PrepareMsgPool {
	return &PrepareMsgPool{
		PreMsgPool: make(map[string]consensus.VoteMsg),
	}
}

func (pmp *PrepareMsgPool) AddPreMsg(preMsg consensus.VoteMsg) {
	pmp.poolMutex.Lock()
	defer pmp.poolMutex.Unlock()

	pmp.PreMsgPool[preMsg.NodeName] = preMsg
}

func (pmp *PrepareMsgPool) DelPreMsg(nodeName string) {
	pmp.poolMutex.Lock()
	defer pmp.poolMutex.Unlock()

	if _, ok := pmp.PreMsgPool[nodeName]; ok {
		delete(pmp.PreMsgPool, nodeName)
	}
}

func (pmp *PrepareMsgPool) DelAllPreMsg() {
	pmp.poolMutex.Lock()
	defer pmp.poolMutex.Unlock()

	pmp.PreMsgPool = make(map[string]consensus.VoteMsg)
}

func (pmp *PrepareMsgPool) MsgNum() int {
	pmp.poolMutex.RLock()
	defer pmp.poolMutex.RUnlock()
	return len(pmp.PreMsgPool)
}

func (pmp *PrepareMsgPool) GetPreMsgByDigest(nodeName string) consensus.VoteMsg {
	pmp.poolMutex.RLock()
	defer pmp.poolMutex.RUnlock()

	return pmp.PreMsgPool[nodeName]
}

func (pmp *PrepareMsgPool) GetAllPreMsg() []consensus.VoteMsg {
	pmp.poolMutex.RLock()
	defer pmp.poolMutex.RUnlock()

	result := make([]consensus.VoteMsg, 0)

	for _, msg := range pmp.PreMsgPool {
		result = append(result, msg)
		//fmt.Printf("消息池保存的PrepareMsg --- Digest: %s, SequenceID: %d NodeID: %s \n", msg.Digest, msg.SequenceID, msg.NodeID)
	}

	return result
}
