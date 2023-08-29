package message

import (
	"blockChain_consensus/tangleChain/crypto"
	"crypto/ecdsa"
	"fmt"
	"reflect"
	"testing"
)

func TestCommonMsg(t *testing.T) {

	prv, _ := crypto.GenerateKey() // 1.生成私钥

	pub := prv.Public().(*ecdsa.PublicKey) // 2.生成公钥

	nodeID := crypto.KeytoNodeID(pub) // 3.生成NodeID

	cMsg := NewCommonMsg([]byte("hello"), nodeID, prv)

	fmt.Println("oldMsg: ", cMsg)
	fmt.Println()

	cBytes := cMsg.EncodeToBytes()
	fmt.Println("bytes: ", cBytes)
	fmt.Println()

	newCMsg := cMsg.DecodeFromBytes(cBytes)
	fmt.Println("newMsg: ", newCMsg)
	fmt.Println()

	if reflect.DeepEqual(cMsg, newCMsg) {
		fmt.Println("cMsg 和 newMsg是相同的,编解码成功")
	} else {
		fmt.Println("cMsg 和 newMsg是不相同的,编解码失败")
	}

	fmt.Println("内容: ", newCMsg.BackPayload())
}
