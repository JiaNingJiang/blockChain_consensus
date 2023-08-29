package message

import (
	"blockChain_consensus/tangleChain/common"
	"blockChain_consensus/tangleChain/crypto"
	loglogrus "blockChain_consensus/tangleChain/log_logrus"
	"blockChain_consensus/tangleChain/rlp"
	"bytes"
	"crypto/ecdsa"
	"time"
)

// 一种通用类型的消息
type Common struct {
	MsgCode uint64

	PayLoad   []byte        // 消息内容
	Sender    common.NodeID // 消息的发送者
	TimeStamp uint64        // 时间戳
	Retrieved byte          // 是否已读（0表示未读 1表示已读）

	Hash      common.Hash // 消息哈希值
	Signature []byte      // 消息签名
}

func NewCommonMsg(content []byte, sender common.NodeID, prv *ecdsa.PrivateKey) *Common {
	cMsg := &Common{
		MsgCode:   CommonCode,
		PayLoad:   content,
		Sender:    sender,
		TimeStamp: uint64(time.Now().UnixNano()),
		Retrieved: 0,
		Signature: make([]byte, 0),
	}
	hash := cMsg.BackHash()
	cMsg.DigitalSignature(hash, prv)

	return cMsg
}

func (c *Common) MsgType() uint64 {
	return c.MsgCode
}

func (c *Common) MarkRetrieved() {
	c.Retrieved = 1
}

func (c *Common) IsRetrieved() byte {
	return c.Retrieved
}

func (c *Common) BackPayload() interface{} {
	return string(c.PayLoad)
}

func (c *Common) BackHash() common.Hash {
	if (c.Hash == common.Hash{}) {
		enc, _ := rlp.EncodeToBytes(c) //rlp编码
		c.Hash = crypto.Sha3Hash(enc)  //计算SHA-3 后的哈希值
	}
	return c.Hash
}

func (c *Common) BackTimeStamp() uint64 {
	return c.TimeStamp
}

func (c *Common) DigitalSignature(msgHash common.Hash, prv *ecdsa.PrivateKey) []byte {
	if signature, err := crypto.Sign(msgHash.Bytes(), prv); err != nil {
		loglogrus.Log.Warnf("[Message] 对Common Msg进行数字签名失败,err=%v\n", err)
		return nil
	} else {
		c.Signature = signature
		return signature
	}
}

func (c *Common) BackSignature() []byte {
	return c.Signature
}

func (c *Common) ValidateSignature(msgHash common.Hash, nodeID common.NodeID, sig []byte) bool {
	pub, err := crypto.NodeIDtoKey(nodeID)
	if err != nil {
		loglogrus.Log.Warnf("[Message] 数字签名验证失败,发送方的NodeID是非法的,err=%v\n", err)
		return false
	}
	// 1.获取公钥的字节流
	publicKeyBytes := crypto.FromECDSAPub(pub)
	// 2.从Msg中获取出签名公钥
	sigPublicKey, err := crypto.Ecrecover(msgHash.Bytes(), sig)
	if err != nil {
		loglogrus.Log.Warnf("[Message] 数字签名验证失败,无法根据数字签名和消息哈希解析出合法的公钥,err=%v\n", err)
		return false
	}

	// 3.比较签名公钥是否正确
	matches := bytes.Equal(sigPublicKey, publicKeyBytes)

	return matches

}

func (c *Common) EncodeToBytes() []byte {
	if bytes, err := rlp.EncodeToBytes(c); err != nil {
		loglogrus.Log.Warnf("[Message] 无法将Common Msg编辑为字节流,err=%v\n", err)
		return nil
	} else {
		return bytes
	}
}

func (c *Common) DecodeFromBytes(bytes []byte) *Common {
	cMsg := new(Common)

	if err := rlp.DecodeBytes(bytes, cMsg); err != nil {
		loglogrus.Log.Warnf("[Message] 无法将字节流解析为Common Msg,err=%v\n", err)
		return nil
	} else {
		return cMsg
	}
}
