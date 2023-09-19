package p2p

import (
	"blockChain_consensus/tangleChain/common"
	"blockChain_consensus/tangleChain/crypto"
	loglogrus "blockChain_consensus/tangleChain/log_logrus"
	"blockChain_consensus/tangleChain/message"
	"bytes"
	"crypto/ecdsa"
	"encoding/json"
	"net/http"
	"os"
	"reflect"
	"sync"
	"time"

	"github.com/gin-gonic/gin"
)

const (
	defaultMsgExpireTime = 4 * time.Second
	nodeIDBuff           = 128
	msgBuff              = 2048 * 10
)

type InitNodeInfo struct {
	NodeID common.NodeID
	Url    string
}

type Address struct {
	IP   string
	Port int
}

type RemotePeer struct {
	NodeID     common.NodeID
	RemoteAddr Address
}

type Peer struct {
	LocalUrl       string
	OthersUrl      map[string]common.NodeID
	OthersUrlMutex sync.RWMutex

	prvKey *ecdsa.PrivateKey
	pubKey *ecdsa.PublicKey
	nodeID common.NodeID

	MessagePool map[common.Hash]message.MsgInterface
	MsgMutex    sync.RWMutex
}

func NewPeer(url string, otherPeers []string) *Peer {
	peer := &Peer{
		LocalUrl:    url,
		OthersUrl:   make(map[string]common.NodeID),
		MessagePool: make(map[common.Hash]message.MsgInterface),
	}

	if prv, err := crypto.GenerateKey(); err != nil { // 1.生成私钥
		loglogrus.Log.Errorf("[P2P] 当前节点无法无法生成私钥,err:%v\n", err)
		os.Exit(1)
	} else {
		peer.prvKey = prv
		peer.pubKey = prv.Public().(*ecdsa.PublicKey) // 2.生成公钥
		peer.nodeID = crypto.KeytoNodeID(peer.pubKey) // 3.生成NodeID
	}

	peer.InitRouter()

	go peer.Expire()         // 启动过期消息销毁协程
	go peer.HttpInitialize() // 消息处理协程

	return peer
}

func (peer *Peer) BackPrvKey() *ecdsa.PrivateKey {
	return peer.prvKey
}

func (peer *Peer) BackPubKey() *ecdsa.PublicKey {
	return peer.pubKey
}

func (peer *Peer) BackNodeID() common.NodeID {
	return peer.nodeID
}

func (peer *Peer) InitRouter() *gin.Engine {

	r := gin.New()
	r.Use(gin.Logger())
	r.Use(gin.Recovery())
	gin.SetMode("release")

	consensus := r.Group("")

	consensus.POST("/nodeInfo", GetNodeInfo(peer)) // 接收其他节点的标识符(NodeID)
	consensus.POST("/newTx", GetNewTx(peer))       // 接收客户端的req

	return r
}

func (peer *Peer) HttpInitialize() {
	router := peer.InitRouter() //返回一个gin路由器

	s := &http.Server{
		Addr:           peer.LocalUrl,
		Handler:        router,
		ReadTimeout:    5 * time.Second,
		WriteTimeout:   5 * time.Second,
		MaxHeaderBytes: 1 << 20,
	}

	go func() { // 指定时间间隔后向其他节点广播自己的NodeID等信息
		time.Sleep(5 * time.Second)
		// 向其他节点发送自己的的NodeID

		nodeInfo := InitNodeInfo{peer.nodeID, peer.LocalUrl}
		jsonBytes, _ := json.Marshal(nodeInfo)

		peer.Broadcast(jsonBytes, "/nodeInfo")
		loglogrus.Log.Infof("向其余节点广播自己的NodeInfo: NodeID(%x) LocalUrl(%s)\n", peer.nodeID, peer.LocalUrl)
	}()

	// 启动共识节点的Server对象,监听server.url,获取其他共识节点的消息并提供服务
	loglogrus.Log.Infof("ConsensusNode will be started at %v...\n", peer.LocalUrl)
	s.ListenAndServe()
}

