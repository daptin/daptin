package server

import (
	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/responses"
	"github.com/artpar/go-imap/server"
	"github.com/daptin/daptin/server/resource"
)

// Use the library's normal selection and protocol parser, and adapt its
// read-only selection behavior at the server extension boundary.
type selectedIMAPExtension struct{}

func (*selectedIMAPExtension) Capabilities(server.Conn) []string { return nil }
func (*selectedIMAPExtension) Command(name string) server.HandlerFactory {
	switch name {
	case "EXAMINE", "SELECT":
		return func() server.Handler { return &selectedIMAPSelect{examine: name == "EXAMINE"} }
	case "IDLE":
		return func() server.Handler { return &selectedIMAPIdle{} }
	case "NOOP":
		return func() server.Handler { return &selectedIMAPNoop{} }
	case "APPEND":
		return func() server.Handler { return &selectedIMAPAppend{} }
	case "COPY":
		return func() server.Handler { return &selectedIMAPCopy{} }
	case "EXPUNGE":
		return func() server.Handler { return &selectedIMAPExpunge{} }
	}
	return nil
}

type selectedIMAPSelect struct {
	server.Select
	examine bool
}

func (cmd *selectedIMAPSelect) Handle(conn server.Conn) error {
	cmd.ReadOnly = cmd.examine
	return cmd.Select.Handle(selectionIMAPResponse{Conn: conn})
}

type selectionIMAPResponse struct{ server.Conn }

func (conn selectionIMAPResponse) WriteResp(response imap.WriterTo) error {
	if selecting, ok := response.(*responses.Select); ok {
		ctx := conn.Context()
		if box, ok := ctx.Mailbox.(*resource.DaptinImapMailBox); ok {
			uids, recent, err := box.ReadIMAPSelection(false)
			if err != nil {
				ctx.Mailbox = nil
				ctx.MailboxReadOnly = false
				return err
			}
			selecting.Mailbox.Messages = uint32(len(uids))
			selecting.Mailbox.Recent = recent
			if len(uids) > 0 && selecting.Mailbox.UidNext <= uids[len(uids)-1] {
				selecting.Mailbox.UidNext = uids[len(uids)-1] + 1
			}
			box.SetIMAPSequenceView(uids)
			selected := &selectedIMAPMailbox{DaptinImapMailBox: box, conn: conn.Conn}
			if ctx.MailboxReadOnly {
				selecting.Mailbox.PermanentFlags = []string{}
				ctx.Mailbox = &readOnlyIMAPMailbox{selectedIMAPMailbox: selected}
			} else {
				ctx.Mailbox = &moveIMAPMailbox{selectedIMAPMailbox: selected}
			}
		}
	}
	return conn.Conn.WriteResp(response)
}

// Preserve the concrete mailbox's read capabilities while suppressing
// implicit writes for this read-only selection.
type readOnlyIMAPMailbox struct{ *selectedIMAPMailbox }

func (box *readOnlyIMAPMailbox) ListMessages(uid bool, seqset *imap.SeqSet, items []imap.FetchItem, messages chan<- *imap.Message) error {
	return box.ListMessagesReadOnly(uid, seqset, items, messages)
}

func (*readOnlyIMAPMailbox) MoveMessages(bool, *imap.SeqSet, string) error {
	return server.ErrMailboxReadOnly
}

// CLOSE deselects a mailbox but must not expunge any read-only selection.
func (*readOnlyIMAPMailbox) Expunge() error { return nil }

// The IMAP server exposes MOVE through backend.MoveMailbox. The selected
// mailbox adapter adds the initiating client's wire responses after commit.
type moveIMAPMailbox struct{ *selectedIMAPMailbox }

func (box *moveIMAPMailbox) MoveMessages(uid bool, set *imap.SeqSet, name string) error {
	if err := box.MoveSelectedMessages(uid, set, name); err != nil {
		return err
	}
	return box.syncSelection(false)
}
