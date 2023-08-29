package p2p

import (
	"blockChain_consensus/tangleChain/common"
	"blockChain_consensus/tangleChain/crypto"
	loglogrus "blockChain_consensus/tangleChain/log_logrus"
	"blockChain_consensus/tangleChain/message"
	"blockChain_consensus/tangleChain/rlp"
	"crypto/ecdsa"
	"fmt"
	"net"
	"os"
	"reflect"
	"sync"
	"time"
)

const (
	defaultMsgExpireTime = 4 * time.Second
	nodeIDBuff           = 128
	msgBuff              = 2048 * 10
)

type Address struct {
	IP   string
	Port int
}

type RemotePeer struct {
	NodeID     common.NodeID
	RemoteAddr Address
	Conn       net.Conn
}

type Peer struct {
	LocalAddr    Address
	ListenSocket net.Listener
	Others       map[common.NodeID]*RemotePeer

	prvKey *ecdsa.PrivateKey
	pubKey *ecdsa.PublicKey
	nodeID common.NodeID

	MessagePool map[common.Hash]message.MsgInterface
	MsgMutex    sync.RWMutex
}

func NewPeer(IP string, Port int) *Peer {
	peer := &Peer{
		LocalAddr:   Address{IP: IP, Port: Port},
		Others:      make(map[common.NodeID]*RemotePeer),
		MessagePool: make(map[common.Hash]message.MsgInterface),
	}

	listener, err := net.Listen("tcp", IP+fmt.Sprintf(":%d", Port))
	if err != nil {
		loglogrus.Log.Errorf("[P2P] 当前节点无法在(%s:%d)上进行TCP连接监听,err:%v\n", IP, Port, err)
		os.Exit(1)
	}
	peer.ListenSocket = listener

	if prv, err := crypto.GenerateKey(); err != nil { // 1.生成私钥
		loglogrus.Log.Errorf("[P2P] 当前节点无法无法生成私钥,err:%v\n", err)
		os.Exit(1)
	} else {
		peer.prvKey = prv
		peer.pubKey = prv.Public().(*ecdsa.PublicKey) // 2.生成公钥
		peer.nodeID = crypto.KeytoNodeID(peer.pubKey) // 3.生成NodeID
	}

	go peer.Receive() // 启动p2p服务器部分
	go peer.Expire()  // 启动过期消息销毁协程

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

// 与其他节点建立TCP连接
func (peer *Peer) LookUpOthers(remoteAddrs []*Address) {
	for _, remote := range remoteAddrs {
		if conn, err := net.Dial("tcp", remote.IP+fmt.Sprintf(":%d", remote.Port)); err != nil {
			loglogrus.Log.Warnf("[P2P] 当前节点(%s:%d)与目标节点(%s:%d)建立tcp连接失败,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port,
				remote.IP, remote.Port, err)
			continue
		} else {
			// 1.等待来自对方节点发送的nodeID
			rbuff := make([]byte, nodeIDBuff)
			n, err := conn.Read(rbuff)
			if err != nil {
				loglogrus.Log.Warnf("[P2P] 接收NodeID数据失败,err:%v\n", err)
				conn.Close()
				continue
			}
			otherNodeID := new(common.NodeID)
			if err := rlp.DecodeBytes(rbuff[:n], otherNodeID); err != nil {
				loglogrus.Log.Errorf("[P2P] 当前节点(%s:%d)无法解码对端发送的的NodeID,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port, err)
				conn.Close()
				continue
			}
			peer.Others[*otherNodeID] = &RemotePeer{NodeID: *otherNodeID, RemoteAddr: Address{IP: remote.IP, Port: remote.Port}, Conn: conn}

			loglogrus.Log.Infof("[P2P] 当前节点(%s:%d)(NodeID:%x)与目标节点(%s:%d)(NodeID:%x)成功建立tcp连接\n", peer.LocalAddr.IP, peer.LocalAddr.Port,
				peer.nodeID, remote.IP, remote.Port, otherNodeID)

			// 2.向对方发送自己的NodeID
			if selfnodeIDBytes, err := rlp.EncodeToBytes(peer.nodeID); err != nil {
				loglogrus.Log.Errorf("[P2P] 当前节点(%s:%d)无法编码自己的NodeID,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port, err)
				conn.Close()
				continue
			} else {
				if _, err := conn.Write(selfnodeIDBytes); err != nil {
					loglogrus.Log.Errorf("[P2P] 当前节点(%s:%d)无法发送自己的NodeID,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port, err)
					conn.Close()
					continue
				}
			}
		}
	}
}

func (peer *Peer) Broadcast(wrapMsg *message.WrapMessage) {
	for _, remote := range peer.Others {
		msgBytes := message.EncodeWrapMessageToBytes(wrapMsg)
		if _, err := remote.Conn.Write(msgBytes); err != nil {
			loglogrus.Log.Warnf("[P2P] 当前节点(%s:%d)向目标节点(%s:%d)发送tcp报文段失败,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port,
				remote.RemoteAddr.IP, remote.RemoteAddr.Port, err)
		} else {
			// loglogrus.Log.Infof("[P2P] 当前节点(%s:%d)向目标节点(%s:%d)发送tcp报文段成功,长度(%d)\n", peer.LocalAddr.IP, peer.LocalAddr.Port,
			// 	remote.RemoteAddr.IP, remote.RemoteAddr.Port, len(msgBytes))
		}
	}
}

func (peer *Peer) Send(desPeer *Address, wrapMsg *message.WrapMessage) {
	dstPeer := &RemotePeer{}

	for _, remote := range peer.Others {
		if reflect.DeepEqual(remote.RemoteAddr, *desPeer) {
			dstPeer.RemoteAddr = remote.RemoteAddr
			dstPeer.Conn = remote.Conn
		}
	}
	msgBytes := message.EncodeWrapMessageToBytes(wrapMsg)
	if _, err := dstPeer.Conn.Write(msgBytes); err != nil {
		loglogrus.Log.Warnf("[P2P] 当前节点(%s:%d)向目标节点(%s:%d)发送tcp报文段失败,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port,
			dstPeer.RemoteAddr.IP, dstPeer.RemoteAddr.Port, err)
	}
}

func (peer *Peer) Receive() {
	for {
		conn, err := peer.ListenSocket.Accept()
		if err != nil {
			loglogrus.Log.Warnf("[P2P] 当前节点(%s:%d)无法接收TCP连接请求,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port, err)
			continue
		}
		// 为每一个 conn socket准备一个单独的协程
		go func(conn net.Conn) {
			// 1.连接成功后，首先需要向对方发送自己的NodeID
			if selfNodeIDBytes, err := rlp.EncodeToBytes(peer.nodeID); err != nil {
				loglogrus.Log.Errorf("[P2P] 当前节点(%s:%d)无法编码自己的NodeID,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port, err)
				conn.Close()
				return
			} else {
				if _, err := conn.Write(selfNodeIDBytes); err != nil {
					loglogrus.Log.Errorf("[P2P] 当前节点(%s:%d)无法发送自己的NodeID,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port, err)
					conn.Close()
					return
				}
			}

			// 2.接着获取对方的NodeID
			rbuff := make([]byte, nodeIDBuff)
			n, err := conn.Read(rbuff)
			if err != nil {
				loglogrus.Log.Warnf("[P2P] 当前节点(%s:%d)接收对端(%s)NodeID数据失败,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port,
					conn.RemoteAddr().String(), err)
				conn.Close()
				return
			}
			otherNodeID := new(common.NodeID)
			if err := rlp.DecodeBytes(rbuff[:n], otherNodeID); err != nil {
				loglogrus.Log.Errorf("[P2P] 当前节点(%s:%d)无法解码对端(%s)发送的的NodeID,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port,
					conn.RemoteAddr().String(), err)
				conn.Close()
				return
			}

			for {
				// 1.等待从conn socket中读取消息
				rbuff := make([]byte, msgBuff)
				n, err := conn.Read(rbuff)
				if err != nil {
					loglogrus.Log.Warnf("[P2P] 当前节点(%s:%d)接收对端(%s)TCP数据失败,err:%v\n", peer.LocalAddr.IP, peer.LocalAddr.Port,
						conn.RemoteAddr().String(), err)
					continue
				}
				// 2.对消息进行解析
				wrapMsg := message.DecodeWrapMessageFromBytes(rbuff[:n])
				msg := message.DecodeWrapMessage(wrapMsg)

				// 3.验证消息是否合法(验证数字签名)
				if pass := msg.ValidateSignature(msg.BackHash(), *otherNodeID, msg.BackSignature()); pass {
					loglogrus.Log.Infof("[P2P] 当前节点(%s:%d)对对端(%s)消息的数字签名验证通过,此为合法消息\n", peer.LocalAddr.IP, peer.LocalAddr.Port,
						conn.RemoteAddr().String())
					// 4.将通过验证的消息存放到对应的缓冲区中
					peer.MsgMutex.Lock()
					peer.MessagePool[msg.BackHash()] = msg
					peer.MsgMutex.Unlock()
				} else {
					loglogrus.Log.Warnf("[P2P] 当前节点(%s:%d)对对端(%s)消息的数字签名验证不通过,此为非法消息\n", peer.LocalAddr.IP, peer.LocalAddr.Port,
						conn.RemoteAddr().String())
				}
			}

		}(conn)
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