func (peer *Peer) Broadcast(bytes []byte, path string) {

	for addr, _ := range peer.OthersUrl { //遍历remoteAddrs, 获取其他共识节点的nodeID和url
		if reflect.DeepEqual(addr, peer.LocalUrl) { // 不需要向自己发送
			continue
		}
		Send(addr+path, bytes) //发送到其他共识节点的相应目录下(不同目录存放不同阶段的共识消息)
	}
}

func Send(url string, msg []byte) {
	buff := bytes.NewBuffer(msg)
	http.Post("http://"+url, "application/json", buff)
}

func GetNodeInfo(peer *Peer) func(*gin.Context) {
	return func(c *gin.Context) {
		var msg InitNodeInfo
		err := json.NewDecoder(c.Request.Body).Decode(&msg)
		if err != nil {
			loglogrus.Log.Errorf("NodeID Msg json解码失败,err:%v", err)
			return
		}
		loglogrus.Log.Infof("当前节点(%s)获取的 NodeInfo Msg: NodeID:(%x) NodeUrl:(%s)\n", peer.LocalUrl, msg.NodeID, msg.Url)

		peer.OthersUrlMutex.Lock()
		peer.OthersUrl[msg.Url] = msg.NodeID
		peer.OthersUrlMutex.Unlock()
	}
}

func GetNewTx(peer *Peer) func(*gin.Context) {
	return func(c *gin.Context) {
		var wrapMsg message.WrapMessage
		err := json.NewDecoder(c.Request.Body).Decode(&wrapMsg)
		if err != nil {
			loglogrus.Log.Errorf("NodeID Msg json解码失败,err:%v", err)
			return
		}
		loglogrus.Log.Infof("当前节点(%s)从节点(%s)处获取的 WrapMessage Msg:  MsgType(%d) \n", peer.LocalUrl, c.Request.RemoteAddr, wrapMsg.MsgType)

		msg := message.DecodeWrapMessage(&wrapMsg)

		// 3.验证消息是否合法(验证数字签名)
		if pass := msg.ValidateSignature(msg.BackHash(), peer.OthersUrl[c.Request.RemoteAddr], msg.BackSignature()); pass {
			loglogrus.Log.Infof("[P2P] 当前节点(%s)对对端(%s)消息的数字签名验证通过,此为合法消息\n", peer.LocalUrl, c.Request.RemoteAddr)
			// 4.将通过验证的消息存放到对应的缓冲区中
			peer.MsgMutex.Lock()
			peer.MessagePool[msg.BackHash()] = msg
			peer.MsgMutex.Unlock()
		} else {
			loglogrus.Log.Warnf("[P2P] 当前节点(%s)对对端(%s)消息的数字签名验证不通过,此为非法消息\n", peer.LocalUrl, c.Request.RemoteAddr)
		}
	}
}

// 删除各个消息池中的过期消息和已读消息
func (peer *Peer) Expire() {
	cycle := time.NewTicker(defaultMsgExpireTime / 2)
	for {
		select {
		case <-cycle.C:
			peer.MsgMutex.Lock()
			for msgHash, msg := range peer.MessagePool {
				if msg.IsRetrieved() == 1 {
					delete(peer.MessagePool, msgHash)
				}
				now := time.Now()
				if uint64(now.UnixNano())-msg.BackTimeStamp() > uint64(defaultMsgExpireTime.Nanoseconds()) {
					delete(peer.MessagePool, msgHash)
				}
			}
			peer.MsgMutex.Unlock()
		}
	}
}

func (peer *Peer) BackAllMsg() map[common.Hash]message.MsgInterface {
	msgPool := make(map[common.Hash]message.MsgInterface)

	peer.MsgMutex.RLock()
	for hash, msg := range peer.MessagePool {
		msgPool[hash] = msg
	}
	peer.MsgMutex.RUnlock()

	return msgPool
}
