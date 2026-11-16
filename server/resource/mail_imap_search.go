package resource

import (
	"bytes"
	"errors"
	"io"
	"strings"

	"github.com/artpar/go-imap"
	"github.com/doug-martin/goqu/v9"
	"github.com/emersion/go-message"
)

var errIMAPSearchNeedsContent = errors.New("IMAP search needs MIME")

func imapSearchNeedsContent() ([]byte, error) { return nil, errIMAPSearchNeedsContent }

type imapSearchHydration struct {
	index int
	size  int64
}

// Use the canonical message sizes to budget raw MIME held by a search window.
// A message with unknown size or larger than the window is loaded by itself.
// The part decoder still processes one message at a time.
func imapSearchHydrationWindow(pending []imapSearchHydration) int {
	const maxRows, maxBytes = 32, 8 << 20
	bytesLeft := int64(maxBytes)
	for i, entry := range pending[:min(len(pending), maxRows)] {
		if entry.size <= 0 || entry.size > bytesLeft {
			return max(1, i)
		}
		bytesLeft -= entry.size
	}
	return min(len(pending), maxRows)
}

type imapSearchNode struct {
	criteria *imap.SearchCriteria
	metadata goqu.Expression
	possible goqu.Expression
	exact    bool
	column   int
	not      []*imapSearchNode
	or       [][2]*imapSearchNode
}

// A content predicate has no SQL equivalent. For AND, keep every necessary
// metadata condition. For OR, only restrict SQL if both branches can be
// restricted. Negation can filter SQL only for an exact metadata subtree.
func planIMAPSearch(criteria *imap.SearchCriteria, alias string) *imapSearchNode {
	if criteria == nil {
		criteria = imap.NewSearchCriteria()
	}
	n := &imapSearchNode{
		criteria: criteria,
		metadata: imapSearchMetadata(criteria, alias),
		exact:    len(criteria.Header) == 0 && len(criteria.Body) == 0 && len(criteria.Text) == 0,
		column:   -1,
	}
	var possible []goqu.Expression
	if n.metadata != nil {
		possible = append(possible, n.metadata)
	}
	for _, child := range criteria.Not {
		child := planIMAPSearch(child, alias)
		n.not = append(n.not, child)
		if child.exact {
			if child.possible == nil {
				possible = append(possible, goqu.L("FALSE"))
			} else {
				possible = append(possible, goqu.L("NOT (?)", child.possible))
			}
		} else {
			n.exact = false
		}
	}
	for _, pair := range criteria.Or {
		left, right := planIMAPSearch(pair[0], alias), planIMAPSearch(pair[1], alias)
		n.or = append(n.or, [2]*imapSearchNode{left, right})
		n.exact = n.exact && left.exact && right.exact
		if left.possible != nil && right.possible != nil {
			possible = append(possible, goqu.Or(left.possible, right.possible))
		}
	}
	if len(possible) > 0 {
		n.possible = goqu.And(possible...)
	}
	return n
}

func (n *imapSearchNode) project(columns *[]interface{}) {
	predicate := n.metadata
	if n.exact {
		predicate = n.possible
	}
	if predicate != nil {
		n.column = len(*columns)
		*columns = append(*columns, goqu.L("CASE WHEN ? THEN 1 ELSE 0 END", predicate))
	}
	if n.exact {
		return
	}
	for _, child := range n.not {
		child.project(columns)
	}
	for _, pair := range n.or {
		pair[0].project(columns)
		pair[1].project(columns)
	}
}

func (n *imapSearchNode) matches(predicates []int, content *imapSearchContent) (bool, error) {
	if n.column >= 0 && predicates[n.column] == 0 {
		return false, nil
	}
	if n.exact {
		return true, nil
	}
	for _, child := range n.not {
		ok, err := child.matches(predicates, content)
		if ok || err != nil {
			return false, err
		}
	}
	for _, pair := range n.or {
		left, right := pair[0], pair[1]
		if right.exact && !left.exact {
			left, right = right, left
		}
		ok, err := left.matches(predicates, content)
		if err != nil {
			return false, err
		}
		if !ok {
			ok, err = right.matches(predicates, content)
			if !ok || err != nil {
				return false, err
			}
		}
	}
	return content.matches(n.criteria)
}

