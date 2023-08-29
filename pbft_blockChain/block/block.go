package block

import (
	"blockChain_consensus/pbftChain/common"
	"blockChain_consensus/pbftChain/crypto"
	loglogrus "blockChain_consensus/pbftChain/log_logrus"
	"blockChain_consensus/pbftChain/rlp"
	"crypto/ecdsa"
)

type Block struct {
	BlockID      common.Hash
	Sender       common.NodeID // 区块的发送者
	Transactions []Transaction
	Votes        []Signature // 每个节点收集到对同一个区块的 2*f 个不同的数字签名时,才会认可此区块,从而向其他节点发送commit。当在收到2*f个commit后才会决定执行区块内交易
}
type Signature []byte

func NewBlock(sender common.NodeID, txSet []Transaction) *Block {
	b := &Block{
		Sender:       sender,
		Transactions: txSet,
		Votes:        make([]Signature, 0),
	}
	b.BlockID, _ = b.Hash()

	return b
}

// 对区块求哈希,其实就是对交易集合求哈希
func (b *Block) Hash() (common.Hash, error) {
	tempBlock := *b
	tempBlock.BlockID = common.Hash{}
	tempBlock.Sender = common.NodeID{}
	tempBlock.Votes = make([]Signature, 0)

	summary, err := rlp.EncodeToBytes(&tempBlock)
	if err != nil {
		return common.Hash{}, err
	}
	return crypto.Sha3Hash(summary), nil
}

// 计算数字签名
func (b *Block) DigitalSignature(msgHash common.Hash, prv *ecdsa.PrivateKey) []byte {
	if signature, err := crypto.Sign(msgHash.Bytes(), prv); err != nil {
		loglogrus.Log.Warnf("[Block] 对区块Block进行数字签名失败,err=%v\n", err)
		return nil
	} else {
		b.Votes = append(b.Votes, signature)
		return signature
	}
}

// 返回所有交易的哈希值
func (b *Block) BackTransactionIDs() []common.Hash {
	txIDs := make([]common.Hash, len(b.Transactions))
	for i := 0; i < len(txIDs); i++ {
		txIDs[i] = b.Transactions[i].TxID
	}
	return txIDs
}
