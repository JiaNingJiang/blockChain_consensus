package fsm

import (
	"fmt"
	"io"
	"strings"

	contract "raftClient/contract"
	"raftClient/database"
	loglogrus "raftClient/log_logrus"

	"github.com/hashicorp/raft"
)

type Fsm struct {
	DataBase *database.Database
}

func NewFsm() *Fsm {
	fsm := &Fsm{
		DataBase: database.NewDatabase(),
	}
	return fsm
}

// TODO:需要实现注册所有会改变数据库状态的操作(比如说,写操作都需要实现注册,而读操作不需要)  这里可以引入数字签名验证功能
func (f *Fsm) Apply(l *raft.Log) interface{} {
	fmt.Println("apply data:", string(l.Data))
	data := strings.Split(string(l.Data), "-")
	op := data[0]

	switch op {
	case "set": // 单纯的数据库写操作
		key := data[1]
		value := data[2]
		f.DataBase.Set(key, value)

	case "tx": // 接收到的是交易,需要根据合约进行执行
		contractName := data[1]
		functionName := data[2]
		argsStr := data[3]

		args := strings.Split(argsStr, " ")

		if res, err := contract.ContractFuncRun(f.DataBase, contractName, functionName, args); err != nil {
			loglogrus.Log.Warnf("[FSM] 合约执行错误,err:%v\n", err)
			return err
		} else {
			return res
		}
	}

	return nil
}

func (f *Fsm) Snapshot() (raft.FSMSnapshot, error) {
	return f.DataBase, nil
}

func (f *Fsm) Restore(io.ReadCloser) error {
	return nil
}
