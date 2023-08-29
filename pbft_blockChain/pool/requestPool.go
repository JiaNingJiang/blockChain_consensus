package pool

import (
	"blockChain_consensus/pbftChain/common"
	"blockChain_consensus/pbftChain/pbft/consensus"
	"fmt"
	"sync"
)

type RequestMsgPool struct {
	ReqMsgPool map[common.Hash]consensus.RequestMsg // request请求的哈希值作为key

	poolMutex sync.RWMutex
}

func NewReqMsgPool() *RequestMsgPool {
	return &RequestMsgPool{
		ReqMsgPool: make(map[common.Hash]consensus.RequestMsg),
	}
}

func (rmp *RequestMsgPool) AddReqMsg(reqMsg consensus.RequestMsg) {
	rmp.poolMutex.Lock()
	defer rmp.poolMutex.Unlock()

	rmp.ReqMsgPool[reqMsg.Hash()] = reqMsg

}

func (rmp *RequestMsgPool) DelReqMsg(hash common.Hash) {
	rmp.poolMutex.Lock()
	defer rmp.poolMutex.Unlock()

	if _, ok := rmp.ReqMsgPool[hash]; ok {
		delete(rmp.ReqMsgPool, hash)
	}
}

func (rmp *RequestMsgPool) GetReqMsgByClientID(hash common.Hash) consensus.RequestMsg {
	rmp.poolMutex.RLock()
	defer rmp.poolMutex.RUnlock()

	if result, ok := rmp.ReqMsgPool[hash]; ok {
		return result
	} else {
		fmt.Println("该Msg不存在。。。。。。。。。。。")
	}
	return consensus.RequestMsg{}

}

func (rmp *RequestMsgPool) GetAllReqMsg() []consensus.RequestMsg {
	rmp.poolMutex.RLock()
	defer rmp.poolMutex.RUnlock()

	result := make([]consensus.RequestMsg, 0)
	for _, msg := range rmp.ReqMsgPool {
		result = append(result, msg)
	}
	return result
}

func (rmp *RequestMsgPool) MsgNum() int {
	rmp.poolMutex.RLock()
	defer rmp.poolMutex.RUnlock()

	return len(rmp.ReqMsgPool)
}
