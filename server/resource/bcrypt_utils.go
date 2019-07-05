package resource

import (
	"crypto/hmac"
	"crypto/md5"
	"fmt"
	"golang.org/x/crypto/bcrypt"
)

func BcryptCheckStringHash(newString, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(newString))
	return err == nil
}

func HmacCheckStringHash(userProvidedPassword, challenge, userPassword string) bool {

	h := hmac.New(md5.New, []byte(userPassword))
	h.Write([]byte(challenge))
	hash := fmt.Sprintf("%x", h.Sum(nil))

	return hash == userProvidedPassword

}

func BcryptHashString(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 11)
	return string(bytes), err
}
