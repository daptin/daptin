package resource

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"mime"
	"net/mail"
	"strings"
	"time"
	"unicode/utf8"

	"github.com/emersion/go-message"
	mailheader "github.com/emersion/go-message/mail"
	"golang.org/x/net/html"
	"golang.org/x/net/idna"
)

type MailRecipient struct {
	Name    string `json:"name"`
	Address string `json:"address"`
}
type MailComposePart struct {
	Filename  string `json:"filename"`
	MediaType string `json:"media_type"`
	ContentID string `json:"content_id"`
	Base64    string `json:"base64"`
}

// MailComposition is a draft's editable content. Sender identity is deliberately
// absent: the operation resolves it from the authorized canonical mail account.
type MailComposition struct {
	To         []MailRecipient   `json:"to"`
	Cc         []MailRecipient   `json:"cc"`
	Bcc        []MailRecipient   `json:"bcc"`
	Subject    string            `json:"subject"`
	Text       string            `json:"text"`
	HTML       string            `json:"html"`
	InReplyTo  string            `json:"in_reply_to"`
	References []string          `json:"references"`
	Parts      []MailComposePart `json:"parts"`
}

func composeMailbox(value MailRecipient) (*mail.Address, error) {
	if strings.ContainsAny(value.Name+value.Address, "\r\n\x00") {
		return nil, errors.New("invalid address header")
	}
	parsed, err := mail.ParseAddress(strings.TrimSpace(value.Address))
	if err != nil || parsed.Name != "" {
		return nil, errors.New("enter one mailbox per recipient")
	}
	at := strings.LastIndex(parsed.Address, "@")
	if at < 1 {
		return nil, errors.New("mailbox domain required")
	}
	for _, r := range parsed.Address[:at] {
		if r > 127 {
			return nil, errors.New("SMTPUTF8 local parts are not supported")
		}
	}
	domain, err := idna.Lookup.ToASCII(parsed.Address[at+1:])
	if err != nil || domain == "" {
		return nil, errors.New("invalid mailbox domain")
	}
	return &mail.Address{Name: value.Name, Address: parsed.Address[:at] + "@" + strings.ToLower(domain)}, nil
}

func composeMessageID(value string) (string, error) {
	value = strings.TrimPrefix(strings.TrimSuffix(value, ">"), "<")
	if value == "" || strings.ContainsAny(value, "\r\n\x00 <>\t") || !strings.Contains(value, "@") {
		return "", errors.New("invalid message identifier")
	}
	for _, character := range value {
		if character < 33 || character > 126 {
			return "", errors.New("message identifiers must be ASCII")
		}
	}
	at := strings.LastIndex(value, "@")
	if at < 1 || at == len(value)-1 {
		return "", errors.New("invalid message identifier")
	}
	return value, nil
}

func composeContentID(value string) (string, error) {
	value = strings.TrimPrefix(strings.TrimSuffix(value, ">"), "<")
	if value == "" || len(value) > 500 || strings.ContainsAny(value, "<>\"\\") {
		return "", errors.New("invalid content identifier")
	}
	for _, character := range value {
		if character < 33 || character > 126 {
			return "", errors.New("invalid content identifier")
		}
	}
	return value, nil
}

// ComposeOutgoingMail never emits a Bcc header. Its returned envelope includes
// Bcc recipients separately. No filesystem paths or URL attachment fetches exist.
func ComposeOutgoingMail(sender, messageID string, date time.Time, value MailComposition) ([]byte, []string, error) {
	return composeMail(sender, messageID, date, value, false)
}
func ComposeDraftMail(sender, messageID string, date time.Time, value MailComposition) ([]byte, error) {
	raw, _, err := composeMail(sender, messageID, date, value, true)
	return raw, err
}

