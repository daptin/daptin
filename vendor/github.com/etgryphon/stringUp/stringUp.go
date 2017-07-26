package stringUp

import (
	"regexp"
	"bytes"
)


var camelingRegex = regexp.MustCompile("[0-9A-Za-z]+")

func CamelCase(src string)(string){
  byteSrc := []byte(src)
  chunks := camelingRegex.FindAll(byteSrc, -1)
  for idx, val := range chunks {
    if idx > 0 { chunks[idx] = bytes.Title(val) }
  }
  return string(bytes.Join(chunks, nil)) 
}
