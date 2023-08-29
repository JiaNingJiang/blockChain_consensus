package common

import (
	"bytes"
	"testing"
)

func TestAddressesCatenateWsKey(t *testing.T) {
	// addr1 := StringToAddress("Hello,")
	// addr2 := StringToAddress("Wolrd!")
	addr1 := HexToAddress("000000000000000000000000000048656c6c6f2c")
	addr2 := HexToAddress("0000000000000000000000000000576f6c726421")

	// t.Logf("addr1: %x, addr2: %x \n", addr1, addr2)
	catenatedWsKey := AddressesCatenateWsKey(addr1, addr2)
	// t.Logf("Catenated WsKey: %x", catenatedWsKey)

	if !bytes.Equal(Hex2Bytes("000000000000000000000000000048656c6c6f2c0000000000000000000000000000576f6c726421"), catenatedWsKey[:]) {
		t.Fatalf("addr1: %x, addr2: %x, but catenated key is: %x", addr1, addr2, catenatedWsKey)
	}
}

func TestWsKeySplitToAddresses(t *testing.T) {
	addr1 := HexToAddress("000000000000000000000000000048656c6c6f2c")
	addr2 := HexToAddress("0000000000000000000000000000576f6c726421")

	catenatedWsKey := AddressesCatenateWsKey(addr1, addr2)
	split1, split2 := WsKeySplitToAddresses(catenatedWsKey)
	if !(addr1 == split1 && addr2 == split2) {
		t.Fatalf("Catenated key is: %x, split1: %x, split2: %x", catenatedWsKey, split1, split2)
	}
}
