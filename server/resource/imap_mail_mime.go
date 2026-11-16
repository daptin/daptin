package resource

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/backend/backendutil"
	"github.com/emersion/go-message"
	"github.com/emersion/go-message/textproto"
)

// FETCH operates on the transfer-encoded bytes. Reuse the MIME package's raw
// multipart reader and Daptin's content type defaults without decoding bodies.
type imapMIMEPart struct {
	header textproto.Header
	body   io.Reader
	media  string
	params map[string]string
}

func newIMAPMIMEPart(header textproto.Header, body io.Reader, parent string) imapMIMEPart {
	content := message.Header{Header: header}
	media, params, err := MailPartContentType(&content, parent)
	if err != nil {
		// A malformed Content-Type must not prevent retrieval of the original
		// bytes. Treat it as an opaque leaf for section traversal.
		media, params = "", nil
	}
	return imapMIMEPart{header, body, strings.ToLower(media), params}
}

func readIMAPMIMEMessage(body io.Reader) (imapMIMEPart, error) {
	reader := bufio.NewReader(body)
	header, err := textproto.ReadHeader(reader)
	if err != nil {
		return imapMIMEPart{}, err
	}
	return newIMAPMIMEPart(header, reader, ""), nil
}

func (part imapMIMEPart) multipart() bool { return strings.HasPrefix(part.media, "multipart/") }

func (part imapMIMEPart) bodyStructure(extended bool, depth int) (*imap.BodyStructure, error) {
	if depth > 32 {
		return nil, errors.New("IMAP FETCH MIME nesting limit exceeded")
	}
	if !part.multipart() && part.media != "message/rfc822" {
		return backendutil.FetchBodyStructure(part.header, part.body, extended)
	}
	typ, sub, _ := strings.Cut(part.media, "/")
	structure := &imap.BodyStructure{MIMEType: typ, MIMESubType: sub, Params: part.params, Extended: extended,
		Id: part.header.Get("Content-ID"), Description: part.header.Get("Content-Description"), Encoding: part.header.Get("Content-Transfer-Encoding")}
	if extended {
		header := message.Header{Header: part.header}
		structure.Disposition, structure.DispositionParams, _ = header.ContentDisposition()
	}
	if part.multipart() {
		reader := textproto.NewMultipartReader(part.body, part.params["boundary"])
		for {
			raw, err := reader.NextPart()
			if err == io.EOF {
				return structure, nil
			}
			if err != nil {
				return nil, err
			}
			child := newIMAPMIMEPart(raw.Header, raw, part.media)
			nested, err := child.bodyStructure(extended, depth+1)
			if err != nil {
				return nil, err
			}
			structure.Parts = append(structure.Parts, nested)
		}
	}
	if structure.Encoding == "" {
		structure.Encoding = "7bit"
	}
	var count imapMIMECount
	message, err := readIMAPMIMEMessage(io.TeeReader(part.body, &count))
	if err != nil {
		return nil, err
	}
	structure.Envelope, err = backendutil.FetchEnvelope(message.header)
	if err != nil {
		return nil, err
	}
	structure.BodyStructure, err = message.bodyStructure(extended, depth+1)
	if err != nil {
		return nil, err
	}
	// Multipart parsing can stop at the closing delimiter. The octet and line
	// counts also include any remaining epilogue in the encapsulated message.
	if _, err := io.Copy(io.Discard, message.body); err != nil {
		return nil, err
	}
	structure.Size, structure.Lines = count.size, count.lines
	if count.size != 0 && count.last != '\n' {
		structure.Lines++
	}
	return structure, nil
}

type imapMIMECount struct {
	size, lines uint32
	last        byte
}

func (count *imapMIMECount) Write(p []byte) (int, error) {
	count.size += uint32(len(p))
	count.lines += uint32(bytes.Count(p, []byte{'\n'}))
	if len(p) != 0 {
		count.last = p[len(p)-1]
	}
	return len(p), nil
}

var errIMAPNoSuchPart = errors.New("IMAP FETCH has no such MIME section")

func (part imapMIMEPart) numberedChild(number int, messageRoot bool) (imapMIMEPart, error) {
	if number < 1 || (!part.multipart() && (!messageRoot || number != 1)) {
		return imapMIMEPart{}, errIMAPNoSuchPart
	}
	if !part.multipart() {
		return part, nil
	}
	reader := textproto.NewMultipartReader(part.body, part.params["boundary"])
	for current := 1; ; current++ {
		raw, err := reader.NextPart()
		if err == io.EOF {
			return imapMIMEPart{}, errIMAPNoSuchPart
		}
		if err != nil {
			return imapMIMEPart{}, err
		}
		if current == number {
			return newIMAPMIMEPart(raw.Header, raw, part.media), nil
		}
	}
}

func fetchIMAPBodySection(raw []byte, section *imap.BodySectionName) (imap.Literal, error) {
	if len(section.Path) > 32 {
		return nil, errIMAPNoSuchPart
	}
	if len(section.Partial) != 0 {
		if len(section.Partial) != 2 || section.Partial[0] < 0 || section.Partial[1] <= 0 || section.Partial[0]+section.Partial[1] < 0 {
			return nil, errors.New("invalid IMAP FETCH partial range")
		}
	}
	// The empty section is the stored message itself (also FETCH RFC822).
	// Parsing and rewriting its headers would change the octet offsets and
	// prevent access to original messages whose headers cannot be parsed.
	if len(section.Path) == 0 && section.Specifier == imap.EntireSpecifier {
		return bytes.NewReader(section.ExtractPartial(raw)), nil
	}
	part, err := readIMAPMIMEMessage(bytes.NewReader(raw))
	if err != nil {
		return nil, err
	}
	messageRoot := true
	for index, number := range section.Path {
		if index != 0 && part.media == "message/rfc822" {
			part, err = readIMAPMIMEMessage(part.body)
			if err != nil {
				return nil, err
			}
			messageRoot = true
		}
		part, err = part.numberedChild(number, messageRoot)
		if err != nil {
			return nil, err
		}
		messageRoot = false
	}
	resolved := *section
	resolved.Path = nil
	if len(section.Path) != 0 {
		switch section.Specifier {
		case imap.EntireSpecifier:
			resolved.Specifier = imap.TextSpecifier
		case imap.MIMESpecifier:
			// Read the containing MIME part's stored headers, including an
			// absent Content-Type on an implicit digest entry.
		case imap.TextSpecifier, imap.HeaderSpecifier:
			if part.media != "message/rfc822" {
				return nil, errIMAPNoSuchPart
			}
			part, err = readIMAPMIMEMessage(part.body)
			if err != nil {
				return nil, err
			}
		}
	} else if section.Specifier == imap.MIMESpecifier {
		return nil, errIMAPNoSuchPart
	}
	return backendutil.FetchBodySection(part.header, part.body, &resolved)
}
