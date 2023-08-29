package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io/ioutil"
	"math/rand"
	"mime/multipart"
	"net/http"
	"time"
)

// 测试客户端,发送一堆交易给raft节点集群,获取TPS

type Client struct {
	NodeID string // 客户节点的名称标识符

	NodeURL []string // 所有节点的http url
}

type RaftHttpResponse struct {
	Code int    `json:"code"`
	Data string `json:"data"`
	Msg  string `json:"msg"`
}

func NewClient(nodeID string, nodeURL []string) *Client {
	return &Client{
		NodeID:  nodeID,
		NodeURL: nodeURL,
	}
}

func (c *Client) GetLeaderHttp() string {

	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(c.NodeURL))

	resMsg := sendHttpGet(c.NodeURL[index] + "/leader")

	raftRes := &RaftHttpResponse{}

	json.Unmarshal([]byte(resMsg), raftRes)

	return raftRes.Data
}

func (c *Client) CommonWrite(args []string) string {
	// 创建一个缓冲区，用于存储multipart/form-data请求的内容
	var buf bytes.Buffer
	// 创建一个multipart.Writer实例，用于构建multipart/form-data请求的body
	writer := multipart.NewWriter(&buf)
	// 添加form-data参数
	writer.WriteField("contractName", "Common")
	writer.WriteField("functionName", "Write")

	argStr := ""
	for i := 0; i < len(args); i++ {
		argStr += args[i]
		if i != len(args)-1 {
			argStr += " "
		}
	}
	writer.WriteField("args", argStr)

	writer.Close()

	leaderHttp := c.GetLeaderHttp()

	// return sendHttpPost(leaderHttp+"/newTx", buf, writer)
	return sendHttpPost(leaderHttp+"/newTx_highTPS", buf, writer) // 对于写操作,当前接口大约要比上面的/newTx接口的TPS高一倍
}

func (c *Client) CommonRead(args []string) string {
	// 创建一个缓冲区，用于存储multipart/form-data请求的内容
	var buf bytes.Buffer
	// 创建一个multipart.Writer实例，用于构建multipart/form-data请求的body
	writer := multipart.NewWriter(&buf)
	// 添加form-data参数
	writer.WriteField("contractName", "Common")
	writer.WriteField("functionName", "Read")

	argStr := ""
	for i := 0; i < len(args); i++ {
		argStr += args[i]
		if i != len(args)-1 {
			argStr += " "
		}
	}
	writer.WriteField("args", argStr)

	writer.Close()

	// 设计读取的交易随机找一个raft节点即可
	rand.Seed(time.Now().UnixNano())
	index := rand.Intn(len(c.NodeURL))
	return sendHttpPost(c.NodeURL[index]+"/newTx", buf, writer)

}

func sendHttpPost(dperurl string, buf bytes.Buffer, writer *multipart.Writer) string {
	// 创建一个http请求
	req, err := http.NewRequest("POST", dperurl, &buf)
	if err != nil {
		panic(err)
	}
	// 设置请求头Content-Type为multipart/form-data
	if writer != nil {
		req.Header.Set("Content-Type", writer.FormDataContentType())
	}

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	// 返回响应内容
	return string(body)
}

func sendHttpGet(dperurl string) string {
	var buf bytes.Buffer
	// 创建一个http请求
	req, err := http.NewRequest("GET", dperurl, &buf)
	if err != nil {
		panic(err)
	}

	// 发送请求
	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		panic(err)
	}
	defer resp.Body.Close()

	// 读取响应内容
	body, err := ioutil.ReadAll(resp.Body)
	if err != nil {
		fmt.Println(err)
		return ""
	}

	// 打印响应内容
	//fmt.Printf("now: %s ---- %s\n", time.Now().Format("2006-01-02 15:04:05"), string(body))
	return string(body)
}
