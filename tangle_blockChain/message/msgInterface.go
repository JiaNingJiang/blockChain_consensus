package message

import (
	"blockChain_consensus/tangleChain/common"
	"crypto/ecdsa"
)

const (
	CommonCode = 0x00
)

type MsgInterface interface {
	MsgType() uint64
	MarkRetrieved()
	IsRetrieved() byte
	BackHash() common.Hash
	BackPayload() interface{}
	BackTimeStamp() uint64
	BackSignature() []byte
	DigitalSignature(msgHash common.Hash, prv *ecdsa.PrivateKey) []byte
	ValidateSignature(msgHash common.Hash, nodeID common.NodeID, sig []byte) bool
}
