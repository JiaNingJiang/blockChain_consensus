package myraft

import (
	"net"
	"path/filepath"
	"strings"
	"time"

	"raftClient/fsm"
	loglogrus "raftClient/log_logrus"

	"github.com/hashicorp/raft"
	raftboltdb "github.com/hashicorp/raft-boltdb"
)

func NewMyRaft(raftAddr, raftId, raftDir string) (*raft.Raft, *fsm.Fsm, error) {
	config := raft.DefaultConfig()
	config.LogOutput = loglogrus.Log.Out
	config.LogLevel = "DEBUG"
	config.LocalID = raft.ServerID(raftId)
	// config.HeartbeatTimeout = 1000 * time.Millisecond
	// config.ElectionTimeout = 1000 * time.Millisecond
	// config.CommitTimeout = 1000 * time.Millisecond

	addr, err := net.ResolveTCPAddr("tcp", raftAddr)
	if err != nil {
		return nil, nil, err
	}
	transport, err := raft.NewTCPTransport(raftAddr, addr, 2, 5*time.Second, loglogrus.Log.Out) // 节点之间采用流式TCP。maxPool是指其余raft节点的个数,timeout应该是tcp超时时间,logOutput指定日志输出目标
	if err != nil {
		return nil, nil, err
	}
	snapshots, err := raft.NewFileSnapshotStore(raftDir, 2, loglogrus.Log.Out) // retain指定当前节点会维持的快照的数目(至少为1)
	if err != nil {
		return nil, nil, err
	}
	logStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft-log.db")) // 底层存储引起使用BoltDB,此处指定日志存储文件名称
	if err != nil {
		return nil, nil, err
	}
	stableStore, err := raftboltdb.NewBoltStore(filepath.Join(raftDir, "raft-stable.db")) // 此处指定raft稳定记录存储文件名称(已经commit的记录)
	if err != nil {
		return nil, nil, err
	}
	fm := fsm.NewFsm()                                                               // 创建FSM状态机
	rf, err := raft.NewRaft(config, fm, logStore, stableStore, snapshots, transport) // 创建raft节点
	if err != nil {
		return nil, nil, err
	}

	return rf, fm, nil
}

// 与其他的raft节点完成连接
func Bootstrap(rf *raft.Raft, raftId, raftAddr, raftCluster string) {
	servers := rf.GetConfiguration().Configuration().Servers // 检查当前节点是否已经与其他raft节点存在连接了
	if len(servers) > 0 {
		return
	}
	peerArray := strings.Split(raftCluster, ",")
	if len(peerArray) == 0 {
		return
	}

	var configuration raft.Configuration
	for _, peerInfo := range peerArray { // 提取出其他raft节点的id和raft通信地址
		peer := strings.Split(peerInfo, "/")
		id := peer[0]
		addr := peer[1]
		server := raft.Server{
			ID:      raft.ServerID(id),
			Address: raft.ServerAddress(addr),
		}
		configuration.Servers = append(configuration.Servers, server)
	}
	rf.BootstrapCluster(configuration) // 完成与其余raft节点的连接
	return
}
