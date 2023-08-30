package main

import (
	"blockChain_consensus/pbftChain/pbft/network"
	"flag"
	"strings"
)

var (
	node_name   string
	node_addr   string
	main_node   string
	pbft_cluser string
)

func init() {
	flag.StringVar(&node_name, "node_name", "MainNode", "pbft节点标识符")
	flag.StringVar(&node_addr, "node_addr", "127.0.0.1:9001", "pbft节点的通信地址")
	flag.StringVar(&main_node, "main_node", "MainNode", "pbft集群中初始主节点的标识名")
	flag.StringVar(&pbft_cluser, "pbft_cluser", "MainNode/127.0.0.1:9001,ReplicaNode1/127.0.0.1:9002,ReplicaNode2/127.0.0.1:9003,ReplicaNode3/127.0.0.1:9004", "pbft集群所有节点的信息")
}

func main() {
	flag.Parse()

	nodeNameTable := make(map[string]string)

	peerMaps := strings.Split(pbft_cluser, ",")
	if len(peerMaps) == 0 {
		return
	}
	for _, peerMap := range peerMaps { // 完成所有节点http地址和raft地址的映射
		peer := strings.Split(peerMap, "/")
		nodeName := peer[0]
		nodeAddr := peer[1]

		nodeNameTable[nodeName] = nodeAddr
	}

	curNode := network.NewServer(node_name, node_addr, main_node, nodeNameTable)

	curNode.HttpInitialize()
}
