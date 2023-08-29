package errorMsg

var MsgFlags = map[int]string{
	SUCCESS:                     "ok",
	ERROR:                       "fail",
	INVALID_PARAMS:              "请求参数错误",
	INVALID_NODE_ID:             "无效的节点ID",
	ERROR_NOT_EXIST_BLOCK:       "请求区块不存在",
	ERROR_EMPTY_BLOCKCHAIN:      "当前节点区块链为空",
	ERROR_NOT_EXIST_TRANSACTION: "请求交易不存在",
	ERROR_NOT_EXIST_DPNETWORK:   "当前节点无法查看DPNetWork信息",
	ERROR_NOT_EXIST_DPSUBNET:    "当前节点无法查看指定的DPSubNet信息",
}

func GetMsg(code int) string {
	msg, ok := MsgFlags[code]
	if ok {
		return msg
	}

	return MsgFlags[ERROR]
}
