package contract

import (
	"errors"
	"raftClient/database"
)

func ContractFuncRun(worldState *database.Database, contract string, function string, args []string) (string, error) {
	switch contract {
	case "Common":
		return CommonContract(worldState, function, args)
	case "BGP":
		panic("to do")
	default:
		return "", errors.New("不存在的合约")
	}
}
