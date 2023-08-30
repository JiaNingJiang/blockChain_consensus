@echo off
start  "node1"  .\tanglePeer --peerIP="127.0.0.1" --peerPort=8001 --peersTable="127.0.0.1:8001,127.0.0.1:8002,127.0.0.1:8003" --sendRate=10 --txCD=2
start  "node2"  .\tanglePeer --peerIP="127.0.0.1" --peerPort=8002 --peersTable="127.0.0.1:8001,127.0.0.1:8002,127.0.0.1:8003" --sendRate=10 --txCD=2
start  "node3"  .\tanglePeer --peerIP="127.0.0.1" --peerPort=8003 --peersTable="127.0.0.1:8001,127.0.0.1:8002,127.0.0.1:8003" --sendRate=10 --txCD=2
