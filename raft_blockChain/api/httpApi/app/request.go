package app

import (
	loglogrus "raftClient/log_logrus"

	"github.com/astaxie/beego/validation"
)

func MarkErrors(method string, errors []*validation.Error) {
	for _, err := range errors {
		loglogrus.Log.Warnf("Http: %s Validation is failed -- Param:%v , ErrMsg:%v\n", method, err.Key, err.Message)
	}

	return
}
