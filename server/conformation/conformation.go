// Package conformation applies the ordered string transforms defined by Daptin schemas.
// The transform pipeline is adapted from github.com/artpar/conform at 3c5c27338a81.
// See NOTICE-conform at the repository root for its MIT license.
package conformation

import (
	"bytes"
	"html/template"
	"regexp"
	"strings"
	"unicode"
	"unicode/utf8"
)

var patterns = map[string]*regexp.Regexp{
	"numbers":    regexp.MustCompile("[0-9]"),
	"nonNumbers": regexp.MustCompile("[^0-9]"),
	"alpha":      regexp.MustCompile("[\\pL]"),
	"nonAlpha":   regexp.MustCompile("[^\\pL]"),
	"name":       regexp.MustCompile("[\\p{L}]([\\p{L}|[:space:]|\\-|\\']*[\\p{L}])*"),
}

// a valid email will only have one "@", but let's treat the last "@" as the domain part separator
func emailLocalPart(s string) string {
	i := strings.LastIndex(s, "@")
	if i == -1 {
		return s
	}
	return s[0:i]
}

func emailDomainPart(s string) string {
	i := strings.LastIndex(s, "@")
	if i == -1 {
		return ""
	}
	return s[i+1:]
}

func email(s string) string {
	// According to rfc5321, "The local-part of a mailbox MUST BE treated as case sensitive"
	return emailLocalPart(s) + "@" + strings.ToLower(emailDomainPart(s))
}

func camelTo(s, sep string) string {
	s = camel(s)
	var result string
	var words []string
	var lastPos int
	// camel always returns ASCII; offsets below mark byte boundaries.

	for i := 0; i < len(s); i++ {
		if i > 0 && s[i] >= 'A' && s[i] <= 'Z' {
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

// Keep the order: removing punctuation can expose repeated separators.
var nameReplaces = []struct {
	pattern     *regexp.Regexp
	replacement string
}{
	{regexp.MustCompile("[^\\pL-\\s']"), ""},
	{regexp.MustCompile("\\s{2,}"), " "},
	{regexp.MustCompile("-{2,}"), "-"},
	{regexp.MustCompile("'{2,}"), "'"},
	{regexp.MustCompile("( )*-( )*"), "-"},
}

func formatName(s string) string {
	s = strings.ToLower(s)
	for _, rule := range nameReplaces {
		s = rule.pattern.ReplaceAllLiteralString(s, rule.replacement)
	}
	return strings.Title(patterns["name"].FindString(s))
}

// camel retains ASCII letters and digits and capitalizes the first letter
// after a separator if a word has already been written. A digit itself can
// start a word, so "item 2fa" becomes "item2fa".
func camel(s string) string {
	var out strings.Builder
	afterSeparator := false
	for i := 0; i < len(s); i++ {
		c := s[i]
		switch {
		case c >= 'a' && c <= 'z':
			if afterSeparator && out.Len() != 0 {
				c -= 'a' - 'A'
			}
		case c >= 'A' && c <= 'Z', c >= '0' && c <= '9':
		default:
			afterSeparator = true
			continue
		}
		out.WriteByte(c)
		afterSeparator = false
	}
	return out.String()
}

// TransformString applies supported comma-separated tags in order.
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
			input = camel(input)
		case "snake":
			input = camelTo(input, "_")
		case "slug":
			input = camelTo(input, "-")
		case "ucfirst":
			input = ucFirst(input)
		case "name":
			input = formatName(input)
		case "email":
			input = email(strings.TrimSpace(input))
		case "num":
			input = onlyNumbers(input)
		case "!num":
			input = stripNumbers(input)
		case "alpha":
			input = onlyAlpha(input)
		case "!alpha":
			input = stripAlpha(input)
		case "!html":
			input = template.HTMLEscapeString(input)
		case "!js":
			input = template.JSEscapeString(input)
		}
	}
	return input
}
