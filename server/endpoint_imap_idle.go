package server

import (
	"context"
	"errors"
	"io"
	"slices"
	"strings"
	"time"

	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/server"
	"github.com/daptin/daptin/server/resource"
)

const imapIdlePollInterval = 2 * time.Second
const imapIdleOperationLimit = 5 * time.Second

// The pinned server owns IDLE's parsing and tagged response; Enable
// deliberately refuses overrides of that built-in. ConnExtension exposes
// the continuation's reader and writer, where we can own the waiting phase.
type idleIMAPConn struct {
	server.Conn
	waiting  bool
	pending  []byte
	baseline *resource.IMAPIdleUpdate
}

func (conn *idleIMAPConn) WriteResp(response imap.WriterTo) error {
	if continuation, ok := response.(*imap.ContinuationReq); ok && continuation.Info == "idling" {
		if conn.Context().User == nil {
			return server.ErrNotAuthenticated
		}
		if selected, ok := conn.Context().Mailbox.(idleIMAPSelection); ok {
			baseline, err := selected.idleBaseline()
			if err != nil {
				return err
			}
			conn.baseline = baseline
		}
		conn.waiting = true
	}
	return conn.Conn.WriteResp(response)
}

func (conn *idleIMAPConn) Read(dst []byte) (int, error) {
	if len(dst) == 0 {
		return 0, nil
	}
	if conn.waiting {
		conn.waiting = false
		baseline := conn.baseline
		conn.baseline = nil
		line, err := waitIMAPIdle(conn.Conn, baseline)
		if err != nil {
			return 0, err
		}
		conn.pending = line
	}
	if len(conn.pending) > 0 {
		n := copy(dst, conn.pending)
		conn.pending = conn.pending[n:]
		return n, nil
	}
	return conn.Conn.Read(dst)
}

func (conn *idleIMAPConn) SetWriteDeadline(deadline time.Time) error {
	if socket, ok := conn.Conn.(interface{ SetWriteDeadline(time.Time) error }); ok {
		return socket.SetWriteDeadline(deadline)
	}
	return nil
}

type idleContinuation struct {
	line []byte
	err  error
}

type idleIMAPSelection interface {
	idleBaseline() (*resource.IMAPIdleUpdate, error)
	pollIdle(context.Context, *resource.IMAPIdleUpdate) (*resource.IMAPIdleUpdate, error)
}

func waitIMAPIdle(conn server.Conn, previous *resource.IMAPIdleUpdate) ([]byte, error) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	finished := make(chan idleContinuation, 1)
	go func() {
		line, err := readIMAPIdleDone(conn)
		cancel()
		finished <- idleContinuation{line, err}
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

	selected, ok := conn.Context().Mailbox.(idleIMAPSelection)
	if !ok {
		result := <-finished
		active = false
		if result.err != nil {
			_ = conn.Close()
		}
		return result.line, result.err
	}
	ticker := time.NewTicker(imapIdlePollInterval)
	defer ticker.Stop()
	for {
		select {
		case result := <-finished:
			active = false
			if result.err != nil {
				_ = conn.Close()
			}
			return result.line, result.err
		case <-ticker.C:
			update, err := selected.pollIdle(ctx, previous)
			if ctx.Err() != nil {
				continue // DONE or transport close; don't announce another update
			}
			if err != nil {
				// The library acknowledges WriteResp before its final flush.
				// Flush BYE before closing this transport and its reader.
				if socket, ok := conn.(interface{ SetWriteDeadline(time.Time) error }); ok {
					_ = socket.SetWriteDeadline(time.Now().Add(imapIdleOperationLimit))
				}
				_ = conn.WriteResp(idleEndResponse{})
				return nil, err
			}
			if update != nil {
				previous = update
			}
		}
	}
}

type idleEndResponse struct{}

func (idleEndResponse) WriteTo(writer *imap.Writer) error {
	bye := &imap.StatusResp{Type: imap.StatusRespBye, Info: "Cannot refresh selected mailbox"}
	if err := bye.WriteTo(writer); err != nil {
		return err
	}
	return writer.Flush()
}

func (box *selectedIMAPMailbox) idleBaseline() (*resource.IMAPIdleUpdate, error) {
	ctx, cancel := context.WithTimeout(context.Background(), imapIdleOperationLimit)
	defer cancel()
	return box.ReadIMAPIdleSelection(ctx, false, nil)
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
	if err := box.announceSelection(update.UIDs, update.Recent); err != nil {
		return nil, err
	}
	if previous != nil {
		for index, uid := range update.UIDs {
			before, wasReadable := previous.Flags[uid]
			after, isReadable := update.Flags[uid]
			if !wasReadable || !isReadable || slices.EqualFunc(before.Values, after.Values, strings.EqualFold) {
				continue
			}
			message := imap.NewMessage(uint32(index+1), []imap.FetchItem{imap.FetchUid, imap.FetchFlags})
			message.Uid = uid
			message.Flags = after.Values
			if after.Recent {
				message.Flags = append(slices.Clone(after.Values), imap.RecentFlag)
			}
			if err := box.conn.WriteResp(imap.NewUntaggedResp([]interface{}{message.SeqNum, imap.RawString("FETCH"), message.Format()})); err != nil {
				return nil, err
			}
		}
	}
	return update, nil
}

// Read exactly the continuation line, preserving the connection's own parser
// buffer for the next tagged command. The only valid continuation is DONE.
func readIMAPIdleDone(conn io.Reader) ([]byte, error) {
	var line [len("DONE\r\n")]byte
	for i := range line {
		if _, err := io.ReadFull(conn, line[i:i+1]); err != nil {
			return nil, err
		}
		if line[i] == '\n' {
			break
		}
	}
	if !strings.EqualFold(string(line[:]), "DONE\r\n") {
		return nil, errors.New("expected DONE continuation")
	}
	return line[:], nil
}
