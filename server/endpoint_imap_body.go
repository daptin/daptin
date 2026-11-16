package server

import (
	"slices"

	"github.com/artpar/go-imap"
	"github.com/artpar/go-imap/responses"
	"github.com/artpar/go-imap/server"
)

// The stock formatter toggles extension data only on the root of the shared
// body tree. Adapt mixed requests so nested BODY parts also omit extensions.
type bodyIMAPExtension struct{}

func (*bodyIMAPExtension) Capabilities(server.Conn) []string { return nil }

func (*bodyIMAPExtension) Command(name string) server.HandlerFactory {
	if name == "FETCH" {
		return func() server.Handler { return &bodyIMAPFetch{} }
	}
	return nil
}

type bodyIMAPFetch struct{ server.Fetch }

func (cmd *bodyIMAPFetch) responseConn(conn server.Conn) server.Conn {
	if slices.Contains(cmd.Items, imap.FetchBody) && slices.Contains(cmd.Items, imap.FetchBodyStructure) {
		return bodyIMAPConn{Conn: conn}
	}
	return conn
}

func (cmd *bodyIMAPFetch) Handle(conn server.Conn) error {
	return cmd.Fetch.Handle(cmd.responseConn(conn))
}

func (cmd *bodyIMAPFetch) UidHandle(conn server.Conn) error {
	return cmd.Fetch.UidHandle(cmd.responseConn(conn))
}

type bodyIMAPConn struct{ server.Conn }

func (conn bodyIMAPConn) WriteResp(response imap.WriterTo) error {
	if fetch, ok := response.(*responses.Fetch); ok {
		return conn.Conn.WriteResp(bodyIMAPResponse{fetch.Messages})
	}
	return conn.Conn.WriteResp(response)
}

type bodyIMAPResponse struct{ messages <-chan *imap.Message }

func (r bodyIMAPResponse) WriteTo(w *imap.Writer) error {
	var writeErr error
	for msg := range r.messages {
		if writeErr != nil {
			continue
		}
		fields := msg.Format()
		for i := 0; i+1 < len(fields); i += 2 {
			if fields[i] == imap.RawString(imap.FetchBody) {
				fields[i+1] = bodyWithoutExtensions(msg.BodyStructure).Format()
				break
			}
		}
		writeErr = imap.NewUntaggedResp([]interface{}{msg.SeqNum, imap.RawString("FETCH"), fields}).WriteTo(w)
	}
	return writeErr
}

func bodyWithoutExtensions(body *imap.BodyStructure) *imap.BodyStructure {
	plain := *body
	plain.Extended = false
	if len(body.Parts) > 0 {
		plain.Parts = make([]*imap.BodyStructure, len(body.Parts))
		for i, part := range body.Parts {
			plain.Parts[i] = bodyWithoutExtensions(part)
		}
	}
	if body.BodyStructure != nil {
		plain.BodyStructure = bodyWithoutExtensions(body.BodyStructure)
	}
	return &plain
}
