package network

import (
	"pbft_blockchain/client"
	"testing"
	"time"
)

func TestFourNode(t *testing.T) {
	mainNode := NewServer("MainNode")

	ReplicaNode1 := NewServer("ReplicaNode1")
	_ = ReplicaNode1

	ReplicaNode2 := NewServer("ReplicaNode2")
	_ = ReplicaNode2

	ReplicaNode3 := NewServer("ReplicaNode3")
	_ = ReplicaNode3

	time.Sleep(5 * time.Second) // 等待节点们完成NodeID等信息的交换

	client := client.NewClient("client1", mainNode.url)

	args := make([][]byte, 2)
	args[0] = []byte("key")
	args[1] = []byte("value")
	client.CommonWrite(args)

	time.Sleep(time.Second)

	arg := make([][]byte, 1)
	arg[0] = []byte("key")

	for i := 0; i < 200; i++ {
		time.Sleep(time.Nanosecond)
		client.CommonRead(arg)
	}

	for {
		time.Sleep(1 * time.Second)
	}

}
