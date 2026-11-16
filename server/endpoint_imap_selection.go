package server

import (
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/commands"
	"github.com/artpar/go-imap/responses"
	"github.com/artpar/go-imap/server"
	"github.com/daptin/daptin/server/resource"
)

// A connection's last announced order is protocol state. The store remains
// authoritative for membership and always supplies a fresh metadata view.
type selectedIMAPMailbox struct {
	*resource.DaptinImapMailBox
	conn server.Conn
}

func (box *selectedIMAPMailbox) syncSelection(consumeRecent bool) error {
	uids, recent, err := box.ReadIMAPSelection(consumeRecent && !box.conn.Context().MailboxReadOnly)
	if err != nil {
		return err
	}
	return box.announceSelection(uids, recent)
}

func (box *selectedIMAPMailbox) announceSelection(uids []uint32, recent uint32) error {
	announced := box.IMAPSequenceView()
	removed := missingIMAPSequences(announced, uids)
	// Descending order leaves all remaining sequence numbers valid as each
	// untagged EXPUNGE renumbers the mailbox.
	for i := len(removed) - 1; i >= 0; i-- {
		if err := box.conn.WriteResp(imap.NewUntaggedResp([]interface{}{removed[i], imap.RawString("EXPUNGE")})); err != nil {
			return err
		}
	}
	if len(uids) > len(announced)-len(removed) {
		status := imap.NewMailboxStatus(box.Name(), []imap.StatusItem{imap.StatusMessages, imap.StatusRecent})
		status.Messages, status.Recent = uint32(len(uids)), recent
		if err := box.conn.WriteResp(&responses.Select{Mailbox: status}); err != nil {
			return err
		}
	}
	box.SetIMAPSequenceView(uids)
	return nil
}

func missingIMAPSequences(previous, current []uint32) []uint32 {
	var removed []uint32
	cursor := 0
	for index, uid := range previous {
		for cursor < len(current) && current[cursor] < uid {
			cursor++
		}
		if cursor == len(current) || current[cursor] != uid {
			removed = append(removed, uint32(index+1))
		} else {
			cursor++
		}
	}
	return removed
}

func syncSelectedIMAPMailbox(conn server.Conn, consumeRecent bool) error {
	if box, ok := conn.Context().Mailbox.(interface{ syncSelection(bool) error }); ok {
		return box.syncSelection(consumeRecent)
	}
	return nil
}

type selectedIMAPNoop struct{ commands.Noop }

func (*selectedIMAPNoop) Handle(conn server.Conn) error { return syncSelectedIMAPMailbox(conn, true) }

type selectedIMAPAppend struct{ server.Append }

func (cmd *selectedIMAPAppend) Handle(conn server.Conn) error {
	selected := conn.Context().Mailbox
	if selected == nil || selected.Name() != imap.CanonicalMailboxName(cmd.Mailbox) {
		return cmd.Append.Handle(conn)
	}
	if err := cmd.Append.Handle(appendIMAPResponse{Conn: conn}); err != nil {
		return err
	}
	return syncSelectedIMAPMailbox(conn, false)
}

// APPEND's default inline EXISTS doesn't identify which UIDs changed. The
// same connection reports all changes once through syncSelection instead.
type appendIMAPResponse struct{ server.Conn }

func (conn appendIMAPResponse) WriteResp(response imap.WriterTo) error {
	if _, isInlineStatus := response.(*responses.Select); isInlineStatus {
		return nil
	}
	return conn.Conn.WriteResp(response)
}

type selectedIMAPCopy struct{ server.Copy }

func (cmd *selectedIMAPCopy) Handle(conn server.Conn) error    { return cmd.handle(conn, false) }
func (cmd *selectedIMAPCopy) UidHandle(conn server.Conn) error { return cmd.handle(conn, true) }
func (cmd *selectedIMAPCopy) handle(conn server.Conn, uid bool) error {
	if uid {
		if err := cmd.Copy.UidHandle(conn); err != nil {
			return err
		}
	} else if err := cmd.Copy.Handle(conn); err != nil {
		return err
	}
	if conn.Context().Mailbox.Name() == imap.CanonicalMailboxName(cmd.Mailbox) {
		return syncSelectedIMAPMailbox(conn, false)
	}
	return nil
}

type selectedIMAPExpunge struct{ commands.Expunge }

func (*selectedIMAPExpunge) Handle(conn server.Conn) error {
	ctx := conn.Context()
	if ctx.Mailbox == nil {
		return server.ErrNoMailboxSelected
	}
	if ctx.MailboxReadOnly {
		return server.ErrMailboxReadOnly
	}
	if err := ctx.Mailbox.Expunge(); err != nil {
		return err
	}
	return syncSelectedIMAPMailbox(conn, false)
}
