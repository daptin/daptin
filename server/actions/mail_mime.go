package actions

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/daptin/daptin/server/resource"
	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	mailheader "github.com/emersion/go-message/mail"
)

const (
	mailMaxRawBytes     int64 = 25 << 20
	mailMaxDecodedBytes int64 = 32 << 20
	mailMaxPartBytes    int64 = 16 << 20
	mailMaxHeaderBytes  int64 = 1 << 20
	mailMaxParts              = 256
	mailMaxNestingDepth       = 20
)

type mailUnpackLimitsConfig struct {
	MaxRawBytes     int64
	MaxDecodedBytes int64
	MaxPartBytes    int64
	MaxHeaderBytes  int64
	MaxParts        int
	MaxNestingDepth int
}

type normalizedMailPart struct {
	Index              int
	Kind               string
	MediaType          string
	Filename           string
	ContentID          string
	ContentDisposition string
	Presentation       string
	DecodedSize        int64
	SHA256             string
	Content            []byte
}

type normalizedMail struct {
	PreferredTextBody string
	HTMLBody          string
	Parts             []normalizedMailPart
	Envelope          map[string]interface{}
}

type mailUnpackErrorDetail struct {
	Code string
	Err  error
}

func (e *mailUnpackErrorDetail) Error() string {
	return e.Code
}

func (e *mailUnpackErrorDetail) Unwrap() error {
	return e.Err
}

func defaultMailUnpackLimits() mailUnpackLimitsConfig {
	return mailUnpackLimitsConfig{
		MaxRawBytes:     mailMaxRawBytes,
		MaxDecodedBytes: mailMaxDecodedBytes,
		MaxPartBytes:    mailMaxPartBytes,
		MaxHeaderBytes:  mailMaxHeaderBytes,
		MaxParts:        mailMaxParts,
		MaxNestingDepth: mailMaxNestingDepth,
	}
}

