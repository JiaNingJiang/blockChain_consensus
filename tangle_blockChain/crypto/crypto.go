package crypto

import (
	"blockChain_consensus/tangleChain/common"
	"blockChain_consensus/tangleChain/crypto/ecies"
	"blockChain_consensus/tangleChain/crypto/secp256k1"
	"blockChain_consensus/tangleChain/crypto/sha3"
	"crypto/ecdsa"
	"crypto/elliptic"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"math/big"
	"os"
)

func Sha3(data ...[]byte) []byte {
	d := sha3.NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	return d.Sum(nil)
}

func Sha3Hash(data ...[]byte) (h common.Hash) {
	d := sha3.NewKeccak256()
	for _, b := range data {
		d.Write(b)
	}
	d.Sum(h[:0])
	return h
}

func Sign(hash []byte, prv *ecdsa.PrivateKey) (sig []byte, err error) {
	if len(hash) != 32 {
		return nil, fmt.Errorf("hash is required to be exactly 32 bytes (%d)", len(hash))
	}

	sig, err = secp256k1.Sign(hash, common.LeftPadBytes(prv.D.Bytes(), prv.Params().BitSize/8))
	return
}

func GenerateKey() (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(S256(), rand.Reader)
}

func FromECDSAPub(pub *ecdsa.PublicKey) []byte {
	if pub == nil || pub.X == nil || pub.Y == nil {
		return nil
	}
	return elliptic.Marshal(S256(), pub.X, pub.Y)
}

func ToECDSAPub(pub []byte) *ecdsa.PublicKey {
	if len(pub) == 0 {
		return nil
	}
	x, y := elliptic.Unmarshal(S256(), pub)
	return &ecdsa.PublicKey{S256(), x, y}
}

func Decrypt(prv *ecdsa.PrivateKey, ct []byte) ([]byte, error) {
	key := ecies.ImportECDSA(prv)
	return key.Decrypt(rand.Reader, ct, nil, nil)
}

func S256() elliptic.Curve {
	return secp256k1.S256()
}

func SigToPub(hash, sig []byte) (*ecdsa.PublicKey, error) {
	s, err := Ecrecover(hash, sig)
	if err != nil {
		return nil, err
	}

	x, y := elliptic.Unmarshal(S256(), s)
	return &ecdsa.PublicKey{S256(), x, y}, nil
}

func Ecrecover(hash, sig []byte) ([]byte, error) {
	return secp256k1.RecoverPubkey(hash, sig)
}

func Encrypt(pub *ecdsa.PublicKey, message []byte) ([]byte, error) {
	return ecies.Encrypt(rand.Reader, ecies.ImportECDSAPublic(pub), message, nil, nil)
}

func FromECDSA(prv *ecdsa.PrivateKey) []byte {
	if prv == nil {
		return nil
	}
	return prv.D.Bytes()
}

func ToECDSA(prv []byte) *ecdsa.PrivateKey {
	if len(prv) == 0 {
		return nil
	}

	priv := new(ecdsa.PrivateKey)
	priv.PublicKey.Curve = S256()
	priv.D = common.BigD(prv)
	priv.PublicKey.X, priv.PublicKey.Y = S256().ScalarBaseMult(prv)
	return priv
}

func PubkeyToAddress(p ecdsa.PublicKey) common.Address {
	pubBytes := FromECDSAPub(&p)
	return common.BytesToAddress(Sha3(pubBytes[1:])[12:])
}

// SaveECDSA saves a secp256k1 private key to the given file with
// restrictive permissions. The key data is saved hex-encoded.
func SaveECDSA(file string, key *ecdsa.PrivateKey) error {
	k := hex.EncodeToString(FromECDSA(key))
	return ioutil.WriteFile(file, []byte(k), 0600)
}

// LoadECDSA loads a secp256k1 private key from the given file.
// The key data is expected to be hex-encoded.
func LoadECDSA(file string) (*ecdsa.PrivateKey, error) {
	buf := make([]byte, 64)
	fd, err := os.Open(file)
	if err != nil {
		return nil, err
	}
	defer fd.Close()
	if _, err := io.ReadFull(fd, buf); err != nil {
		return nil, err
	}

	key, err := hex.DecodeString(string(buf))
	if err != nil {
		return nil, err
	}

	return ToECDSA(key), nil
}

// KeytoNodeID returns a marshaled representation of the given public key.
func KeytoNodeID(pub *ecdsa.PublicKey) common.NodeID {
	var id common.NodeID
	pbytes := elliptic.Marshal(pub.Curve, pub.X, pub.Y)
	if len(pbytes)-1 != len(id) {
		panic(fmt.Errorf("need %d bit pubkey, got %d bits", (len(id)+1)*8, len(pbytes)))
	}
	copy(id[:], pbytes[1:])
	return id
}

// NodeIDtoKey returns the public key represented by the node ID.
// It returns an error if the ID is not a point on the curve.
func NodeIDtoKey(id common.NodeID) (*ecdsa.PublicKey, error) {
	p := &ecdsa.PublicKey{Curve: S256(), X: new(big.Int), Y: new(big.Int)}
	half := len(id) / 2
	p.X.SetBytes(id[:half])
	p.Y.SetBytes(id[half:])
	if !p.Curve.IsOnCurve(p.X, p.Y) {
		return nil, errors.New("not a point on the S256 curve")
	}
	return p, nil
}

// NodeIDtoAddress returns the address of a node
func NodeIDtoAddress(id common.NodeID) (common.Address, error) {
	p, err := NodeIDtoKey(id)
	if err != nil {
		return common.Address{}, err
	}
	return PubkeyToAddress(*p), nil
}

// To valid the signature with the given address
func SignatureValid(address common.Address, signature []byte, hash common.Hash) (bool, error) {
	pbk, err := SigToPub(hash[:], signature)
	if err != nil {
		return false, err
	}
	if PubkeyToAddress(*pbk) == address {
		return true, nil
	} else {
		return false, nil
	}
}

// use signature and hash to compute out the address
func Signature2Addr(signature []byte, hash common.Hash) (common.Address, error) {
	pbk, err := SigToPub(hash[:], signature)
	if err != nil {
		return common.Address{}, err
	}
	addr := PubkeyToAddress(*pbk)
	return addr, nil
}

func SignHash(hash common.Hash, prv *ecdsa.PrivateKey) (sig []byte, err error) {
	sig, err = secp256k1.Sign(hash[:], common.LeftPadBytes(prv.D.Bytes(), prv.Params().BitSize/8))
	return
}

func GenerateSpecificKey(reader io.Reader) (*ecdsa.PrivateKey, error) {
	return ecdsa.GenerateKey(S256(), reader)
}
