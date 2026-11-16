package actions

import (
	"bytes"
	"strings"
)

// The parser supplies wire-order leaves. Keep their parentage for body and
// resource selection without changing the leaves or their download indices.
type mailMIMEPart struct {
	media, cid, start, rootType string
	attached, opaque            bool
	flowed, delSp               bool
	index                       int
	parent                      *mailMIMEPart
	children                    []*mailMIMEPart
}

func (node *mailMIMEPart) relatedRoot() *mailMIMEPart {
	if len(node.children) == 0 {
		return nil
	}
	root := node.children[0]
	if node.start != "" {
		id := strings.Trim(strings.TrimSpace(node.start), "<>")
		root = nil
		for _, child := range node.children {
			if id != "" && child.cid == id {
				if root != nil {
					return nil
				}
				root = child
			}
		}
	}
	if root != nil && node.rootType != "" && !strings.EqualFold(node.rootType, root.media) {
		return nil
	}
	return root
}

func (m *normalizedMail) preferredBody(node *mailMIMEPart, media string, relatedRoot bool) *mailMIMEPart {
	if node == nil || node.opaque || (node.attached && !relatedRoot) {
		return nil
	}
	if node.index >= 0 {
		if node.media == media && len(bytes.TrimSpace(m.Parts[node.index].Content)) != 0 {
			return node
		}
		return nil
	}
	switch node.media {
	case "multipart/related":
		return m.preferredBody(node.relatedRoot(), media, true)
	case "multipart/alternative":
		for i := len(node.children) - 1; i >= 0; i-- {
			if part := m.preferredBody(node.children[i], media, false); part != nil {
				return part
			}
		}
	default:
		for _, child := range node.children {
			if part := m.preferredBody(child, media, false); part != nil {
				return part
			}
		}
	}
	return nil
}

func childContaining(parent, leaf *mailMIMEPart) *mailMIMEPart {
	for child := leaf; child != nil; child = child.parent {
		if child.parent == parent {
			return child
		}
	}
	return nil
}

func relatedScope(node *mailMIMEPart) *mailMIMEPart {
	for node = node.parent; node != nil; node = node.parent {
		if node.media == "multipart/related" {
			return node
		}
	}
	return nil
}

func (m *normalizedMail) hasRasterCID(node *mailMIMEPart) bool {
	if node.index >= 0 && node.cid != "" {
		part := m.Parts[node.index]
		switch part.MediaType {
		case "image/png", "image/jpeg", "image/gif", "image/webp":
			return part.DecodedSize <= 5<<20
		}
	}
	for _, child := range node.children {
		if m.hasRasterCID(child) {
			return true
		}
	}
	return false
}

// Calendar payloads are independently downloadable, including scheduling
// parts that accompany a human-readable alternative. They never supply
// the displayed body or an HTML inline resource.
func (m *normalizedMail) retainCalendarParts(node *mailMIMEPart) {
	if node.opaque {
		return
	}
	if node.index >= 0 && node.media == "text/calendar" {
		part := &m.Parts[node.index]
		part.Presentation = "attachment"
		if part.Filename == "" {
			part.Filename = "event.ics"
		}
	}
	for _, child := range node.children {
		m.retainCalendarParts(child)
	}
}

func (m *normalizedMail) selectPresentation(tree *mailMIMEPart) {
	if tree == nil {
		return
	}
	m.retainCalendarParts(tree)
	plain := m.preferredBody(tree, "text/plain", false)
	html := m.preferredBody(tree, "text/html", false)
	if plain != nil {
		m.Parts[plain.index].Presentation = "body"
		m.PreferredTextBody = string(m.Parts[plain.index].Content)
		if plain.flowed {
			m.PreferredTextBody = unwrapFlowedMailText(m.PreferredTextBody, plain.delSp)
		}
	}
	var scope *mailMIMEPart
	if html != nil {
		m.Parts[html.index].Presentation = "body"
		m.HTMLBody = string(m.Parts[html.index].Content)
		scope = relatedScope(html)
	}
	inline := map[string]int{}
	var visit func(*mailMIMEPart, bool, bool)
	visit = func(node *mailMIMEPart, allowCID, isRelatedRoot bool) {
		if node.opaque || node.media == "text/calendar" {
			return
		}
		if node.index >= 0 {
			part := &m.Parts[node.index]
			if part.Presentation != "body" && (part.Kind == "attachment" || part.Filename != "" || part.ContentID != "") {
				part.Presentation = "attachment"
				if html != nil && allowCID && part.ContentID != "" && relatedScope(node) == scope && (scope != nil || !node.attached) {
					if _, duplicate := inline[part.ContentID]; duplicate {
						inline[part.ContentID] = -1
					} else {
						inline[part.ContentID] = node.index
					}
				}
			}
			return
		}
		allowCID = allowCID && (!node.attached || isRelatedRoot)
		switch node.media {
		case "multipart/related":
			root := node.relatedRoot()
			for _, child := range node.children {
				visit(child, allowCID && root != nil, child == root)
			}
		case "multipart/alternative":
			chosen := childContaining(node, html)
			if chosen == nil {
				chosen = childContaining(node, plain)
			}
			if chosen == nil && len(node.children) != 0 {
				chosen = node.children[len(node.children)-1]
				if html != nil && allowCID && relatedScope(node) == scope {
					for i := len(node.children) - 1; i >= 0; i-- {
						if m.hasRasterCID(node.children[i]) {
							chosen = node.children[i]
							break
						}
					}
				}
			}
			if chosen != nil {
				visit(chosen, allowCID, false)
			}
		default:
			for _, child := range node.children {
				visit(child, allowCID, false)
			}
		}
	}
	visit(tree, true, false)
	for _, index := range inline {
		if index >= 0 {
			m.Parts[index].Presentation = "html-resource"
		}
	}
}
