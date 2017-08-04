package main

import (
	validator "gopkg.in/go-playground/validator.v9"
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
