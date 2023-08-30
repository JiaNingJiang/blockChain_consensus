package client

import (
	loglogrus "blockChain_consensus/pbftChain/log_logrus"
	"blockChain_consensus/pbftChain/pbft/consensus"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"
)

type Client struct {
	NodeID     string // 客户节点的名称标识符
	NodeUrl    string // 客户端节点的url(用来接收PBFT主节点的reply Msg)
	PrimaryURL string // PBFT主节点的url

	successfulTx      int // 成功执行的交易数量
	successfulTxMutex sync.RWMutex
}

func NewClient(nodeID string, nodeUrl string, primaryURL string) *Client {

	c := &Client{
		NodeID:       nodeID,
		NodeUrl:      nodeUrl,
		PrimaryURL:   primaryURL,
		successfulTx: 0,
	}

	http.HandleFunc("/result", func(w http.ResponseWriter, r *http.Request) {
		var result consensus.ExcuteResult
		err := json.NewDecoder(r.Body).Decode(&result)
		if err != nil {
			loglogrus.Log.Errorf("[Client] ExcuteResult Msg json解码失败,err:%v", err)
			return
		} else {
			if result.Error == "nil" {
				c.successfulTxMutex.Lock()
				c.successfulTx += 1
				c.successfulTxMutex.Unlock()
			}
			resultStr, _ := json.MarshalIndent(result, "", "")
			loglogrus.Log.Infof("[Client] 执行结果: %s\n", string(resultStr))
		}
	})

	// 启动HTTP服务器并监听端口
	go http.ListenAndServe(nodeUrl, nil)

	return c

}

func (c *Client) BakcSuccessfulTxCount() int {
	c.successfulTxMutex.RLock()
	defer c.successfulTxMutex.RUnlock()

	return c.successfulTx
}

func (c *Client) CommonWrite(args [][]byte) {
	timer := time.Now()

	request := consensus.RequestMsg{
		Timestamp: uint64(timer.UnixNano()),
		ClientID:  c.NodeID,
		ClientUrl: c.NodeUrl,
		Operation: "Common::Write",
		Args:      args,
	}

	encodeByte, err := json.Marshal(request)
	if err != nil {
		fmt.Println("RequestMsg json encode is failed")
	}

	send(c.PrimaryURL+"/req", encodeByte)
	loglogrus.Log.Infof("[Client] 客户端发送请求: Common::Write args0:%s  args1:%s\n", args[0], args[1])
}

func (c *Client) CommonRead(args [][]byte) {
	timer := time.Now()

	request := consensus.RequestMsg{
		Timestamp: uint64(timer.UnixNano()),
		ClientID:  c.NodeID,
		ClientUrl: c.NodeUrl,
		Operation: "Common::Read",
		Args:      args,
	}

	encodeByte, err := json.Marshal(request)
	if err != nil {
		fmt.Println("RequestMsg json encode is failed")
	}

	send(c.PrimaryURL+"/req", encodeByte)
	loglogrus.Log.Infof("[Client] 客户端发送请求: Common::Read args:%s\n", args[0])

}

func send(url string, msg []byte) {
	buff := bytes.NewBuffer(msg)
	http.Post("http://"+url, "application/json", buff)
}
