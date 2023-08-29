package keys

import (
	"blockChain_consensus/tangleChain/common"
	"blockChain_consensus/tangleChain/crypto"
	"blockChain_consensus/tangleChain/crypto/randentropy"
	loglogrus "blockChain_consensus/tangleChain/log_logrus"
	"bytes"
	"crypto/aes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"reflect"

	"github.com/pborman/uuid"
	"golang.org/x/crypto/pbkdf2"
	"golang.org/x/crypto/scrypt"
)

const (
	keyHeaderKDF = "scrypt"
	// 2^18 / 8 / 1 uses 256MB memory and approx 1s CPU time on a modern CPU.
	scryptN     = 1 << 18
	scryptr     = 8
	scryptp     = 1
	scryptdkLen = 32
)

type keyStorePassphrase struct {
	keysDirPath string
}

func NewKeyStorePassphrase(path string) KeyStore {
	return &keyStorePassphrase{path}
}

func (ks keyStorePassphrase) GenerateNewKey(rand io.Reader, auth string) (key *Key, err error) {
	return GenerateNewKeyDefault(ks, rand, auth)
}

func (ks keyStorePassphrase) GetKey(keyAddr common.Address, auth string) (key *Key, err error) {
	keyBytes, keyId, err := decryptKeyFromFile(ks.keysDirPath, keyAddr, auth)
	if err == nil {
		key = &Key{
			Id:         uuid.UUID(keyId),
			Address:    keyAddr,
			PrivateKey: crypto.ToECDSA(keyBytes),
		}
	}
	return
}

func (ks keyStorePassphrase) Cleanup(keyAddr common.Address) (err error) {
	return cleanup(ks.keysDirPath, keyAddr)
}

func (ks keyStorePassphrase) GetKeyAddresses() (addresses []common.Address, err error) {
	return getKeyAddresses(ks.keysDirPath)
}

func (ks keyStorePassphrase) StoreKey(key *Key, auth string) (err error) {
	authArray := []byte(auth)
	salt := randentropy.GetEntropyCSPRNG(32)
	derivedKey, err := scrypt.Key(authArray, salt, scryptN, scryptr, scryptp, scryptdkLen)
	if err != nil {
		return err
	}

	encryptKey := derivedKey[:16]
	keyBytes := crypto.FromECDSA(key.PrivateKey)

	iv := randentropy.GetEntropyCSPRNG(aes.BlockSize) // 16
	cipherText, err := aesCTRXOR(encryptKey, keyBytes, iv)
	if err != nil {
		return err
	}

	mac := crypto.Sha3(derivedKey[16:32], cipherText)

	scryptParamsJSON := make(map[string]interface{}, 5)
	scryptParamsJSON["n"] = scryptN
	scryptParamsJSON["r"] = scryptr
	scryptParamsJSON["p"] = scryptp
	scryptParamsJSON["dklen"] = scryptdkLen
	scryptParamsJSON["salt"] = hex.EncodeToString(salt)

	cipherParamsJSON := cipherparamsJSON{
		IV: hex.EncodeToString(iv),
	}

	cryptoStruct := cryptoJSON{
		Cipher:       "aes-128-ctr",
		CipherText:   hex.EncodeToString(cipherText),
		CipherParams: cipherParamsJSON,
		KDF:          "scrypt",
		KDFParams:    scryptParamsJSON,
		MAC:          hex.EncodeToString(mac),
	}
	encryptedKeyJSONV3 := encryptedKeyJSONV3{
		hex.EncodeToString(key.Address[:]),
		cryptoStruct,
		key.Id.String(),
		version,
	}
	keyJSON, err := json.Marshal(encryptedKeyJSONV3)
	if err != nil {
		return err
	}

	return writeKeyFile(key.Address, ks.keysDirPath, keyJSON)
}

func (ks keyStorePassphrase) DeleteKey(keyAddr common.Address, auth string) (err error) {
	// only delete if correct passphrase is given
	_, _, err = decryptKeyFromFile(ks.keysDirPath, keyAddr, auth)
	if err != nil {
		return err
	}

	return deleteKey(ks.keysDirPath, keyAddr)
}

func decryptKeyFromFile(keysDirPath string, keyAddr common.Address, auth string) (keyBytes []byte, keyId []byte, err error) {
	m := make(map[string]interface{})
	err = getKey(keysDirPath, keyAddr, &m)
	if err != nil {
		loglogrus.Log.Errorf("%v", err)
		return
	}

	v := reflect.ValueOf(m["version"])
	if v.Kind() == reflect.String && v.String() == "1" {
		k := new(encryptedKeyJSONV1)
		err = getKey(keysDirPath, keyAddr, &k)
		if err != nil {
			loglogrus.Log.Debugf("%v", err)
			return
		}
		return decryptKeyV1(k, auth)
	} else {
		k := new(encryptedKeyJSONV3)
		err = getKey(keysDirPath, keyAddr, &k)
		//k.Crypto.Cipher = "aes-128-ctr" //
		if err != nil {
			loglogrus.Log.Debugf("%v", err)
			return
		}
		return decryptKeyV3(k, auth)
	}
}

