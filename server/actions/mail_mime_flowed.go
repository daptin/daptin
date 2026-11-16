package actions

import "strings"

// RFC 3676 formatting applies only to the selected plain-text presentation.
// MIME leaf bytes and their hashes still describe the source's physical lines.
func unwrapFlowedMailText(text string, delSp bool) string {
	type physicalLine struct {
		quote, content           string
		flowed, signature, ended bool
	}
	nextLine := func(input string) (physicalLine, string) {
		line, rest, ended := strings.Cut(input, "\n")
		if ended {
			line = strings.TrimSuffix(line, "\r")
		}
		depth := 0
		for depth < len(line) && line[depth] == '>' {
			depth++
		}
		// Count quote marks before removing one space of transport stuffing.
		content := strings.TrimPrefix(line[depth:], " ")
		signature := content == "-- "
		return physicalLine{line[:depth], content, !signature && strings.HasSuffix(content, " "), signature, ended}, rest
	}
	var result strings.Builder
	result.Grow(len(text))
	continued := false
	line, rest := nextLine(text)
	for {
		if !continued && line.quote != "" {
			result.WriteString(line.quote)
			if line.content != "" {
				result.WriteByte(' ')
			}
		}
		content := line.content
		if line.ended && line.flowed && delSp {
			content = strings.TrimSuffix(content, " ")
		}
		result.WriteString(content)
		if !line.ended {
			break
		}
		next, remaining := nextLine(rest)
		// Quote depth, a signature separator, and EOF end the paragraph.
		continued = rest != "" && line.flowed && !next.signature && line.quote == next.quote
		if !continued {
			result.WriteByte('\n')
		}
		line, rest = next, remaining
	}
	return result.String()
}
