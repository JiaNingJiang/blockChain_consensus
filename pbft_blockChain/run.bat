@echo off

if exist .\pbftNode.exe (  
    del .\pbftNode.exe
    go build -o pbftNode.exe .\main.go
) else (
    go build -o pbftNode.exe .\main.go
)


if exist .\client.exe (  
    del .\client.exe
    go build  .\client.go
) else (
    go build  .\client.go
)


start  pbftNode.exe -id="MainNode" -log=1
start  pbftNode.exe -id="ReplicaNode1" -log=2
start  pbftNode.exe -id="ReplicaNode2" -log=3
start  pbftNode.exe -id="ReplicaNode3" -log=4

timeout /t 3

start client.exe