package p2p

import (
	"blockChain_consensus/tangleChain/message"
	"fmt"
	"testing"
	"time"
)

func TestP2P(t *testing.T) {
	mockAddr1 := &Address{"127.0.0.1", 8001}
	mockAddr2 := &Address{"127.0.0.1", 8002}
	mockAddr3 := &Address{"127.0.0.1", 8003}

	peer1 := NewPeer(mockAddr1.IP, mockAddr1.Port)
	peer2 := NewPeer(mockAddr2.IP, mockAddr2.Port)
	peer3 := NewPeer(mockAddr3.IP, mockAddr3.Port)

	time.Sleep(1 * time.Second)

	peer1.LookUpOthers([]*Address{mockAddr2, mockAddr3})
	peer2.LookUpOthers([]*Address{mockAddr1, mockAddr3})
	peer3.LookUpOthers([]*Address{mockAddr1, mockAddr2})

	time.Sleep(1 * time.Second)

	cMsg := message.NewCommonMsg([]byte("hello"), peer1.nodeID, peer1.prvKey)
	wrapMsg := message.EncodeToWrapMessage(cMsg)

	peer1.Broadcast(wrapMsg)

	time.Sleep(1 * time.Second)

	for hash, msg := range peer2.MessagePool {
		fmt.Printf("节点2消息池中消息 (hash:%x) (内容:%s)\n", hash, msg.BackPayload())
		msg.MarkRetrieved()
	}

	for hash, msg := range peer3.MessagePool {
		fmt.Printf("节点3消息池中消息 (hash:%x) (内容:%s)\n", hash, msg.BackPayload())
		msg.MarkRetrieved()
	}

	time.Sleep(6 * time.Second)

	// 检查已读标记是否起效
	for hash, msg := range peer2.MessagePool {
		fmt.Printf("节点2消息池中消息 (hash:%x) (内容:%s)\n", hash, msg.BackPayload())
	}

	for hash, msg := range peer3.MessagePool {
		fmt.Printf("节点3消息池中消息 (hash:%x) (内容:%s)\n", hash, msg.BackPayload())
	}
}
