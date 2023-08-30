@echo off
go build .\pbftServer.go

start  "node1"  .\pbftServer --node_name="MainNode" --node_addr="127.0.0.1:9001" --main_node="MainNode" --pbft_cluser="MainNode/127.0.0.1:9001,ReplicaNode1/127.0.0.1:9002,ReplicaNode2/127.0.0.1:9003,ReplicaNode3/127.0.0.1:9004"
start  "node2"  .\pbftServer --node_name="ReplicaNode1" --node_addr="127.0.0.1:9002" --main_node="MainNode" --pbft_cluser="MainNode/127.0.0.1:9001,ReplicaNode1/127.0.0.1:9002,ReplicaNode2/127.0.0.1:9003,ReplicaNode3/127.0.0.1:9004"
start  "node3"  .\pbftServer --node_name="ReplicaNode2" --node_addr="127.0.0.1:9003" --main_node="MainNode" --pbft_cluser="MainNode/127.0.0.1:9001,ReplicaNode1/127.0.0.1:9002,ReplicaNode2/127.0.0.1:9003,ReplicaNode3/127.0.0.1:9004"
start  "node4"  .\pbftServer --node_name="ReplicaNode3" --node_addr="127.0.0.1:9004" --main_node="MainNode" --pbft_cluser="MainNode/127.0.0.1:9001,ReplicaNode1/127.0.0.1:9002,ReplicaNode2/127.0.0.1:9003,ReplicaNode3/127.0.0.1:9004"

