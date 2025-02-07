package resource

import (
	"crypto/aes"
	"crypto/cipher"
	"crypto/ecdsa"
	"crypto/hmac"
	"crypto/md5"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"crypto/sha512"
	"io"
	"math/big"
)

import (
	"encoding/base64"
	"encoding/hex"
	"log"
)

// Map of frequently used encoding/decoding functions
var EncodingFuncMap = map[string]interface{}{
	"btoa":         Btoa,
	"atob":         Atob,
	"FromJson":     FromJson,
	"ToJson":       ToJson,
	"HexEncode":    HexEncode,
	"HexDecode":    HexDecode,
	"Base64Encode": Base64Encode,
	"Base64Decode": Base64Decode,
	"URLEncode":    URLEncode,
	"URLDecode":    URLDecode,
}

// Base64 encoding (similar to Btoa in JavaScript)
func Btoa(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64 encoding (similar to Btoa in JavaScript)
func FromJson(data []byte) interface{} {
	var mapIns interface{}
	mapIns = make(map[string]interface{})
	if data[0] == '[' {
		mapIns = make([]interface{}, 0)
	}
	err := json.Unmarshal(data, &mapIns)
	if err != nil {
		log.Printf("Failed to unmarshal as json [" + string(data) + "] => " + err.Error())
		return nil
	}
	return mapIns
}

// Base64 encoding (similar to Btoa in JavaScript)
func ToJson(mapIns interface{}) string {
	var data []byte

	data, err := json.Marshal(&mapIns)
	if err != nil {
		log.Printf("Failed to unmarshal as json [" + string(data) + "] => " + err.Error())
		return ""
	}
	return string(data)
}

// Base64 decoding (similar to Atob in JavaScript)
func Atob(data string) string {
	decodedData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Printf("Atob failed: %v", err)
		return ""
	}
	return string(decodedData)
}

// Hex encoding function
func HexEncode(data []byte) string {
	return hex.EncodeToString(data)
}

// Hex decoding function
func HexDecode(data string) []byte {
	decodedData, err := hex.DecodeString(data)
	if err != nil {
		log.Printf("HexDecode failed: %v", err)
		return nil
	}
	return decodedData
}

// Base64 standard encoding function
func Base64Encode(data []byte) string {
	return base64.StdEncoding.EncodeToString(data)
}

// Base64 standard decoding function
func Base64Decode(data string) []byte {
	decodedData, err := base64.StdEncoding.DecodeString(data)
	if err != nil {
		log.Printf("Base64Decode failed: %v", err)
		return nil
	}
	return decodedData
}

// Base64 URL encoding function
func URLEncode(data []byte) string {
	return base64.URLEncoding.EncodeToString(data)
}

// Base64 URL decoding function
func URLDecode(data string) []byte {
	decodedData, err := base64.URLEncoding.DecodeString(data)
	if err != nil {
		log.Printf("URLDecode failed: %v", err)
		return nil
	}
	return decodedData
}

// Map of frequently used cryptographic functions
var CryptoFuncMap = map[string]interface{}{
	"SHA256Hash":     SHA256Hash,
	"SHA512Hash":     SHA512Hash,
	"MD5Hash":        MD5Hash,
	"HMACSHA256":     HMACSHA256,
	"HMACSHA512":     HMACSHA512,
	"AESGCMEncrypt":  AESGCMEncrypt,
	"AESGCMDecrypt":  AESGCMDecrypt,
	"RSAGenerateKey": RSAGenerateKey,
	"RSAEncrypt":     RSAEncrypt,
	"RSADecrypt":     RSADecrypt,
	"ECDSASign":      ECDSASign,
	"ECDSAVerify":    ECDSAVerify,
	"Encrypt":        Encrypt,
	"Decrypt":        Decrypt,
}

// SHA256 hash function
func SHA256Hash(data []byte) []byte {
	hash := sha256.New()
	hash.Write(data)
	return hash.Sum(nil)
}

// SHA512 hash function
func SHA512Hash(data []byte) []byte {
	hash := sha512.New()
	hash.Write(data)
	return hash.Sum(nil)
}

// MD5 hash function
func MD5Hash(data []byte) []byte {
	hash := md5.New()
	hash.Write(data)
	return hash.Sum(nil)
}

// HMAC-SHA256 function
func HMACSHA256(key, data []byte) []byte {
	h := hmac.New(sha256.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// HMAC-SHA512 function
func HMACSHA512(key, data []byte) []byte {
	h := hmac.New(sha512.New, key)
	h.Write(data)
	return h.Sum(nil)
}

// AES-GCM encryption
func AESGCMEncrypt(key, plaintext []byte) (ciphertext, nonce []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, nil, err
	}
	nonce = make([]byte, aesgcm.NonceSize())
	if _, err = io.ReadFull(rand.Reader, nonce); err != nil {
		return nil, nil, err
	}
	ciphertext = aesgcm.Seal(nil, nonce, plaintext, nil)
	return ciphertext, nonce, nil
}

// AES-GCM decryption
func AESGCMDecrypt(key, nonce, ciphertext []byte) (plaintext []byte, err error) {
	block, err := aes.NewCipher(key)
	if err != nil {
		return nil, err
	}
	aesgcm, err := cipher.NewGCM(block)
	if err != nil {
		return nil, err
	}
	plaintext, err = aesgcm.Open(nil, nonce, ciphertext, nil)
	if err != nil {
		return nil, err
	}
	return plaintext, nil
}

// RSA key generation
func RSAGenerateKey(bits int) (*rsa.PrivateKey, error) {
	privateKey, err := rsa.GenerateKey(rand.Reader, bits)
	if err != nil {
		return nil, err
	}
	return privateKey, nil
}

// RSA encryption
func RSAEncrypt(pubKey *rsa.PublicKey, data []byte) ([]byte, error) {
	return rsa.EncryptOAEP(sha256.New(), rand.Reader, pubKey, data, nil)
}

// RSA decryption
func RSADecrypt(privKey *rsa.PrivateKey, ciphertext []byte) ([]byte, error) {
	return rsa.DecryptOAEP(sha256.New(), rand.Reader, privKey, ciphertext, nil)
}

// ECDSA signing
func ECDSASign(privKey *ecdsa.PrivateKey, data []byte) (r, s []byte, err error) {
	hash := sha256.Sum256(data)
	rInt, sInt, err := ecdsa.Sign(rand.Reader, privKey, hash[:])
	if err != nil {
		return nil, nil, err
	}
	return rInt.Bytes(), sInt.Bytes(), nil
}

// ECDSA verification
func ECDSAVerify(pubKey *ecdsa.PublicKey, data, r, s []byte) bool {
	hash := sha256.Sum256(data)
	rInt := new(big.Int).SetBytes(r)
	sInt := new(big.Int).SetBytes(s)
	return ecdsa.Verify(pubKey, hash[:], rInt, sInt)
}
