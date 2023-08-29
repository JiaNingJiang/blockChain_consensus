package main

import (
	"flag"
	"net/http"
	"os"
	"strings"
	"sync/atomic"
	"time"

	api "raftClient/api/httpApi"
	loglogrus "raftClient/log_logrus"
	"raftClient/myraft"
)

var (
	httpAddr           string
	raftAddr           string
	http_raft_addr_map string
	raftId             string
	raftCluster        string
	raftDir            string
)

var (
	isLeader int64
)

var (
	addrMap = make(map[string]string) // 映射raft节点的http_addr和raft_addr
)

func init() {
	flag.StringVar(&httpAddr, "http_addr", "127.0.0.1:7001", "http listen addr")                                       // 设置当前节点的http通信端口(与客户端)
	flag.StringVar(&raftAddr, "raft_addr", "127.0.0.1:7000", "raft listen addr")                                       // 设置当前节点的raft节点间通信端口
	flag.StringVar(&raftId, "raft_id", "1", "raft id")                                                                 // 设置当前节点的raft节点编号
	flag.StringVar(&raftCluster, "raft_cluster", "1/127.0.0.1:7000,2/127.0.0.1:8000,3/127.0.0.1:9000", "cluster info") // 设置其余节点的 编号+raft通信端口
	flag.StringVar(&http_raft_addr_map, "http_raft_addr_map", "127.0.0.1:7001/127.0.0.1:7000,127.0.0.1:8001/127.0.0.1:8000,127.0.0.1:9001/127.0.0.1:9000", "httpAddr map raftAddr")

	peerMaps := strings.Split(http_raft_addr_map, ",")
	if len(peerMaps) == 0 {
		return
	}
	for _, peerMap := range peerMaps { // 完成所有节点http地址和raft地址的映射
		peer := strings.Split(peerMap, "/")
		httpAddr := peer[0]
		raftAddr := peer[1]

		addrMap[raftAddr] = httpAddr
	}

}

func main() {
	flag.Parse()
	// 初始化配置
	if httpAddr == "" || raftAddr == "" || raftId == "" || raftCluster == "" {
		loglogrus.Log.Errorf("[Initialization] Node Config error: httpAddr == nil or raftAddr == nil or raftId == nil or raftCluster == nil\n")
		os.Exit(1)
		return
	}
	raftDir := "node/raft_" + raftId // 指定每一个raft节点的持久化存储目录(存储当前节点产生的log数据以及状态快照)
	os.MkdirAll(raftDir, 0700)

	// 初始化raft
	myRaft, fm, err := myraft.NewMyRaft(raftAddr, raftId, raftDir)
	if err != nil {
		loglogrus.Log.Errorf("[Initialization] Node Raft Initialize failed: err:%v\n", err)
		os.Exit(1)
		return
	}

	// 启动raft(当前节点完成与其他raft节点的连接)
	myraft.Bootstrap(myRaft, raftId, raftAddr, raftCluster)

	// 监听leader变化（使用此方法无法保证强一致性读，仅做leader变化过程观察）
	go func() {
		for leader := range myRaft.LeaderCh() { // 返回当前是否有leader节点(有就会返回true,没有就会返回false)
			if leader {
				atomic.StoreInt64(&isLeader, 1)
			} else {
				atomic.StoreInt64(&isLeader, 0)
			}
		}
	}()

	// 启动http server

	httpServer := api.InitRouter(myRaft, fm, addrMap)

	s := &http.Server{
		Addr:           httpAddr,
		Handler:        httpServer,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	s.ListenAndServe()

	// 关闭raft
	// shutdownFuture := myRaft.Shutdown()
	// if err := shutdownFuture.Error(); err != nil {
	// 	fmt.Printf("shutdown raft error:%v \n", err)
	// }

	// 退出http server
	// fmt.Println("shutdown kv http server")
}
