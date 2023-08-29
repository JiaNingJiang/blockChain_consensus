package keys

import (
	"blockChain_consensus/tangleChain/common"
	"blockChain_consensus/tangleChain/crypto"
	"bytes"
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"

	"github.com/pborman/uuid"
	"golang.org/x/crypto/pbkdf2"
)

const (
	version = 3
)

type Key struct {
	Id uuid.UUID // Version 4 "random" for unique id not derived from key data
	// to simplify lookups we also store the address
	Address common.Address
	// we only store privkey as pubkey/address can be derived from it
	// privkey in this struct is always in plaintext
	PrivateKey *ecdsa.PrivateKey
}

type plainKeyJSON struct {
	Address    string `json:"address"`
	PrivateKey string `json:"privatekey"`
	Id         string `json:"id"`
	Version    int    `json:"version"`
}

type encryptedKeyJSONV3 struct {
	Address string `json:"address"`
	Crypto  cryptoJSON
	Id      string `json:"id"`
	Version int    `json:"version"`
}

type encryptedKeyJSONV1 struct {
	Address string `json:"address"`
	Crypto  cryptoJSON
	Id      string `json:"id"`
	Version string `json:"version"`
}

type cryptoJSON struct {
	Cipher       string                 `json:"cipher"`
	CipherText   string                 `json:"ciphertext"`
	CipherParams cipherparamsJSON       `json:"cipherparams"`
	KDF          string                 `json:"kdf"`
	KDFParams    map[string]interface{} `json:"kdfparams"`
	MAC          string                 `json:"mac"`
}

type cipherparamsJSON struct {
	IV string `json:"iv"`
}

type scryptParamsJSON struct {
	N     int    `json:"n"`
	R     int    `json:"r"`
	P     int    `json:"p"`
	DkLen int    `json:"dklen"`
	Salt  string `json:"salt"`
}

func (k *Key) MarshalJSON() (j []byte, err error) {
	jStruct := plainKeyJSON{
		hex.EncodeToString(k.Address[:]),
		hex.EncodeToString(crypto.FromECDSA(k.PrivateKey)),
		k.Id.String(),
		version,
	}
	j, err = json.Marshal(jStruct)
	return j, err
}

func (k *Key) UnmarshalJSON(j []byte) (err error) {
	keyJSON := new(plainKeyJSON)
	err = json.Unmarshal(j, &keyJSON)
	if err != nil {
		return err
	}

	u := new(uuid.UUID)
	*u = uuid.Parse(keyJSON.Id)
	k.Id = *u
	addr, err := hex.DecodeString(keyJSON.Address)
	if err != nil {
		return err
	}

	privkey, err := hex.DecodeString(keyJSON.PrivateKey)
	if err != nil {
		return err
	}

	k.Address = common.BytesToAddress(addr)
	k.PrivateKey = crypto.ToECDSA(privkey)

	return nil
}

func NewKeyFromECDSA(privateKeyECDSA *ecdsa.PrivateKey) *Key {
	id := uuid.NewRandom()
	key := &Key{
		Id:         id,
		Address:    crypto.PubkeyToAddress(privateKeyECDSA.PublicKey),
		PrivateKey: privateKeyECDSA,
	}
	return key
}

func NewKey(rand io.Reader) *Key {
	randBytes := make([]byte, 64)
	_, err := rand.Read(randBytes)
	if err != nil {
		panic("key generation: could not read from random source: " + err.Error())
	}
	reader := bytes.NewReader(randBytes)
	privateKeyECDSA, err := ecdsa.GenerateKey(crypto.S256(), reader)
	if err != nil {
		panic("key generation: ecdsa.GenerateKey failed: " + err.Error())
	}

	return NewKeyFromECDSA(privateKeyECDSA)
}

// TODO: at this time, we are not sure a presale key is necessary in DP-Chain
// creates a Key and stores that in the given KeyStore by decrypting a presale key JSON
func ImportPreSaleKey(keyStore KeyStore, keyJSON []byte, password string) (*Key, error) {
	key, err := decryptPreSaleKey(keyJSON, password)
	if err != nil {
		return nil, err
	}
	key.Id = uuid.NewRandom()
	err = keyStore.StoreKey(key, password)
	return key, err
}

// TODO: As DP-Chain is a consortium blockchain, we could set a special SALT
func decryptPreSaleKey(fileContent []byte, password string) (key *Key, err error) {
	preSaleKeyStruct := struct {
		EncSeed string
		DpAddr  string
		Email   string
	}{}
	err = json.Unmarshal(fileContent, &preSaleKeyStruct)
	if err != nil {
		return nil, err
	}
	encSeedBytes, err := hex.DecodeString(preSaleKeyStruct.EncSeed)
	if err != nil {
		return nil, err
	}
	iv := encSeedBytes[:16]
	cipherText := encSeedBytes[16:]
	passBytes := []byte(password)
	derivedKey := pbkdf2.Key(passBytes, passBytes, 2000, 16, sha256.New)
	plainText, err := aesCBCDecrypt(derivedKey, cipherText, iv)
	dpPriv := crypto.Sha3(plainText)
	ecKey := crypto.ToECDSA(dpPriv)
	key = &Key{
		Id:         nil,
		Address:    crypto.PubkeyToAddress(ecKey.PublicKey),
		PrivateKey: ecKey,
	}
	derivedAddr := hex.EncodeToString(key.Address.Bytes()) // needed because .Hex() gives leading "0x"
	expectedAddr := preSaleKeyStruct.DpAddr
	if derivedAddr != expectedAddr {
		err = fmt.Errorf("decrypted addr %s not equal to expected addr %s", derivedAddr, expectedAddr)
	}
	return key, err
}

// AES-128 is selected due to size of encryptKey
func aesCTRXOR(key, inText, iv []byte) ([]byte, error) {
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	stream := cipher.NewCTR(aesBlock, iv)
	outText := make([]byte, len(inText))
	stream.XORKeyStream(outText, inText)
	return outText, err
}

func aesCBCDecrypt(key, cipherText, iv []byte) ([]byte, error) {
	aesBlock, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	decrypter := cipher.NewCBCDecrypter(aesBlock, iv)
	paddedPlaintext := make([]byte, len(cipherText))
	decrypter.CryptBlocks(paddedPlaintext, cipherText)
	plaintext := PKCS7Unpad(paddedPlaintext)
	if plaintext == nil {
		err = errors.New("Decryption failed: PKCS7Unpad failed after AES decryption")
	}
	return plaintext, err
}

func PKCS7Unpad(in []byte) []byte {
	if len(in) == 0 {
		return nil
	}

	padding := in[len(in)-1]
	if int(padding) > len(in) || padding > aes.BlockSize {
		return nil
	} else if padding == 0 {
		return nil
	}

	for i := len(in) - 1; i > len(in)-int(padding)-1; i-- {
		if in[i] != padding {
			return nil
		}
	}
	return in[:len(in)-int(padding)]
}
