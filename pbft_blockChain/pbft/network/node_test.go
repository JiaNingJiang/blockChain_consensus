package network

import (
	"blockChain_consensus/pbftChain/client"
	"testing"
	"time"
)

func TestFourNode(t *testing.T) {
	mainNode := NewServer("MainNode", "127.0.0.1:9001", "MainNode", map[string]string{
		"MainNode":     "127.0.0.1:9001",
		"ReplicaNode1": "127.0.0.1:9002",
		"ReplicaNode2": "127.0.0.1:9003",
		"ReplicaNode3": "127.0.0.1:9004",
	})
	go mainNode.HttpInitialize()

	ReplicaNode1 := NewServer("ReplicaNode1", "127.0.0.1:9002", "MainNode", map[string]string{
		"MainNode":     "127.0.0.1:9001",
		"ReplicaNode1": "127.0.0.1:9002",
		"ReplicaNode2": "127.0.0.1:9003",
		"ReplicaNode3": "127.0.0.1:9004",
	})
	go ReplicaNode1.HttpInitialize()
	_ = ReplicaNode1

	ReplicaNode2 := NewServer("ReplicaNode2", "127.0.0.1:9003", "MainNode", map[string]string{
		"MainNode":     "127.0.0.1:9001",
		"ReplicaNode1": "127.0.0.1:9002",
		"ReplicaNode2": "127.0.0.1:9003",
		"ReplicaNode3": "127.0.0.1:9004",
	})
	go ReplicaNode2.HttpInitialize()
	_ = ReplicaNode2

	ReplicaNode3 := NewServer("ReplicaNode3", "127.0.0.1:9004", "MainNode", map[string]string{
		"MainNode":     "127.0.0.1:9001",
		"ReplicaNode1": "127.0.0.1:9002",
		"ReplicaNode2": "127.0.0.1:9003",
		"ReplicaNode3": "127.0.0.1:9004",
	})
	go ReplicaNode3.HttpInitialize()
	_ = ReplicaNode3

	time.Sleep(5 * time.Second) // 等待节点们完成NodeID等信息的交换

	client := client.NewClient("client1", "127.0.0.1:8001", "127.0.0.1:9001")

	args := make([][]byte, 2)
	args[0] = []byte("key")
	args[1] = []byte("value")
	client.CommonWrite(args)

	time.Sleep(time.Second)

	arg := make([][]byte, 1)
	arg[0] = []byte("key")

	client.CommonRead(arg)
	// for i := 0; i < 200; i++ {
	// 	time.Sleep(time.Nanosecond)
	// 	client.CommonRead(arg)
	// }

	for {
		time.Sleep(1 * time.Second)
	}

}
