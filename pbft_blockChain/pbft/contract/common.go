package contract

import (
	"blockChain_consensus/pbftChain/database"
	loglogrus "blockChain_consensus/pbftChain/log_logrus"
	"errors"
)

func CommonContract(worldState database.Database, function string, args [][]byte) (string, error) {
	switch function {
	case "Write":
		if len(args) != 2 {
			loglogrus.Log.Warnf("[contract] args: %v\n", len(args))
			return "", errors.New("参数数量不对")
		}
		key := string(args[0])

		value := string(args[1])

		err := CommonExecuteWrite(worldState, key, value)
		return "写入成功", err

	case "Read":
		if len(args) != 1 {
			return "", errors.New("参数数量不对")
		}
		key := string(args[0])
		value, err := CommonExecuteRead(worldState, key)
		return value, err

	default:
		return "", errors.New("不存在的合约函数")
	}
}

// 简单的实现合约功能(写入操作)
func CommonExecuteWrite(worldState database.Database, key, value string) error {
	keyBytes := []byte(key)
	valueBytes := []byte(value)

	if err := worldState.Put(keyBytes, valueBytes); err != nil {
		loglogrus.Log.Infof("[Contract] 执行Common::Write失败   key:%s  err:%v\n", key, err)
		return err
	} else {
		loglogrus.Log.Infof("[Contract] 执行Common::Write成功   key:%s  value:%s\n", key, value)
		return nil
	}

}

// 简单的实现合约功能(读取操作)
func CommonExecuteRead(worldState database.Database, key string) (string, error) {
	keyBytes := []byte(key)
	if valueBytes, err := worldState.Get(keyBytes); err != nil {
		loglogrus.Log.Infof("[Contract] 执行Common::Read失败   key:%s  err:%v\n", key, err)
		return "", err
	} else {
		loglogrus.Log.Infof("[Contract] 执行Common::Read成功   key:%s  得到的value:%s\n", key, string(valueBytes))
		return string(valueBytes), nil
	}
}
