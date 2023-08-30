package tangle

import (
	"blockChain_consensus/tangleChain/p2p"
	"context"
	"fmt"
	"testing"
	"time"
)

var (
	peer1 = &p2p.Peer{}
	peer2 = &p2p.Peer{}
	peer3 = &p2p.Peer{}
)

func p2pNet() {
	mockAddr1 := &p2p.Address{"127.0.0.1", 8001}
	mockAddr2 := &p2p.Address{"127.0.0.1", 8002}
	mockAddr3 := &p2p.Address{"127.0.0.1", 8003}

	peer1 = p2p.NewPeer(mockAddr1.IP, mockAddr1.Port)
	peer2 = p2p.NewPeer(mockAddr2.IP, mockAddr2.Port)
	peer3 = p2p.NewPeer(mockAddr3.IP, mockAddr3.Port)

	time.Sleep(1 * time.Second)

	peer1.LookUpOthers([]*p2p.Address{mockAddr2, mockAddr3})
	peer2.LookUpOthers([]*p2p.Address{mockAddr1, mockAddr3})
	peer3.LookUpOthers([]*p2p.Address{mockAddr1, mockAddr2})

	time.Sleep(1 * time.Second)
}

func TestTangle(t *testing.T) {
	p2pNet()

	tangle1 := NewTangle(10, 4*time.Second, peer1)
	tangle2 := NewTangle(10, 4*time.Second, peer2)
	tangle3 := NewTangle(10, 4*time.Second, peer3)

	ctx := context.Background()

	go tangle1.ReadMsgFromP2PPool(ctx)
	go tangle2.ReadMsgFromP2PPool(ctx)
	go tangle3.ReadMsgFromP2PPool(ctx)

	go tangle1.UpdateTipSet(ctx)
	go tangle2.UpdateTipSet(ctx)
	go tangle3.UpdateTipSet(ctx)

	// 测试一: 仅让tangle1(peer1)发布一笔交易
	go func() {
		for i := 0; i < 9; i++ {
			tangle1.PublishTransaction(fmt.Sprintf("tx%d", i), CommonWriteAndReadCode)
		}
	}()

	// tangle1.PublishTransaction("tx1")
	time.Sleep(10 * time.Second)

	for tipID, _ := range tangle3.TipSet {
		fmt.Printf("--------------tip TxID : %x----------------\n", tipID)
	}

	for candidateID, candidate := range tangle3.CandidateTips {
		for _, approveTx := range candidate.ApproveTx {
			fmt.Printf("--------------------candidate TxID : %x  ,  approve TxID: %x----------------------\n", candidateID, approveTx)
		}

	}

	fmt.Println()

	// 测试二: 让tangle1(peer1),tangle2(peer2)各自发布一笔交易
	tangle1.PublishTransaction("tx2", CommonWriteAndReadCode)
	tangle2.PublishTransaction("tx2", CommonWriteAndReadCode)
	_ = tangle3
	time.Sleep(15 * time.Second)

	for tipID, _ := range tangle3.TipSet {
		fmt.Printf("--------------tip TxID : %x----------------\n", tipID)
	}

	for candidateID, candidate := range tangle3.CandidateTips {
		for _, approveTx := range candidate.ApproveTx {
			fmt.Printf("--------------------candidate TxID : %x  ,  approve TxID: %x----------------------\n", candidateID, approveTx)
		}
	}

	time.Sleep(10 * time.Second)

	// 测试三: 让tangle1(peer1),tangle2(peer2)各自再发布一笔交易
	tangle1.PublishTransaction("tx3", CommonWriteAndReadCode)
	tangle2.PublishTransaction("tx3", CommonWriteAndReadCode)
	_ = tangle3
	time.Sleep(10 * time.Second)

	for tipID, _ := range tangle3.TipSet {
		fmt.Printf("--------------tip TxID : %x----------------\n", tipID)
	}

	for candidateID, candidate := range tangle3.CandidateTips {
		for _, approveTx := range candidate.ApproveTx {
			fmt.Printf("--------------------candidate TxID : %x  ,  approve TxID: %x----------------------\n", candidateID, approveTx)
		}
	}

}
