package tangle

import (
	"blockChain_consensus/tangleChain/common"
	"blockChain_consensus/tangleChain/crypto"
	"fmt"
	"reflect"
	"testing"
)

func TestTxEncodeAndDecode(t *testing.T) {
	tx := NewGenesisTx(common.NodeID{}) // 生成交易
	tx.SelectApproveTx(nil)             // 为交易选择支持的前置交易(需要合法)
	tx.Pow()                            // 对该交易进行Pow验证
	// 交易打包成 wrap Message , 从而通过p2p网络进行传播
	prv, _ := crypto.GenerateKey() // 1.生成私钥

	wrapMsg := EncodeTxToWrapMsg(tx, prv)

	// 从 wrap Message 中解包出交易
	newRawTx := DecodeTxFromCommonMsg(wrapMsg)

	if reflect.DeepEqual(tx.RawTx, newRawTx) {
		fmt.Println("将交易打包进Wrap Message前后没有发生篡改")
		if tx.PowValidator() {
			fmt.Println("Pow验证通过")
		} else {
			fmt.Println("Pow验证失败")
		}

	} else {
		fmt.Println("将交易打包进Wrap Message前后发生了篡改")
		fmt.Println("before: ", tx.RawTx)
		fmt.Println("after: ", newRawTx)
	}
}
