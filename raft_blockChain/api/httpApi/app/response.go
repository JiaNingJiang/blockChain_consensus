package app

import (
	errorMsg "blockChain_consensus/raftChain/api/httpApi/error"

	"github.com/gin-gonic/gin"
)

type Gin struct {
	C *gin.Context
}

func (g *Gin) Response(httpCode, errCode int, data interface{}) {
	g.C.JSON(httpCode, gin.H{
		"code": errCode,
		"msg":  errorMsg.GetMsg(errCode),
		"data": data,
	})

}
