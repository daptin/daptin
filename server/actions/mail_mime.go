package actions

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
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
	DecodedSize        int64
	SHA256             string
	Content            []byte
}

type normalizedMail struct {
	PreferredTextBody string
	HTMLBody          string
	Parts             []normalizedMailPart
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

	entity, err := message.ReadWithOptions(bytes.NewReader(messageBytes), &message.ReadOptions{MaxHeaderBytes: limits.MaxHeaderBytes})
	if err != nil {
		if strings.Contains(err.Error(), "header exceeds maximum size") {
			return nil, mailUnpackError("mail_headers_too_large", err)
		}
		return nil, mailUnpackError("mail_malformed", err)
	}

	normalized := &normalizedMail{
		Parts: make([]normalizedMailPart, 0),
	}

	var decodedTotal int64
	var headerTotal int64
	err = entity.Walk(func(path []int, part *message.Entity, walkErr error) error {
		if walkErr != nil {
			return mailUnpackError("mail_malformed", walkErr)
		}
		if len(path) > limits.MaxNestingDepth {
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
		if part.MultipartReader() != nil {
			return nil
		}
		if len(normalized.Parts) >= limits.MaxParts {
			return mailUnpackError("mail_too_many_parts", nil)
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
		disposition = strings.ToLower(strings.TrimSpace(disposition))
		contentID := strings.Trim(strings.TrimSpace(part.Header.Get("Content-ID")), "<>")
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
		if err != nil {
			return mailUnpackError("mail_malformed", err)
		}
		if int64(len(content)) > readLimit {
			return mailUnpackError(limitCode, nil)
		}
		decodedTotal += int64(len(content))
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
		}
		normalized.Parts = append(normalized.Parts, normalizedPart)
		if kind == "inline" && len(bytes.TrimSpace(content)) > 0 {
			switch mediaType {
			case "text/plain":
				if normalized.PreferredTextBody == "" {
					normalized.PreferredTextBody = string(content)
				}
			case "text/html":
				if normalized.HTMLBody == "" {
					normalized.HTMLBody = string(content)
				}
			}
		}
		return nil
	})
	if err != nil {
		var unpackErr *mailUnpackErrorDetail
		if errors.As(err, &unpackErr) {
			return nil, err
		}
		return nil, mailUnpackError("mail_malformed", err)
	}
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
