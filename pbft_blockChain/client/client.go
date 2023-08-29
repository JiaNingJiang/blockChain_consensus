package client

import (
	loglogrus "blockChain_consensus/pbftChain/log_logrus"
	"blockChain_consensus/pbftChain/pbft/consensus"
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

type Client struct {
	NodeID string // 客户节点的名称标识符

	PrimaryURL string // PBFT主节点的url
}

func NewClient(nodeID string, primaryURL string) *Client {
	return &Client{
		NodeID:     nodeID,
		PrimaryURL: primaryURL,
	}
}

func (c *Client) CommonWrite(args [][]byte) {
	timer := time.Now()

	request := consensus.RequestMsg{
		Timestamp: uint64(timer.UnixNano()),
		ClientID:  c.NodeID,
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
