package consensus

import (
	"blockChain_consensus/pbftChain/block"
	"blockChain_consensus/pbftChain/common"
	"blockChain_consensus/pbftChain/crypto"
	loglogrus "blockChain_consensus/pbftChain/log_logrus"
	"crypto/ecdsa"
	"encoding/json"
)

type RequestMsg struct {
	Timestamp  uint64   `json:"timestamp"`
	ClientID   string   `json:"clientID"`
	Operation  string   `json:"operation"`  // 包含合约名称/合约函数名称
	Args       [][]byte `json:"args"`       // 合约函数参数
	SequenceID int64    `json:"sequenceID"` //client产生消息时不需要填，共识过程会为其添加
}

func (requestMsg *RequestMsg) Hash() common.Hash {
	summary, err := json.Marshal(&requestMsg)
	if err != nil {
		loglogrus.Log.Warnf("[Consensus] Fail in RequestMsg hash,err:%v\n", err)
	}

	rMsgHash := crypto.Sha3Hash(summary)

	return rMsgHash
}

type ReplyMsg struct {
	ViewID    uint64 `json:"viewID"`
	Timestamp uint64 `json:"timestamp"`
	ClientID  string `json:"clientID"`

	ResultSet []ExcuteResult `json:"excuteResultSet"`
	NodeName  string         `json:"nodeName"`
	NodeID    common.NodeID  `json:"nodeID"`
	Signature []byte         `json:"signature"`
}

type ExcuteResult struct {
	ClientID  string `json:"clientID"`  // 该交易回执需要返回给哪一个client
	TimeStamp uint64 `json:"timeStamp"` // 交易关联的客户端请求的时间戳
	Result    string `json:"result"`

	Error string `json:"error"`
}

func (replyMsg *ReplyMsg) Hash() common.Hash {
	tempRMsg := *replyMsg
	tempRMsg.NodeID = common.NodeID{} // 不同副本节点的NodeID不一样,会导致哈希结果不同,因此不纳入哈希的计算中
	tempRMsg.NodeName = ""            // 理由同上
	tempRMsg.Signature = []byte{}

	summary, err := json.Marshal(&tempRMsg)
	if err != nil {
		loglogrus.Log.Warnf("[Consensus] Fail in ReplyMsg hash,err:%v\n", err)
	}

	rMsgHash := crypto.Sha3Hash(summary)

	return rMsgHash
}

func (replyMsg *ReplyMsg) DigitalSignature(prv *ecdsa.PrivateKey) []byte {

	msgHash := replyMsg.Hash()

	if signature, err := crypto.Sign(msgHash.Bytes(), prv); err != nil {
		loglogrus.Log.Warnf("[Consensus] 对ReplyMsg进行数字签名失败,err=%v\n", err)
		return nil
	} else {
		replyMsg.Signature = signature
		return signature
	}
}

type NodeInfo struct {
	NodeID   common.NodeID
	NodeName string
	NodeUrl  string
}

type PrePrepareMsg struct {
	ViewID     uint64 `json:"viewID"`
	SequenceID int64  `json:"sequenceID"`

	NodeName string        `json:"nodeName"` // 区块签名方的NodeName
	NodeID   common.NodeID `json:"nodeID"`   // 区块签名方的NodeID
	Block    *block.Block  `json:"block"`    // RequestMsg中提取的信息组成交易 (TODO:为了提高TPS，这里应该改成交易集合)
}

type VoteMsg struct {
	MsgType    `json:"msgType"`
	ViewID     uint64 `json:"viewID"`
	SequenceID int64  `json:"sequenceID"`

	NodeName string        `json:"nodeName"`
	NodeID   common.NodeID `json:"nodeID"` // 交易签名方的NodeID
	Block    *block.Block  `json:"block"`
}

type MsgType int

const (
	PrepareMsg MsgType = iota
	CommitMsg
)
