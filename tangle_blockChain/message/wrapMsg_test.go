package message

import (
	"blockChain_consensus/tangleChain/crypto"
	"crypto/ecdsa"
	"fmt"
	"reflect"
	"testing"
)

func TestWrapMsg(t *testing.T) {

	prv, _ := crypto.GenerateKey()         // 1.生成私钥
	pub := prv.Public().(*ecdsa.PublicKey) // 2.生成公钥
	nodeID := crypto.KeytoNodeID(pub)      // 3.生成NodeID
	cMsg := NewCommonMsg([]byte("hello"), nodeID, prv)

	wrapMsg := EncodeToWrapMessage(cMsg)
	wrapMsgBytes := EncodeWrapMessageToBytes(wrapMsg)

	newWrapMsg := DecodeWrapMessageFromBytes(wrapMsgBytes)

	if !reflect.DeepEqual(wrapMsg, newWrapMsg) {
		fmt.Println("rlp编解码前后的 Wrap Message不相同")
	} else {
		fmt.Println("rlp编解码前后的 Wrap Message相同")
	}

	newCMsg := DecodeWrapMessage(newWrapMsg)

	fmt.Printf("消息的具体类型是: %d ,消息载荷为: (%s)\n", newCMsg.MsgType(), newCMsg.BackPayload())

}
