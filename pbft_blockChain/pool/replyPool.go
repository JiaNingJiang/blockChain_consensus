package pool

import (
	"pbft_blockchain/common"
	"pbft_blockchain/pbft/consensus"
	"sync"
)

type ReplyMsgPool struct {
	RyMsgPool map[common.NodeID]consensus.ReplyMsg //NodeID做key值

	poolMutex sync.RWMutex
}

func NewRyMsgPool() *ReplyMsgPool {
	return &ReplyMsgPool{
		RyMsgPool: make(map[common.NodeID]consensus.ReplyMsg),
	}
}

func (ry *ReplyMsgPool) AddRyMsg(ryMsg consensus.ReplyMsg) {
	ry.poolMutex.Lock()
	defer ry.poolMutex.Unlock()

	ry.RyMsgPool[ryMsg.NodeID] = ryMsg

}

func (ry *ReplyMsgPool) DelRyMsg(nodeID common.NodeID) {
	ry.poolMutex.Lock()
	defer ry.poolMutex.Unlock()

	if _, ok := ry.RyMsgPool[nodeID]; ok {
		delete(ry.RyMsgPool, nodeID)
	}
}
func (ry *ReplyMsgPool) DelAllRyMsg() {
	ry.poolMutex.Lock()
	defer ry.poolMutex.Unlock()

	ry.RyMsgPool = make(map[common.NodeID]consensus.ReplyMsg)
}

func (ry *ReplyMsgPool) GetRyMsgByClientID(nodeID common.NodeID) consensus.ReplyMsg {
	ry.poolMutex.RLock()
	defer ry.poolMutex.RUnlock()

	return ry.RyMsgPool[nodeID]

}

func (ry *ReplyMsgPool) GetAllRyMsg() []consensus.ReplyMsg {
	ry.poolMutex.RLock()
	defer ry.poolMutex.RUnlock()

	result := make([]consensus.ReplyMsg, 0)
	for _, msg := range ry.RyMsgPool {
		result = append(result, msg)
	}
	return result
}

func (ry *ReplyMsgPool) MsgNum() int {
	ry.poolMutex.RLock()
	defer ry.poolMutex.RUnlock()

	return len(ry.RyMsgPool)
}
