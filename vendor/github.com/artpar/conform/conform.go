package conform

import (
	"bytes"
	"errors"
	"fmt"
	"reflect"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/etgryphon/stringUp"
)

type x map[string]string

var patterns = map[string]*regexp.Regexp{
	"numbers":    regexp.MustCompile("[0-9]"),
	"nonNumbers": regexp.MustCompile("[^0-9]"),
	"alpha":      regexp.MustCompile("[\\pL]"),
	"nonAlpha":   regexp.MustCompile("[^\\pL]"),
	"name":       regexp.MustCompile("[\\p{L}]([\\p{L}|[:space:]|-]*[\\p{L}])*"),
}

func camelTo(s, sep string) string {
	var result string
	var words []string
	var lastPos int
	rs := []rune(s)

	for i := 0; i < len(rs); i++ {
		if i > 0 && unicode.IsUpper(rs[i]) {
			if initialism := startsWithInitialism(s[lastPos:]); initialism != "" {
				words = append(words, initialism)

				i += len(initialism) - 1
				lastPos = i
				continue
			}

			words = append(words, s[lastPos:i])
			lastPos = i
		}
	}

	// append the last word
	if s[lastPos:] != "" {
		words = append(words, s[lastPos:])
	}

	for k, word := range words {
		if k > 0 {
			result += sep
		}

		result += strings.ToLower(word)
	}

	return result
}

// startsWithInitialism returns the initialism if the given string begins with it
func startsWithInitialism(s string) string {
	var initialism string
	// the longest initialism is 5 char, the shortest 2
	for i := 1; i <= 5; i++ {
		if len(s) > i-1 && commonInitialisms[s[:i]] {
			initialism = s[:i]
		}
	}
	return initialism
}

// commonInitialisms, taken from
// https://github.com/golang/lint/blob/3d26dc39376c307203d3a221bada26816b3073cf/lint.go#L482
var commonInitialisms = map[string]bool{
	"API":   true,
	"ASCII": true,
	"CPU":   true,
	"CSS":   true,
	"DNS":   true,
	"EOF":   true,
	"GUID":  true,
	"HTML":  true,
	"HTTP":  true,
	"HTTPS": true,
	"ID":    true,
	"IP":    true,
	"JSON":  true,
	"LHS":   true,
	"QPS":   true,
	"RAM":   true,
	"RHS":   true,
	"RPC":   true,
	"SLA":   true,
	"SMTP":  true,
	"SSH":   true,
	"TLS":   true,
	"TTL":   true,
	"UI":    true,
	"UID":   true,
	"UUID":  true,
	"URI":   true,
	"URL":   true,
	"UTF8":  true,
	"VM":    true,
	"XML":   true,
}

func ucFirst(s string) string {
	if s == "" {
		return s
	}
	toRune, size := utf8.DecodeRuneInString(s)
	if !unicode.IsLower(toRune) {
		return s
	}
	buf := &bytes.Buffer{}
	buf.WriteRune(unicode.ToUpper(toRune))
	buf.WriteString(s[size:])
	return buf.String()
}

func onlyNumbers(s string) string {
	return patterns["nonNumbers"].ReplaceAllLiteralString(s, "")
}

func stripNumbers(s string) string {
	return patterns["numbers"].ReplaceAllLiteralString(s, "")
}

func onlyAlpha(s string) string {
	return patterns["nonAlpha"].ReplaceAllLiteralString(s, "")
}

func stripAlpha(s string) string {
	return patterns["alpha"].ReplaceAllLiteralString(s, "")
}

func onlyOne(s string, m []x) string {
	for _, v := range m {
		for f, r := range v {
			s = regexp.MustCompile(fmt.Sprintf("%s{2,}", f)).ReplaceAllLiteralString(s, r)
		}
	}
	return s
}

func formatName(s string) string {
	first := onlyOne(strings.ToLower(s), []x{x{"[^\\pL-\\s]": ""}, x{"\\s": " "}, x{"-": "-"}})
	return strings.Title(patterns["name"].FindString(first))
}

// Strings conforms strings based on reflection tags
func Strings(iface interface{}) error {
	ifv := reflect.ValueOf(iface)
	if ifv.Kind() != reflect.Ptr {
		return errors.New("Not a pointer")
	}
	ift := reflect.Indirect(ifv).Type()
	for i := 0; i < ift.NumField(); i++ {
		v := ift.Field(i)
		el := reflect.Indirect(ifv.Elem().FieldByName(v.Name))
		switch el.Kind() {
		case reflect.Slice:
			if el.CanInterface() {
				if slice, ok := el.Interface().([]string); ok {
					for i, input := range slice {
						tags := v.Tag.Get("conform")
						slice[i] = TransformString(input, tags)
					}
					return nil
				} else {
					val := reflect.ValueOf(el.Interface())
					for i := 0; i < val.Len(); i++ {
						Strings(val.Index(i).Addr().Interface())
					}
					return nil
				}
			}
		case reflect.Struct:
			if el.CanAddr() && el.Addr().CanInterface() {
				Strings(el.Addr().Interface())
			}
		case reflect.String:
			if el.CanSet() {
				tags := v.Tag.Get("conform")
				input := el.String()
				el.SetString(TransformString(input, tags))
			}
		}
	}
	return nil
}

func TransformString(input, tags string) string {
	if tags == "" {
		return input
	}
	for _, split := range strings.Split(tags, ",") {
		switch split {
		case "trim":
			input = strings.TrimSpace(input)
		case "ltrim":
			input = strings.TrimLeft(input, " ")
		case "rtrim":
			input = strings.TrimRight(input, " ")
		case "lower":
			input = strings.ToLower(input)
		case "upper":
			input = strings.ToUpper(input)
		case "title":
			input = strings.Title(input)
		case "camel":
			input = stringUp.CamelCase(input)
		case "snake":
			input = camelTo(stringUp.CamelCase(input), "_")
		case "slug":
			input = camelTo(stringUp.CamelCase(input), "-")
		case "ucfirst":
			input = ucFirst(input)
		case "name":
			input = formatName(input)
		case "email":
			input = strings.ToLower(strings.TrimSpace(input))
		case "num":
			input = onlyNumbers(input)
		case "!num":
			input = stripNumbers(input)
		case "alpha":
			input = onlyAlpha(input)
		case "!alpha":
			input = stripAlpha(input)
		}
	}
	return input
}
