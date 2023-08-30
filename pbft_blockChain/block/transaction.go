package block

import (
	"blockChain_consensus/pbftChain/common"
	"blockChain_consensus/pbftChain/crypto"
	loglogrus "blockChain_consensus/pbftChain/log_logrus"
	"blockChain_consensus/pbftChain/rlp"
	"crypto/ecdsa"
)

type Transaction struct {
	TxID       common.Hash
	ClientID   string // 请求客户端的标识符
	ClientAddr string
	TimeStamp  uint64        // 客户端请求的时间戳
	Sender     common.NodeID // 交易发送者的NodeID
	Signature  []byte        // 交易生成者的签名

	Contract string
	Function string
	Args     [][]byte // function argument

}

func NewTransaction(clientID string, clientAddr string, sender common.NodeID, nonce uint64, version common.Hash, contract, function string, args [][]byte) *Transaction {
	return &Transaction{
		ClientID:   clientID,
		ClientAddr: clientAddr,
		Sender:     sender,
		Contract:   contract,
		Function:   function,
		Args:       args,
	}
}

func (tx *Transaction) Hash() common.Hash {
	plainTx := *tx
	plainTx.TxID = common.Hash{}
	plainTx.Signature = []byte{} // 不同的人对同一笔交易会有不同的数字签名,因此数字签名不能包含在交易哈希的计算中
	summary, err := rlp.EncodeToBytes(&plainTx)
	if err != nil {
		loglogrus.Log.Warnf("[Transaction] Fail in transaction hash!")
	}

	txHash := crypto.Sha3Hash(summary)

	tx.TxID = txHash

	return txHash
}

// 计算数字签名
func (tx *Transaction) DigitalSignature(msgHash common.Hash, prv *ecdsa.PrivateKey) []byte {
	if signature, err := crypto.Sign(msgHash.Bytes(), prv); err != nil {
		loglogrus.Log.Warnf("[Transaction] 对Common Msg进行数字签名失败,err=%v\n", err)
		return nil
	} else {
		tx.Signature = signature
		return signature
	}
}
