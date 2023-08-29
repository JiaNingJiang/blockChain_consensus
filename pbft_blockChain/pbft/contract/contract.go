package contract

import (
	"errors"
	"pbft_blockchain/database"
)

func ContractFuncRun(worldState database.Database, contract string, function string, args [][]byte) (string, error) {
	switch contract {
	case "Common":
		return CommonContract(worldState, function, args)
	case "BGP":
		panic("to do")
	default:
		return "", errors.New("不存在的合约")
	}
}
