package common

import (
	"encoding/hex"
	"fmt"
	"math/big"
	"math/rand"
	"reflect"
)

const (
	hashLength    = 32
	addressLength = 20
	wsKeyLength   = 40
	nodeIDBits    = 512
)

type (
	Hash    [hashLength]byte
	Address [addressLength]byte // Address is the only kind of way used to represent users, nodes, contracts and contract value storage in dpchain.
	WsKey   [wsKeyLength]byte   // World State Key is catenated by two address (domain address and inner address)

	NodeID [nodeIDBits / 8]byte
)

func BytesToHash(b []byte) Hash {
	var h Hash
	h.SetBytes(b)
	return h
}
func StringToHash(s string) Hash { return BytesToHash([]byte(s)) }
func BigToHash(b *big.Int) Hash  { return BytesToHash(b.Bytes()) }
func HexToHash(s string) Hash    { return BytesToHash(FromHex(s)) }

// Don't use the default 'String' method in case we want to overwrite

// Get the string representation of the underlying hash
func (h Hash) Str() string   { return string(h[:]) }
func (h Hash) Bytes() []byte { return h[:] }
func (h Hash) Big() *big.Int { return Bytes2Big(h[:]) }
func (h Hash) Hex() string   { return "0x" + Bytes2Hex(h[:]) }

// Sets the hash to the value of b. If b is larger than len(h) it will panic
func (h *Hash) SetBytes(b []byte) {
	if len(b) > len(h) {
		b = b[len(b)-hashLength:]
	}

	copy(h[hashLength-len(b):], b)
}

// Set string `s` to h. If s is larger than len(h) it will panic
func (h *Hash) SetString(s string) { h.SetBytes([]byte(s)) }

// Sets h to other
func (h *Hash) Set(other Hash) {
	for i, v := range other {
		h[i] = v
	}
}

// Generate implements testing/quick.Generator.
func (h Hash) Generate(rand *rand.Rand, size int) reflect.Value {
	m := rand.Intn(len(h))
	for i := len(h) - 1; i > m; i-- {
		h[i] = byte(rand.Uint32())
	}
	return reflect.ValueOf(h)
}

func EmptyHash(h Hash) bool {
	return h == Hash{}
}

// ///////// Address
func BytesToAddress(b []byte) Address {
	var a Address
	a.SetBytes(b)
	return a
}
func StringToAddress(s string) Address { return BytesToAddress([]byte(s)) }
func BigToAddress(b *big.Int) Address  { return BytesToAddress(b.Bytes()) }
func HexToAddress(s string) Address    { return BytesToAddress(FromHex(s)) }

// Get the string representation of the underlying address
func (a Address) Str() string   { return string(a[:]) }
func (a Address) Bytes() []byte { return a[:] }
func (a Address) Big() *big.Int { return Bytes2Big(a[:]) }
func (a Address) Hash() Hash    { return BytesToHash(a[:]) }
func (a Address) Hex() string   { return "0x" + Bytes2Hex(a[:]) }

// Sets the address to the value of b. If b is larger than len(a) it will panic
func (a *Address) SetBytes(b []byte) {
	if len(b) > len(a) {
		b = b[len(b)-addressLength:]
	}
	copy(a[addressLength-len(b):], b)
}

// Set string `s` to a. If s is larger than len(a) it will panic
func (a *Address) SetString(s string) { a.SetBytes([]byte(s)) }

// Sets a to other
func (a *Address) Set(other Address) {
	for i, v := range other {
		a[i] = v
	}
}

// PP Pretty Prints a byte slice in the following format:
//
//	hex(value[:4])...(hex[len(value)-4:])
func PP(value []byte) string {
	if len(value) <= 8 {
		return Bytes2Hex(value)
	}

	return fmt.Sprintf("%x...%x", value[:4], value[len(value)-4])
}

// AddressesCatenateWsKey catenate two addresses into one world state key,
// and is like follows:
// addr1 + addr2 = addr1addr2
// 123...201 + 312..341 = 123..201312...341
func AddressesCatenateWsKey(addr1, addr2 Address) WsKey {
	var wsKey WsKey
	copy(wsKey[:], addr1[:])
	copy(wsKey[addressLength:], addr2[:])
	return wsKey
}

// WsKeySplitToAddresses split a world state into domain address and inner
// address
func WsKeySplitToAddresses(wsKey WsKey) (addr1, addr2 Address) {
	copy(addr1[:], wsKey[:addressLength])
	copy(addr2[:], wsKey[addressLength:])
	return addr1, addr2
}

func HashToAddress(hash Hash) Address {
	addr := Address{}
	copy(addr[:], hash[:])
	return addr
}

func WsKeyLess(wsk1, wsk2 WsKey) bool {
	for i := 0; i < wsKeyLength; i++ {
		if wsk1[i] < wsk2[i] {
			return true
		} else if wsk1[i] == wsk2[i] {
			continue
		} else {
			break
		}
	}
	return false
}

func HexToNodeID(s string) (NodeID, error) {
	var bytes NodeID
	decoded, err := hex.DecodeString(s)
	if err != nil {
		return bytes, err
	}
	copy(bytes[:], decoded)
	return bytes, nil
}
