package server

import (
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/responses"
	"github.com/artpar/go-imap/server"
	"github.com/daptin/daptin/server/resource"
)

// Use the library's normal selection and protocol parser, and adapt its
// read-only selection behavior at the server extension boundary.
type readOnlyIMAPExtension struct{}

func (*readOnlyIMAPExtension) Capabilities(server.Conn) []string { return nil }
func (*readOnlyIMAPExtension) Command(name string) server.HandlerFactory {
	if name == "EXAMINE" || name == "SELECT" {
		return func() server.Handler { return &readOnlyIMAPSelect{examine: name == "EXAMINE"} }
	}
	return nil
}

type readOnlyIMAPSelect struct {
	server.Select
	examine bool
}

func (cmd *readOnlyIMAPSelect) Handle(conn server.Conn) error {
	c := conn
	if cmd.examine {
		cmd.ReadOnly = true
		c = readOnlyIMAPResponse{Conn: conn}
	}
	err := cmd.Select.Handle(c)
	ctx := conn.Context()
	if box, ok := ctx.Mailbox.(*resource.DaptinImapMailBox); ok && ctx.MailboxReadOnly {
		ctx.Mailbox = &readOnlyIMAPMailbox{DaptinImapMailBox: box}
	} else if ok {
		ctx.Mailbox = &moveIMAPMailbox{DaptinImapMailBox: box, conn: conn}
	}
	return err
}

type readOnlyIMAPResponse struct{ server.Conn }

func (conn readOnlyIMAPResponse) WriteResp(response imap.WriterTo) error {
	if selected, ok := response.(*responses.Select); ok {
		// Daptin Status allocates a fresh response. An empty, non-nil list
		// explicitly sends PERMANENTFLAGS () on the wire (RFC 3501 §6.3.2).
		selected.Mailbox.PermanentFlags = []string{}
	}
	return conn.Conn.WriteResp(response)
}

// Preserve the concrete mailbox's read capabilities while suppressing
// implicit writes for this read-only selection.
type readOnlyIMAPMailbox struct{ *resource.DaptinImapMailBox }

func (box *readOnlyIMAPMailbox) ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, messages chan<- *imap.Message) error {
	return box.ListMessagesReadOnly(uid, seqset, items, messages)
}

func (box *readOnlyIMAPMailbox) Poll() error { return box.PollReadOnly() }

func (*readOnlyIMAPMailbox) MoveMessages(bool, *imap.SeqSet, string) error {
	return server.ErrMailboxReadOnly
}

// CLOSE deselects a mailbox but must not expunge any read-only selection.
func (*readOnlyIMAPMailbox) Expunge() error { return nil }

// The IMAP server exposes MOVE through backend.MoveMailbox. The selected
// mailbox adapter adds the initiating client's wire responses after commit.
type moveIMAPMailbox struct {
	*resource.DaptinImapMailBox
	conn server.Conn
}

func (box *moveIMAPMailbox) MoveMessages(uid bool, set *imap.SeqSet, name string) error {
	removed, exists, err := box.MoveSelectedMessages(uid, set, name)
	if err != nil {
		return err
	}
	// Descending sequence numbers retain their original meaning as each
	// EXPUNGE renumbers the remaining messages.
	for i := len(removed) - 1; i >= 0; i-- {
		if err = box.conn.WriteResp(imap.NewUntaggedResp([]interface{}{removed[i], imap.RawString("EXPUNGE")})); err != nil {
			return err
		}
	}
	if exists != nil {
		return box.conn.WriteResp(imap.NewUntaggedResp([]interface{}{*exists, imap.RawString("EXISTS")}))
	}
	return nil
}
