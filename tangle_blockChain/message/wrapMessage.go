package message

import (
	loglogrus "blockChain_consensus/tangleChain/log_logrus"
	"blockChain_consensus/tangleChain/rlp"
)

type WrapMessage struct {
	MsgType uint64
	Content []byte
}

func EncodeWrapMessageToBytes(wrapMsg *WrapMessage) []byte {
	if bytes, err := rlp.EncodeToBytes(wrapMsg); err != nil {
		loglogrus.Log.Warnf("[Message] 无法将WrapMessage编辑为字节流,err=%v\n", err)
		return nil
	} else {
		return bytes
	}
}

func DecodeWrapMessageFromBytes(msgBytes []byte) *WrapMessage {
	wrapMsg := new(WrapMessage)

	if err := rlp.DecodeBytes(msgBytes, wrapMsg); err != nil {
		loglogrus.Log.Warnf("[Message] 无法将字节流解析为WrapMessage,err=%v\n", err)
		return nil
	} else {
		return wrapMsg
	}
}

// 将具体的消息统一打包成 WrapMessage 用于发送
func EncodeToWrapMessage(msg MsgInterface) *WrapMessage {
	switch msg.MsgType() {
	case CommonCode:
		cMsg := msg.(*Common)
		cMsgBytes := cMsg.EncodeToBytes()
		wrapMsg := new(WrapMessage)
		wrapMsg.MsgType = CommonCode
		wrapMsg.Content = cMsgBytes

		return wrapMsg

	default:
		return nil
	}
}

// 将接收到的 WrapMessage 拆解成 具体的消息
func DecodeWrapMessage(wrapMsg *WrapMessage) MsgInterface {
	switch wrapMsg.MsgType {
	case CommonCode:
		cMsg := new(Common)
		if err := rlp.DecodeBytes(wrapMsg.Content, cMsg); err != nil {
			loglogrus.Log.Warnf("[Message] 无法将WrapMessage解析为Common Msg,err=%v\n", err)
			return nil
		} else {
			return cMsg
		}
	default:
		return nil
	}
}
