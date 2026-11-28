package resource

import (
	"context"
	"database/sql"
	"errors"

	"github.com/daptin/daptin/server/database"
	"github.com/google/uuid"
	"github.com/jmoiron/sqlx"
)

const restartGenerationKey = "runtime.restart"

var ErrRestartPending = errors.New("runtime restart pending")

// RuntimeRestart compares this process's startup snapshot with the committed
// cluster request. SQL owns the marker; PubSub only wakes the runtime monitor.
type RuntimeRestart struct {
	config *ConfigStore
	db     database.DatabaseConnection
	loaded string
}

func NewRuntimeRestart(db database.DatabaseConnection, config *ConfigStore) (*RuntimeRestart, error) {
	state := &RuntimeRestart{db: db, config: config}
	tx, err := db.Beginx()
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	state.loaded, err = state.current(tx)
	if err != nil {
		return nil, err
	}
	return state, nil
}
func (s *RuntimeRestart) current(tx *sqlx.Tx) (string, error) {
	value, err := s.config.GetConfigValueFor(restartGenerationKey, "backend", tx)
	if errors.Is(err, sql.ErrNoRows) {
		return "", nil
	}
	return value, err
}
func (s *RuntimeRestart) Request(tx *sqlx.Tx) error {
	return s.config.SetConfigValueFor(restartGenerationKey, uuid.NewString(), "backend", tx)
}
func (s *RuntimeRestart) CheckTransaction(tx *sqlx.Tx) error {
	current, err := s.current(tx)
	if err != nil {
		return err
	}
	if current != s.loaded {
		return ErrRestartPending
	}
	return nil
}
func (s *RuntimeRestart) Check(ctx context.Context) error {
	starter, ok := s.db.(interface {
		BeginTxx(context.Context, *sql.TxOptions) (*sqlx.Tx, error)
	})
	if !ok {
		return errors.New("database requires context-aware transactions")
	}
	tx, err := starter.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	return s.CheckTransaction(tx)
}
