@echo off
go build .\tanglePeer.go

start  "node1"  .\tanglePeer --peerUrl="127.0.0.1:8001" --peersTable="127.0.0.1:8001,127.0.0.1:8002,127.0.0.1:8003" --sendRate=100 --txCD=1
start  "node2"  .\tanglePeer --peerUrl="127.0.0.1:8002" --peersTable="127.0.0.1:8001,127.0.0.1:8002,127.0.0.1:8003" --sendRate=100 --txCD=1
start  "node3"  .\tanglePeer --peerUrl="127.0.0.1:8003" --peersTable="127.0.0.1:8001,127.0.0.1:8002,127.0.0.1:8003" --sendRate=100 --txCD=1
