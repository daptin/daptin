package main

import (
	"github.com/go-playground/validator"
	"fmt"
)

func main() {
	validate := validator.New()
	//s1 := "abcd"
	data := map[string]interface{}{
		"password":        "hello",
		"confirmPassword": []string{
			"hello1",
		},
	}
	fmt.Printf("%v", validate.VarWithValue(data["password"], data, "eqfield=InnerStructField[confirmPassword]"))
}
