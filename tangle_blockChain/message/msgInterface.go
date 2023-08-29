package message

import (
	"crypto/ecdsa"
	"tangle/common"
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
