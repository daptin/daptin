package server

import (
	"context"
	"errors"
	"io"
	"strings"
	"time"

	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/server"
	"github.com/daptin/daptin/server/resource"
)

const imapIdlePollInterval = 2 * time.Second
const imapIdleOperationLimit = 5 * time.Second

type selectedIMAPIdle struct{}

func (*selectedIMAPIdle) Parse(args []interface{}) error {
	if len(args) != 0 {
		return errors.New("IDLE takes no arguments")
	}
	return nil
}

func (*selectedIMAPIdle) Handle(conn server.Conn) error {
	if conn.Context().User == nil {
		return server.ErrNotAuthenticated
	}
	if err := conn.WriteResp(&imap.ContinuationReq{Info: "idling"}); err != nil {
		return err
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan error, 1)
	go func() {
		err := readIMAPIdleDone(conn)
		cancel()
		finished <- err
	}()
	active := true
	defer func() {
		if active {
			// On a server-side error, unblock and join the continuation
			// reader before handing the connection back to the parser.
			_ = conn.Close()
			<-finished
		}
	}()

	selected, ok := conn.Context().Mailbox.(interface {
		pollIdle(context.Context, *resource.IMAPIdleUpdate) (*resource.IMAPIdleUpdate, error)
	})
	if !ok {
		err := <-finished
		active = false
		if err != nil {
			_ = conn.Close()
		}
		return err
	}
	ticker := time.NewTicker(imapIdlePollInterval)
	defer ticker.Stop()
	var previous *resource.IMAPIdleUpdate
	for {
		select {
		case err := <-finished:
			active = false
			if err != nil {
				_ = conn.Close()
			}
			return err
		case <-ticker.C:
			update, err := selected.pollIdle(ctx, previous)
			if ctx.Err() != nil {
				continue // DONE or transport close; don't announce another update
			}
			if err != nil {
				_ = conn.WriteResp(&imap.StatusResp{Type: imap.StatusRespBye, Info: "Cannot refresh selected mailbox"})
				return err
			}
			if update != nil {
				previous = update
			}
		}
	}
}

func (box *selectedIMAPMailbox) pollIdle(parent context.Context, previous *resource.IMAPIdleUpdate) (*resource.IMAPIdleUpdate, error) {
	ctx, cancel := context.WithTimeout(parent, imapIdleOperationLimit)
	defer cancel()
	update, err := box.ReadIMAPIdleSelection(ctx, !box.conn.Context().MailboxReadOnly, previous)
	if err != nil || update == nil || parent.Err() != nil {
		return nil, err
	}
	if conn, ok := box.conn.(interface{ SetWriteDeadline(time.Time) error }); ok {
		if err = conn.SetWriteDeadline(time.Now().Add(imapIdleOperationLimit)); err != nil {
			return nil, err
		}
		defer conn.SetWriteDeadline(time.Time{})
	}
	return update, box.announceSelection(update.UIDs, update.Recent)
}

// Read exactly the continuation line, preserving the connection's own parser
// buffer for the next tagged command. The only valid continuation is DONE.
func readIMAPIdleDone(conn io.Reader) error {
	var line [len("DONE\r\n")]byte
	for i := range line {
		if _, err := io.ReadFull(conn, line[i:i+1]); err != nil {
			return err
		}
		if line[i] == '\n' {
			break
		}
	}
	if !strings.EqualFold(string(line[:]), "DONE\r\n") {
		return errors.New("expected DONE continuation")
	}
	return nil
}