func normalizeMail(messageBytes []byte, limits mailUnpackLimitsConfig) (*normalizedMail, error) {
	if int64(len(messageBytes)) > limits.MaxRawBytes {
		return nil, mailUnpackError("mail_raw_too_large", nil)
	}
	if rootMailHeaderBytes(messageBytes) > limits.MaxHeaderBytes {
		return nil, mailUnpackError("mail_headers_too_large", nil)
	}

	entity, rootErr := message.ReadWithOptions(bytes.NewReader(messageBytes), &message.ReadOptions{MaxHeaderBytes: limits.MaxHeaderBytes})
	if rootErr != nil && !resource.MailMIMEEncodingUnsupported(rootErr) {
		if strings.Contains(rootErr.Error(), "header exceeds maximum size") {
			return nil, mailUnpackError("mail_headers_too_large", rootErr)
		}
		return nil, mailUnpackError("mail_malformed", rootErr)
	}

	normalized := &normalizedMail{
		Parts: make([]normalizedMailPart, 0),
	}
	header := mailheader.Header{Header: entity.Header}
	warnings := []string{}
	addresses := func(name string) []map[string]string {
		result := []map[string]string{}
		values, err := header.AddressList(name)
		if err != nil {
			warnings = append(warnings, name)
			return result
		}
		for _, value := range values {
			result = append(result, map[string]string{"name": value.Name, "address": value.Address})
		}
		return result
	}
	subject, _ := header.Subject()
	messageID, _ := header.MessageID()
	reply, _ := header.MsgIDList("In-Reply-To")
	references, _ := header.MsgIDList("References")
	if references == nil {
		references = []string{}
	}
	inReplyTo := ""
	if len(reply) > 0 {
		inReplyTo = reply[0]
	}
	normalized.Envelope = map[string]interface{}{"subject": subject, "message_id": messageID, "in_reply_to": inReplyTo, "references": references,
		"from": addresses("From"), "to": addresses("To"), "cc": addresses("Cc"), "bcc": addresses("Bcc"), "reply_to": addresses("Reply-To")}
	normalized.Envelope["warnings"] = warnings

	var decodedTotal int64
	var headerTotal int64
	var tree *mailMIMEPart
	var ancestry [mailMaxNestingDepth + 1]*mailMIMEPart
	err := entity.Walk(func(path []int, part *message.Entity, walkErr error) error {
		if len(path) == 0 {
			walkErr = rootErr
		}
		opaque := resource.MailMIMEEncodingUnsupported(walkErr)
		if walkErr != nil && !opaque {
			return mailUnpackError("mail_malformed", walkErr)
		}
		if len(path) > limits.MaxNestingDepth || len(path) >= len(ancestry) {
			return mailUnpackError("mail_nesting_too_deep", nil)
		}
		partHeaderBytes, err := normalizedMailHeaderBytes(&part.Header)
		if err != nil {
			return mailUnpackError("mail_malformed", err)
		}
		headerTotal += partHeaderBytes
		if headerTotal > limits.MaxHeaderBytes {
			return mailUnpackError("mail_headers_too_large", nil)
		}
		mediaType, contentTypeParams, err := part.Header.ContentType()
		if err != nil {
			return mailUnpackError("mail_malformed", err)
		}
		disposition, dispositionParams, err := part.Header.ContentDisposition()
		if err != nil && strings.TrimSpace(part.Header.Get("Content-Disposition")) != "" {
			return mailUnpackError("mail_malformed", err)
		}
		mediaType = strings.ToLower(strings.TrimSpace(mediaType))
		if opaque {
			mediaType = "application/octet-stream"
		}
		disposition = strings.ToLower(strings.TrimSpace(disposition))
		contentID := strings.Trim(strings.TrimSpace(part.Header.Get("Content-ID")), "<>")
		node := &mailMIMEPart{media: mediaType, cid: contentID, start: contentTypeParams["start"], rootType: contentTypeParams["type"], attached: disposition == "attachment", opaque: opaque, index: -1}
		if depth := len(path); depth == 0 {
			tree = node
		} else {
			node.parent = ancestry[depth-1]
			node.parent.children = append(node.parent.children, node)
		}
		ancestry[len(path)] = node
		if part.MultipartReader() != nil {
			return nil
		}
		if len(normalized.Parts) >= limits.MaxParts {
			return mailUnpackError("mail_too_many_parts", nil)
		}
		kind := "attachment"
		if disposition == "inline" || (disposition != "attachment" && (strings.HasPrefix(mediaType, "text/") || contentID != "")) {
			kind = "inline"
		}
		filename := dispositionParams["filename"]
		if filename == "" {
			filename = contentTypeParams["name"]
		}

		remainingTotal := limits.MaxDecodedBytes - decodedTotal
		readLimit := limits.MaxPartBytes
		limitCode := "mail_part_too_large"
		if remainingTotal < readLimit {
			readLimit = remainingTotal
			limitCode = "mail_decoded_too_large"
		}
		content, err := io.ReadAll(io.LimitReader(part.Body, readLimit+1))
		if int64(len(content)) > readLimit {
			return mailUnpackError(limitCode, nil)
		}
		decodedTotal += int64(len(content))
		presentation := "unselected"
		if resource.IsDamagedMailBase64(err) {
			// Keep the original part index, but never publish a decoded prefix.
			node.opaque = true
			content = nil
			presentation = "damaged"
		} else if err != nil {
			return mailUnpackError("mail_malformed", err)
		}
		digest := sha256.Sum256(content)
		normalizedPart := normalizedMailPart{
			Index:              len(normalized.Parts),
			Kind:               kind,
			MediaType:          mediaType,
			Filename:           filename,
			ContentID:          contentID,
			ContentDisposition: disposition,
			DecodedSize:        int64(len(content)),
			SHA256:             hex.EncodeToString(digest[:]),
			Content:            content,
			Presentation:       presentation,
		}
		node.index = normalizedPart.Index
		normalized.Parts = append(normalized.Parts, normalizedPart)
		return nil
	})
	if err != nil {
		var unpackErr *mailUnpackErrorDetail
		if errors.As(err, &unpackErr) {
			return nil, err
		}
		return nil, mailUnpackError("mail_malformed", err)
	}
	normalized.selectPresentation(tree)
	return normalized, nil
}

func rootMailHeaderBytes(messageBytes []byte) int64 {
	if index := bytes.Index(messageBytes, []byte("\r\n\r\n")); index >= 0 {
		return int64(index + 2)
	}
	if index := bytes.Index(messageBytes, []byte("\n\n")); index >= 0 {
		return int64(index + 1)
	}
	return int64(len(messageBytes))
}

func normalizedMailHeaderBytes(header *message.Header) (int64, error) {
	var total int64
	fields := header.Fields()
	for fields.Next() {
		raw, err := fields.Raw()
		if err != nil {
			return 0, err
		}
		total += int64(len(raw))
	}
	return total, nil
}

func mailUnpackError(code string, err error) error {
	if err == nil {
		err = fmt.Errorf("%s", code)
	}
	return &mailUnpackErrorDetail{Code: code, Err: err}
}
