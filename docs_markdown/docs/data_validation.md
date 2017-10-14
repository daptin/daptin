# Data validation

Daptin uses the excellent [go-playground/validator](https://github.com/go-playground/validator) library to provide extensive validations when creating and updating data.

It gives us the following unique features:

- Cross Field and Cross Struct validations by using validation tags or custom validators.
- Slice, Array and Map diving, which allows any or all levels of a multidimensional field to be validated.