type imapSearchContent struct {
	load       func() ([]byte, error)
	entity     *message.Entity
	opaqueRoot bool
	textParts  []string
	readBody   bool
}

func (c *imapSearchContent) matches(criteria *imap.SearchCriteria) (bool, error) {
	if len(criteria.Header) == 0 && len(criteria.Body) == 0 && len(criteria.Text) == 0 {
		return true, nil
	}
	if c.entity == nil {
		raw, err := c.load()
		if err != nil {
			return false, err
		}
		entity, err := message.Read(bytes.NewReader(raw))
		c.opaqueRoot = MailMIMEEncodingUnsupported(err)
		if err != nil && !c.opaqueRoot {
			return false, err
		}
		c.entity = entity
	}
	for name, values := range criteria.Header {
		for _, value := range values {
			matched := false
			value = strings.ToLower(value)
			for fields := c.entity.Header.FieldsByKey(name); fields.Next(); {
				decoded, _ := fields.Text()
				if strings.Contains(strings.ToLower(decoded), value) {
					matched = true
					break
				}
			}
			if !matched {
				return false, nil
			}
		}
	}
	for _, value := range criteria.Body {
		ok, err := c.inBody(value)
		if !ok || err != nil {
			return false, err
		}
	}
	for _, value := range criteria.Text {
		value = strings.ToLower(value)
		matched := false
		for fields := c.entity.Header.Fields(); fields.Next(); {
			decoded, _ := fields.Text()
			if strings.Contains(strings.ToLower(fields.Key()+": "+decoded), value) {
				matched = true
				break
			}
		}
		if !matched {
			ok, err := c.inBody(value)
			if !ok || err != nil {
				return false, err
			}
		}
	}
	return true, nil
}

func (c *imapSearchContent) inBody(value string) (bool, error) {
	if value == "" {
		return true, nil
	}
	if c.opaqueRoot {
		return false, nil
	}
	if !c.readBody {
		// Bound decoded text per message, including embedded messages. Binary
		// attachments are never read into the search text.
		remaining := int64(64 << 20)
		if err := c.collectText(c.entity, 0, &remaining); err != nil {
			return false, err
		}
		c.readBody = true
	}
	value = strings.ToLower(value)
	for _, part := range c.textParts {
		if strings.Contains(part, value) {
			return true, nil
		}
	}
	return false, nil
}

func (c *imapSearchContent) collectText(entity *message.Entity, depth int, remaining *int64) error {
	if depth > 32 {
		return errors.New("IMAP search MIME nesting limit exceeded")
	}
	if multipart := entity.MultipartReader(); multipart != nil {
		for {
			part, err := multipart.NextPart()
			if err == io.EOF {
				return nil
			}
			if MailMIMEEncodingUnsupported(err) {
				continue
			}
			if err != nil {
				return err
			}
			if err := c.collectText(part, depth+1, remaining); err != nil {
				return err
			}
		}
	}
	kind, _, _ := entity.Header.ContentType()
	if strings.EqualFold(kind, "message/rfc822") {
		nested, err := message.Read(entity.Body)
		if MailMIMEEncodingUnsupported(err) {
			return nil
		}
		if err != nil {
			return err
		}
		return c.collectText(nested, depth+1, remaining)
	}
	if !strings.HasPrefix(strings.ToLower(kind), "text/") {
		return nil
	}
	data, err := io.ReadAll(io.LimitReader(entity.Body, *remaining+1))
	if err != nil {
		return err
	}
	*remaining -= int64(len(data))
	if *remaining < 0 {
		return errors.New("IMAP search decoded text limit exceeded")
	}
	c.textParts = append(c.textParts, strings.ToLower(string(data)))
	return nil
}
