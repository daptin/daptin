package server

import (
	"context"
	"encoding/binary"
	"errors"
	"fmt"
	"hash/fnv"
	"io"
	"os"
	"strings"
	"sync"
)

const yjsJournalLimit = 50 << 20

type yjsStore interface {
	read(room string) ([]byte, error)
	appendUpdate(ctx context.Context, room string, framed []byte) error
}

// A journal is a sequence of complete binary WebSocket frames, each preceded
// by its unsigned varint length. Keep that format for both existing stores.
func yjsJournalRecord(frame []byte) []byte {
	return append(binary.AppendUvarint(nil, uint64(len(frame))), frame...)
}

func yjsReplay(journal []byte) ([][]byte, error) {
	if len(journal) > yjsJournalLimit {
		return nil, fmt.Errorf("YJS journal exceeds %d bytes", yjsJournalLimit)
	}
	var updates [][]byte
	for len(journal) > 0 {
		length, prefix := binary.Uvarint(journal)
		if prefix <= 0 || length == 0 || length > uint64(len(journal)-prefix) {
			return nil, errors.New("invalid YJS journal entry")
		}
		journal = journal[prefix:]
		frame := journal[:length]
		kind, err := classifyYjsFrame(frame)
		if err != nil {
			return nil, err
		}
		if kind == yjsUpdate {
			updates = append(updates, frame)
		}
		journal = journal[length:]
	}
	return updates, nil
}

var errInvalidYjsRoom = errors.New("invalid standalone YJS room name")

type diskYjsStore struct {
	dir string
	mu  [64]sync.Mutex
}

func (s *diskYjsStore) open(room string) (root *os.Root, release func(), err error) {
	if room == "" || room == "." || room == ".." || strings.ContainsAny(room, "/\\\x00") {
		return nil, nil, errInvalidYjsRoom
	}
	h := fnv.New32a()
	_, _ = h.Write([]byte(room))
	mu := &s.mu[h.Sum32()%uint32(len(s.mu))]
	mu.Lock()
	root, err = os.OpenRoot(s.dir)
	if err != nil {
		mu.Unlock()
		return nil, nil, err
	}
	return root, func() {
		_ = root.Close()
		mu.Unlock()
	}, nil
}

func (s *diskYjsStore) read(room string) ([]byte, error) {
	root, release, err := s.open(room)
	if err != nil {
		return nil, err
	}
	defer release()
	file, err := root.Open(room)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	defer file.Close()
	return io.ReadAll(io.LimitReader(file, yjsJournalLimit+1))
}

func (s *diskYjsStore) appendUpdate(ctx context.Context, room string, frame []byte) error {
	root, release, err := s.open(room)
	if err != nil {
		return err
	}
	defer release()
	if err := ctx.Err(); err != nil {
		return err
	}

	file, err := root.OpenFile(room, os.O_WRONLY|os.O_CREATE, 0600)
	if err != nil {
		return err
	}
	defer file.Close()
	length, err := file.Seek(0, io.SeekEnd)
	if err != nil {
		return err
	}
	if int64(len(frame)) > yjsJournalLimit-length {
		return errors.New("YJS room storage limit reached")
	}
	if _, err := file.Write(frame); err != nil {
		return errors.Join(err, file.Truncate(length))
	}
	if err := file.Sync(); err != nil {
		return errors.Join(err, file.Truncate(length))
	}
	return nil
}
