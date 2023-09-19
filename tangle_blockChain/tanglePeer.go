package main

import (
	loglogrus "blockChain_consensus/tangleChain/log_logrus"
	"blockChain_consensus/tangleChain/p2p"
	"blockChain_consensus/tangleChain/tangle"
	"context"
	"flag"
	"fmt"
	"os"
	"time"
)

var (
	peerUrl    string
	peersTable string

	sendRate          int // 节点发送交易的速率
	txConfirmDuration int // 交易从发布到上链所需的确定时延

	powDiff uint64 // pow难度
)

func init() {
	flag.StringVar(&peerUrl, "peerUrl", "127.0.0.1:65588", "Tangle节点的Url监听地址")
	flag.StringVar(&peersTable, "peersTable", "127.0.0.1:8001,127.0.0.1:8002,127.0.0.1:8003", "Tangle集群中所有节点的Url地址")

	flag.IntVar(&sendRate, "sendRate", 10, "节点发送交易的速率")
	flag.IntVar(&txConfirmDuration, "txCD", 4, "交易从发布到上链所需的确定时延")

	flag.Uint64Var(&powDiff, "powDiff", 3, "节点生成交易时的pow难度")
}

func main() {
	flag.Parse()

	otherPeers := make([]string, 0)

	peer := p2p.NewPeer(peerUrl, otherPeers) // 建立本地p2p节点

	tanglePeer := tangle.NewTangle(sendRate, time.Duration(txConfirmDuration)*time.Second, powDiff, peer) // 在p2p节点之上创建tangle节点

	ctx, finFunc := context.WithCancel(context.Background())

	tanglePeer.Start(ctx) // 启动tangle节点

	txSendCycle := time.NewTicker(1 * time.Second)
	finTimer := time.NewTimer(20 * time.Second)

	startTime := time.Now()
	for {
		select {
		case <-txSendCycle.C:
			for i := 0; i < sendRate; i++ {
				tanglePeer.PublishTransaction(tangle.CommonWriteAndReadCode, []string{fmt.Sprintf("test_key%d", i), fmt.Sprintf("test_value%d", i)})
			}
		case <-finTimer.C:
			txSendCycle.Stop()
			finFunc()

			txCount := tanglePeer.BackTxCount()
			endTime := time.Now()
			loglogrus.Log.Infof(" 当前节点(%s) -- 起始时间(%s) -- 终止时间(%s) -- 时间差(%v) -- 上链交易数为:%d\n", peerUrl,
				startTime.Format("2006-01-02 15:04:05"), endTime.Format("2006-01-02 15:04:05"), endTime.Sub(startTime).Seconds(), txCount)

			time.Sleep(5 * time.Second)
			os.Exit(0)
		}
	}

}
