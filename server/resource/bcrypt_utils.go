package resource

import (
	"golang.org/x/crypto/bcrypt"
)

func BcryptCheckStringHash(newString, hash string) bool {
	err := bcrypt.CompareHashAndPassword([]byte(hash), []byte(newString))
	return err == nil
}


func BcryptHashString(password string) (string, error) {
	bytes, err := bcrypt.GenerateFromPassword([]byte(password), 11)
	return string(bytes), err
}