func decryptKeyV3(keyProtected *encryptedKeyJSONV3, auth string) (keyBytes []byte, keyId []byte, err error) {
	if keyProtected.Version != version {
		loglogrus.Log.Errorf("%v", err)
		return nil, nil, fmt.Errorf("Version not supported: %v", keyProtected.Version)
	}

	if keyProtected.Crypto.Cipher != "aes-128-ctr" {
		loglogrus.Log.Errorf("%v", err)
		return nil, nil, fmt.Errorf("Cipher not supported: %v", keyProtected.Crypto.Cipher)
	}
	keyId = uuid.Parse(keyProtected.Id)

	mac, err := hex.DecodeString(keyProtected.Crypto.MAC)
	if err != nil {
		loglogrus.Log.Errorf("%v", err)
		return nil, nil, err
	}
	iv, err := hex.DecodeString(keyProtected.Crypto.CipherParams.IV)
	if err != nil {
		loglogrus.Log.Errorf("%v", err)
		return nil, nil, err
	}
	cipherText, err := hex.DecodeString(keyProtected.Crypto.CipherText)
	if err != nil {
		loglogrus.Log.Errorf("%v", err)
		return nil, nil, err
	}
	derivedKey, err := getKDFKey(keyProtected.Crypto, auth)
	if err != nil {
		loglogrus.Log.Errorf("%v", err)
		return nil, nil, err
	}
	calculatedMAC := crypto.Sha3(derivedKey[16:32], cipherText)
	if !bytes.Equal(calculatedMAC, mac) {
		loglogrus.Log.Errorf("%v", err)
		return nil, nil, errors.New("Decryption failed: MAC mismatch")
	}
	plainText, err := aesCTRXOR(derivedKey[:16], cipherText, iv)
	if err != nil {
		loglogrus.Log.Errorf("%v", err)
		return nil, nil, err
	}
	return plainText, keyId, err
}

func decryptKeyV1(keyProtected *encryptedKeyJSONV1, auth string) (keyBytes []byte, keyId []byte, err error) {
	keyId = uuid.Parse(keyProtected.Id)
	mac, err := hex.DecodeString(keyProtected.Crypto.MAC)
	if err != nil {
		return nil, nil, err
	}

	iv, err := hex.DecodeString(keyProtected.Crypto.CipherParams.IV)
	if err != nil {
		return nil, nil, err
	}

	cipherText, err := hex.DecodeString(keyProtected.Crypto.CipherText)
	if err != nil {
		return nil, nil, err
	}

	derivedKey, err := getKDFKey(keyProtected.Crypto, auth)
	if err != nil {
		return nil, nil, err
	}

	calculatedMAC := crypto.Sha3(derivedKey[16:32], cipherText)
	if !bytes.Equal(calculatedMAC, mac) {
		return nil, nil, errors.New("Decryption failed: MAC mismatch")
	}

	plainText, err := aesCBCDecrypt(crypto.Sha3(derivedKey[:16])[:16], cipherText, iv)
	if err != nil {
		return nil, nil, err
	}
	return plainText, keyId, err
}

func getKDFKey(cryptoJSON cryptoJSON, auth string) ([]byte, error) {
	authArray := []byte(auth)
	salt, err := hex.DecodeString(cryptoJSON.KDFParams["salt"].(string))
	if err != nil {
		return nil, err
	}
	dkLen := ensureInt(cryptoJSON.KDFParams["dklen"])

	if cryptoJSON.KDF == "scrypt" {
		n := ensureInt(cryptoJSON.KDFParams["n"])
		r := ensureInt(cryptoJSON.KDFParams["r"])
		p := ensureInt(cryptoJSON.KDFParams["p"])
		return scrypt.Key(authArray, salt, n, r, p, dkLen)

	} else if cryptoJSON.KDF == "pbkdf2" {
		c := ensureInt(cryptoJSON.KDFParams["c"])
		prf := cryptoJSON.KDFParams["prf"].(string)
		if prf != "hmac-sha256" {
			return nil, fmt.Errorf("Unsupported PBKDF2 PRF: %s", prf)
		}
		key := pbkdf2.Key(authArray, salt, c, dkLen, sha256.New)
		return key, nil
	}

	return nil, fmt.Errorf("Unsupported KDF: %s", cryptoJSON.KDF)
}

// TODO: can we do without this when unmarshalling dynamic JSON?
// why do integers in KDF params end up as float64 and not int after
// unmarshal?
func ensureInt(x interface{}) int {
	res, ok := x.(int)
	if !ok {
		res = int(x.(float64))
	}
	return res
}
