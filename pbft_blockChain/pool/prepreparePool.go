package pool

import (
	"blockChain_consensus/pbftChain/common"
	"blockChain_consensus/pbftChain/pbft/consensus"
	"sync"
)

type PrePrepareMsgPool struct {
	PPMsgPool map[common.Hash]consensus.PrePrepareMsg //包含的交易的txID作为key

	poolMutex sync.RWMutex
}

func NewPPMsgPool() *PrePrepareMsgPool {
	return &PrePrepareMsgPool{
		PPMsgPool: make(map[common.Hash]consensus.PrePrepareMsg),
	}
}

func (ppmp *PrePrepareMsgPool) AddPPMsg(ppMsg consensus.PrePrepareMsg) {
	ppmp.poolMutex.Lock()
	defer ppmp.poolMutex.Unlock()

	ppmp.PPMsgPool[ppMsg.Block.BlockID] = ppMsg
}

func (ppmp *PrePrepareMsgPool) DelPPMsg(txID common.Hash) {
	ppmp.poolMutex.Lock()
	defer ppmp.poolMutex.Unlock()

	if _, ok := ppmp.PPMsgPool[txID]; ok {
		delete(ppmp.PPMsgPool, txID)
	}
}

func (ppmp *PrePrepareMsgPool) MsgNum() int {
	ppmp.poolMutex.RLock()
	defer ppmp.poolMutex.RUnlock()

	return len(ppmp.PPMsgPool)
}

func (ppmp *PrePrepareMsgPool) GetPPMsgByDigest(txID common.Hash) consensus.PrePrepareMsg {
	ppmp.poolMutex.RLock()
	defer ppmp.poolMutex.RUnlock()

	return ppmp.PPMsgPool[txID]
}

func (ppmp *PrePrepareMsgPool) GetAllPPMsg() []consensus.PrePrepareMsg {
	ppmp.poolMutex.RLock()
	defer ppmp.poolMutex.RUnlock()

	result := make([]consensus.PrePrepareMsg, 0)
	for _, msg := range ppmp.PPMsgPool {
		result = append(result, msg)
	}
	return result
}
