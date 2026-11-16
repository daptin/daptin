package resource

import (
	"bytes"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/emersion/go-message"
	_ "github.com/emersion/go-message/charset"
	mailheader "github.com/emersion/go-message/mail"
)

// MailSearchMetadata is the display metadata stored alongside original MIME.
// Native content searches still use the authorized original message.
type MailSearchMetadata struct {
	MessageID     string
	From          string
	To            string
	Cc            string
	Bcc           string
	Sender        string
	ReplyTo       string
	Subject       string
	Body          string
	ContentType   string
	SentDate      time.Time
	HasAttachment bool
}

func ExtractMailSearchMetadata(raw []byte) (MailSearchMetadata, error) {
	entity, err := message.Read(bytes.NewReader(raw))
	if err != nil && !MailMIMEEncodingUnsupported(err) {
		return MailSearchMetadata{}, err
	}
	return mailSearchMetadataFromEntity(entity, err)
}

// go-message still provides a readable entity for these errors, but its body
// is not decoded text. A MIME consumer must not render or index it as such.
func MailMIMEEncodingUnsupported(err error) bool {
	return message.IsUnknownCharset(err) || message.IsUnknownEncoding(err)
}

func mailSearchMetadataFromEntity(entity *message.Entity, rootErr error) (MailSearchMetadata, error) {
	header := mailheader.Header{Header: entity.Header}
	addresses := func(name string) string {
		addressList, _ := header.AddressList(name)
		values := make([]string, 0, len(addressList))
		for _, address := range addressList {
			values = append(values, address.String())
		}
		return strings.Join(values, ", ")
	}
	subject, _ := header.Subject()
	messageID, _ := header.MessageID()
	date, _ := header.Date()
	metadata := MailSearchMetadata{
		MessageID: messageID, From: addresses("From"), To: addresses("To"),
		Cc: addresses("Cc"), Bcc: addresses("Bcc"), Sender: addresses("Sender"),
		ReplyTo: addresses("Reply-To"), Subject: subject, SentDate: date, ContentType: header.Get("Content-Type"),
	}

	var text strings.Builder
	// Like mail.unpack, interpret the MIME tree rather than constraining the
	// order or media types of multipart children. Never materialize attachments
	// to produce a list flag or summary body.
	remaining := int64(64 << 20)
	err := entity.Walk(func(path []int, part *message.Entity, walkErr error) error {
		if len(path) == 0 {
			walkErr = rootErr
		}
		opaque := MailMIMEEncodingUnsupported(walkErr)
		if walkErr != nil && !opaque {
			return walkErr
		}
		if len(path) > 32 {
			return errors.New("mail metadata MIME nesting limit exceeded")
		}
		mediaType, _, err := part.Header.ContentType()
		if err != nil {
			return err
		}
		disposition, _, err := part.Header.ContentDisposition()
		if err != nil && part.Header.Get("Content-Disposition") != "" {
			return err
		}
		if part.MultipartReader() != nil {
			return nil
		}
		isText := strings.HasPrefix(mediaType, "text/")
		isAttachment := disposition == "attachment" ||
			(disposition != "inline" && !isText && strings.Trim(strings.TrimSpace(part.Header.Get("Content-ID")), "<>") == "")
		if isAttachment {
			metadata.HasAttachment = true
		}
		if opaque || isAttachment || mediaType != "text/plain" {
			return nil
		}
		data, err := io.ReadAll(io.LimitReader(part.Body, remaining+1))
		if err != nil {
			return err
		}
		remaining -= int64(len(data))
		if remaining < 0 {
			return errors.New("mail metadata decoded text limit exceeded")
		}
		if text.Len() != 0 && len(data) != 0 {
			text.WriteByte('\n')
		}
		text.Write(data)
		return nil
	})
	metadata.Body = text.String()
	return metadata, err
}
