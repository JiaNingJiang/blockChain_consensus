package consensus

import (
	"blockChain_consensus/pbftChain/common"
	"blockChain_consensus/pbftChain/crypto"
	loglogrus "blockChain_consensus/pbftChain/log_logrus"
	"bytes"
	"crypto/sha256"
	"encoding/hex"
)

func Hash(content []byte) string {
	h := sha256.New()
	h.Write(content)
	return hex.EncodeToString(h.Sum(nil))
}

// 验证数字签名
func ValidateSignature(msgHash common.Hash, nodeID common.NodeID, sig []byte) bool {
	pub, err := crypto.NodeIDtoKey(nodeID)
	if err != nil {
		loglogrus.Log.Warnf("[consensus] 数字签名验证失败,发送方的NodeID是非法的,err=%v\n", err)
		return false
	}
	// 1.获取公钥的字节流
	publicKeyBytes := crypto.FromECDSAPub(pub)
	// 2.从Msg中获取出签名公钥
	sigPublicKey, err := crypto.Ecrecover(msgHash.Bytes(), sig)
	if err != nil {
		loglogrus.Log.Warnf("[consensus] 数字签名验证失败,无法根据数字签名和消息哈希解析出合法的公钥,err=%v\n", err)
		return false
	}

	// 3.比较签名公钥是否正确
	matches := bytes.Equal(sigPublicKey, publicKeyBytes)

	return matches

}