func composeMail(sender, messageID string, date time.Time, value MailComposition, draft bool) ([]byte, []string, error) {
	from, err := composeMailbox(MailRecipient{Address: sender})
	if err != nil {
		return nil, nil, err
	}
	id, err := composeMessageID(messageID)
	if err != nil {
		return nil, nil, err
	}
	if strings.ContainsAny(value.Subject, "\r\n\x00") || utf8.RuneCountInString(value.Subject) > 500 || len(value.References) > 100 || len(value.Parts) > 100 {
		return nil, nil, errors.New("message headers or part count exceed limits")
	}
	if len(value.Text)+len(value.HTML) > 4<<20 {
		return nil, nil, errors.New("message bodies exceed 4 MiB")
	}
	var header mailheader.Header
	header.SetAddressList("From", []*mail.Address{from})
	header.SetSubject(value.Subject)
	header.SetDate(date)
	header.SetMessageID(id)
	envelope := []string{}
	seen := map[string]bool{}
	for _, field := range []struct {
		name       string
		recipients []MailRecipient
	}{{"To", value.To}, {"Cc", value.Cc}, {"Bcc", value.Bcc}} {
		addresses := []*mail.Address{}
		for _, recipient := range field.recipients {
			address, err := composeMailbox(recipient)
			if err != nil {
				return nil, nil, err
			}
			addresses = append(addresses, address)
			if !seen[address.Address] {
				seen[address.Address] = true
				envelope = append(envelope, address.Address)
			}
		}
		if len(addresses) > 0 && (field.name != "Bcc" || draft) {
			header.SetAddressList(field.name, addresses)
		}
	}
	if len(value.To)+len(value.Cc)+len(value.Bcc) > 100 {
		return nil, nil, errors.New("at most 100 recipients are supported")
	}
	if value.InReplyTo != "" {
		id, err := composeMessageID(value.InReplyTo)
		if err != nil {
			return nil, nil, err
		}
		header.SetMsgIDList("In-Reply-To", []string{id})
	}
	references := []string{}
	for _, value := range value.References {
		id, err := composeMessageID(value)
		if err != nil {
			return nil, nil, err
		}
		references = append(references, id)
	}
	if len(references) > 0 {
		header.SetMsgIDList("References", references)
	}
	decoded := make([][]byte, len(value.Parts))
	cids := map[string]bool{}
	total := len(value.Text) + len(value.HTML)
	for i, part := range value.Parts {
		if part.MediaType != "" {
			mediaType, params, err := mime.ParseMediaType(part.MediaType)
			if err != nil || len(params) != 0 || strings.HasPrefix(mediaType, "multipart/") || !strings.Contains(mediaType, "/") {
				return nil, nil, errors.New("invalid attachment media type")
			}
		}
		if strings.ContainsAny(part.Filename+part.MediaType+part.ContentID, "\r\n\x00") || len(part.Filename) > 500 || len(part.Base64) > 24<<20 {
			return nil, nil, errors.New("invalid attachment metadata or size")
		}
		data, err := base64.StdEncoding.Strict().DecodeString(part.Base64)
		if err != nil || len(data) > 16<<20 {
			return nil, nil, errors.New("invalid attachment encoding or size")
		}
		decoded[i] = data
		total += len(data)
		if total > 20<<20 {
			return nil, nil, errors.New("decoded message exceeds 20 MiB")
		}
		if part.ContentID != "" {
			cid, err := composeContentID(part.ContentID)
			if err != nil || cids[cid] || len(data) > 5<<20 {
				return nil, nil, errors.New("invalid, duplicate or oversized inline image")
			}
			if part.MediaType != "image/png" && part.MediaType != "image/jpeg" && part.MediaType != "image/gif" && part.MediaType != "image/webp" {
				return nil, nil, errors.New("inline parts must be raster images")
			}
			cids[cid] = true
		}
	}
	if value.HTML != "" {
		root, err := html.Parse(strings.NewReader(value.HTML))
		if err != nil {
			return nil, nil, err
		}
		var check func(*html.Node) error
		check = func(node *html.Node) error {
			if node.Type == html.ElementNode && node.Data == "img" {
				for _, attr := range node.Attr {
					if attr.Key == "src" {
						lower := strings.ToLower(attr.Val)
						if strings.HasPrefix(lower, "blob:") || strings.HasPrefix(lower, "data:") {
							return errors.New("persist inline images as CID parts before saving")
						}
						if strings.HasPrefix(lower, "cid:") && !cids[attr.Val[4:]] {
							return errors.New("inline image has no matching MIME part")
						}
					}
				}
			}
			for child := node.FirstChild; child != nil; child = child.NextSibling {
				if err := check(child); err != nil {
					return err
				}
			}
			return nil
		}
		if err = check(root); err != nil {
			return nil, nil, err
		}
	}
	header.SetContentType("multipart/mixed", nil)
	var output bytes.Buffer
	writer, err := message.CreateWriter(&output, header.Header)
	if err != nil {
		return nil, nil, err
	}
	var alternativeHeader message.Header
	alternativeHeader.SetContentType("multipart/alternative", nil)
	alternative, err := writer.CreatePart(alternativeHeader)
	if err != nil {
		return nil, nil, err
	}
	writePart := func(parent *message.Writer, mediaType string, data []byte, part *MailComposePart) error {
		var h message.Header
		params := map[string]string{}
		if strings.HasPrefix(mediaType, "text/") {
			params["charset"] = "utf-8"
		}
		h.SetContentType(mediaType, params)
		h.Set("Content-Transfer-Encoding", "base64")
		if part != nil {
			disposition := "attachment"
			if part.ContentID != "" {
				disposition = "inline"
				h.Set("Content-ID", "<"+strings.Trim(part.ContentID, "<>")+">")
			}
			h.SetContentDisposition(disposition, map[string]string{"filename": part.Filename})
		}
		child, err := parent.CreatePart(h)
		if err != nil {
			return err
		}
		if _, err = child.Write(data); err != nil {
			return err
		}
		return child.Close()
	}
	if err = writePart(alternative, "text/plain", []byte(value.Text), nil); err != nil {
		return nil, nil, err
	}
	if value.HTML != "" {
		var relatedHeader message.Header
		relatedHeader.SetContentType("multipart/related", nil)
		related, err := alternative.CreatePart(relatedHeader)
		if err != nil {
			return nil, nil, err
		}
		if err = writePart(related, "text/html", []byte(value.HTML), nil); err != nil {
			return nil, nil, err
		}
		for i, part := range value.Parts {
			if part.ContentID != "" {
				if err = writePart(related, part.MediaType, decoded[i], &part); err != nil {
					return nil, nil, err
				}
			}
		}
		if err = related.Close(); err != nil {
			return nil, nil, err
		}
	} else if len(cids) > 0 {
		return nil, nil, errors.New("inline images require an HTML body")
	}
	if err = alternative.Close(); err != nil {
		return nil, nil, err
	}
	for i, part := range value.Parts {
		if part.ContentID == "" {
			mediaType := part.MediaType
			if mediaType == "" {
				mediaType = "application/octet-stream"
			}
			if err = writePart(writer, mediaType, decoded[i], &part); err != nil {
				return nil, nil, err
			}
		}
	}
	if err = writer.Close(); err != nil {
		return nil, nil, err
	}
	if output.Len() > 25<<20 {
		return nil, nil, fmt.Errorf("encoded message exceeds 25 MiB")
	}
	return output.Bytes(), envelope, nil
}
