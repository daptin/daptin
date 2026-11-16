package resource

import (
	"slices"

	"github.com/artpar/go-imap"
	"github.com/jmoiron/sqlx"
)

// IMAPSequenceView is the UID order the server has announced on a selected
// connection. The protocol adapter replaces it only after sending updates.
type IMAPSequenceView []uint32

func (dimb *DaptinImapMailBox) SetIMAPSequenceView(uids IMAPSequenceView) {
	dimb.sequenceView = uids
}

func (dimb *DaptinImapMailBox) IMAPSequenceView() IMAPSequenceView {
	return dimb.sequenceView
}

func (view IMAPSequenceView) sequence(uid uint32) uint32 {
	index, ok := slices.BinarySearch(view, uid)
	if !ok {
		return 0
	}
	return uint32(index + 1)
}

func (view IMAPSequenceView) resolve(set *imap.SeqSet, uid bool) *imap.SeqSet {
	if set == nil {
		return nil
	}
	resolved := new(imap.SeqSet)
	if len(view) == 0 {
		return resolved
	}
	limit := uint32(len(view))
	if uid {
		limit = view[len(view)-1]
	}
	for _, part := range set.Set {
		start, end := part.Start, part.Stop
		if start == 0 {
			start = limit
		}
		if end == 0 {
			end = limit
		}
		if start > end {
			start, end = end, start
		}
		if start > limit {
			continue
		}
		end = min(end, limit)
		if !uid {
			start, end = view[start-1], view[end-1]
		}
		// UIDs are monotonic and can never be reused. A gap between two
		// announced UIDs cannot acquire a new message behind the session.
		resolved.AddRange(start, end)
	}
	return resolved
}

func (dimb *DaptinImapMailBox) selectAnnouncedMessages(uid bool, set *imap.SeqSet, tx *sqlx.Tx) ([]MailSelection, error) {
	requested := dimb.sequenceView.resolve(set, uid)
	if requested == nil || requested.Empty() {
		return nil, nil
	}
	rows, err := dimb.dbResource["mail"].SelectMailBoxCandidates(dimb.mailBoxId, true, requested, tx)
	if err != nil {
		return nil, err
	}
	visible := rows[:0]
	for _, row := range rows {
		if seq := dimb.sequenceView.sequence(row.UID); seq != 0 {
			row.SequenceNumber = seq
			visible = append(visible, row)
		}
	}
	return visible, nil
}

func sameIMAPTarget(row map[string]interface{}, uid uint32) bool {
	actual, err := ResourceRowInt64(row["uid"])
	// A zero stored UID uses the canonical row's numeric ID. The caller
	// already restricts the lookup to that exact canonical reference.
	return err == nil && (actual == 0 || actual == int64(uid))
}
