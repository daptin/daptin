package database

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"io"
	"reflect"
	"sync"

	"github.com/jmoiron/sqlx"
)

// Open keeps the transaction's cancellation context on the driver connection.
// Existing sqlx transaction helpers therefore inherit their owning operation's
// deadline even when they use Exec/Query instead of the Context variants.
func Open(name, dsn string) (*sqlx.DB, error) {
	original, err := sql.Open(name, dsn)
	if err != nil {
		return nil, err
	}
	underlying := original.Driver()
	_ = original.Close()
	wrapped := "daptin-context-" + name
	registration.Lock()
	if !registration.names[wrapped] {
		sql.Register(wrapped, contextDriver{underlying})
		registration.names[wrapped] = true
	}
	registration.Unlock()
	db, err := sql.Open(wrapped, dsn)
	if err != nil {
		return nil, err
	}
	return sqlx.NewDb(db, name), nil
}

var registration = struct {
	sync.Mutex
	names map[string]bool
}{names: map[string]bool{}}

type contextDriver struct{ driver.Driver }

func (d contextDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.Driver.Open(name)
	if err != nil {
		return nil, err
	}
	return &contextConn{Conn: conn}, nil
}

func (d contextDriver) OpenConnector(name string) (driver.Connector, error) {
	var connector driver.Connector
	if contextual, ok := d.Driver.(driver.DriverContext); ok {
		var err error
		connector, err = contextual.OpenConnector(name)
		if err != nil {
			return nil, err
		}
	} else {
		connector = directConnector{base: d.Driver, dsn: name}
	}
	return contextConnector{Connector: connector, owner: d}, nil
}

type directConnector struct {
	base driver.Driver
	dsn  string
}

func (c directConnector) Connect(ctx context.Context) (driver.Conn, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return c.base.Open(c.dsn)
}

func (c directConnector) Driver() driver.Driver { return c.base }

type contextConnector struct {
	driver.Connector
	owner contextDriver
}

func (c contextConnector) Driver() driver.Driver { return c.owner }
func (c contextConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}
	return &contextConn{Conn: conn}, nil
}

type contextConn struct {
	driver.Conn
	mu          sync.RWMutex
	transaction context.Context
}

func (c *contextConn) operation(ctx context.Context) (context.Context, func()) {
	c.mu.RLock()
	transaction := c.transaction
	c.mu.RUnlock()
	if transaction == nil {
		return ctx, func() {}
	}
	if ctx.Done() == nil {
		return transaction, func() {}
	}
	combined, cancel := context.WithCancel(ctx)
	stop := context.AfterFunc(transaction, cancel)
	return combined, func() { stop(); cancel() }
}
func (c *contextConn) clear() { c.mu.Lock(); c.transaction = nil; c.mu.Unlock() }
func (c *contextConn) Begin() (driver.Tx, error) {
	return c.BeginTx(context.Background(), driver.TxOptions{})
}
func (c *contextConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	tx, err := c.Conn.(driver.ConnBeginTx).BeginTx(ctx, opts)
	if err != nil {
		return nil, err
	}
	c.mu.Lock()
	c.transaction = ctx
	c.mu.Unlock()
	return &contextTx{Tx: tx, conn: c}, nil
}
func (c *contextConn) Prepare(query string) (driver.Stmt, error) {
	return c.PrepareContext(context.Background(), query)
}
func (c *contextConn) PrepareContext(ctx context.Context, query string) (driver.Stmt, error) {
	ctx, done := c.operation(ctx)
	defer done()
	stmt, err := c.Conn.(driver.ConnPrepareContext).PrepareContext(ctx, query)
	if err != nil {
		return nil, err
	}
	return &contextStmt{Stmt: stmt, conn: c}, nil
}
func (c *contextConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	ctx, done := c.operation(ctx)
	defer done()
	return c.Conn.(driver.ExecerContext).ExecContext(ctx, query, args)
}
func (c *contextConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	ctx, done := c.operation(ctx)
	rows, err := c.Conn.(driver.QueryerContext).QueryContext(ctx, query, args)
	if err != nil {
		done()
		return nil, err
	}
	return &contextRows{Rows: rows, done: done}, nil
}
func (c *contextConn) Ping(ctx context.Context) error { return c.Conn.(driver.Pinger).Ping(ctx) }
func (c *contextConn) ResetSession(ctx context.Context) error {
	c.clear()
	if reset, ok := c.Conn.(driver.SessionResetter); ok {
		return reset.ResetSession(ctx)
	}
	return nil
}
func (c *contextConn) IsValid() bool {
	if validator, ok := c.Conn.(driver.Validator); ok {
		return validator.IsValid()
	}
	return true
}
func (c *contextConn) CheckNamedValue(value *driver.NamedValue) error {
	if checker, ok := c.Conn.(driver.NamedValueChecker); ok {
		return checker.CheckNamedValue(value)
	}
	return driver.ErrSkip
}

type contextTx struct {
	driver.Tx
	conn *contextConn
}

func (t *contextTx) Commit() error   { defer t.conn.clear(); return t.Tx.Commit() }
func (t *contextTx) Rollback() error { defer t.conn.clear(); return t.Tx.Rollback() }

type contextStmt struct {
	driver.Stmt
	conn *contextConn
}

func (s *contextStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (driver.Result, error) {
	ctx, done := s.conn.operation(ctx)
	defer done()
	return s.Stmt.(driver.StmtExecContext).ExecContext(ctx, args)
}
func (s *contextStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (driver.Rows, error) {
	ctx, done := s.conn.operation(ctx)
	rows, err := s.Stmt.(driver.StmtQueryContext).QueryContext(ctx, args)
	if err != nil {
		done()
		return nil, err
	}
	return &contextRows{Rows: rows, done: done}, nil
}

type contextRows struct {
	driver.Rows
	done func()
}

func (r *contextRows) Close() error { defer r.done(); return r.Rows.Close() }
func (r *contextRows) ColumnTypeDatabaseTypeName(i int) string {
	if rows, ok := r.Rows.(driver.RowsColumnTypeDatabaseTypeName); ok {
		return rows.ColumnTypeDatabaseTypeName(i)
	}
	return ""
}
func (r *contextRows) ColumnTypeLength(i int) (int64, bool) {
	if rows, ok := r.Rows.(driver.RowsColumnTypeLength); ok {
		return rows.ColumnTypeLength(i)
	}
	return 0, false
}
func (r *contextRows) ColumnTypeNullable(i int) (bool, bool) {
	if rows, ok := r.Rows.(driver.RowsColumnTypeNullable); ok {
		return rows.ColumnTypeNullable(i)
	}
	return false, false
}
func (r *contextRows) ColumnTypePrecisionScale(i int) (int64, int64, bool) {
	if rows, ok := r.Rows.(driver.RowsColumnTypePrecisionScale); ok {
		return rows.ColumnTypePrecisionScale(i)
	}
	return 0, 0, false
}
func (r *contextRows) ColumnTypeScanType(i int) reflect.Type {
	if rows, ok := r.Rows.(driver.RowsColumnTypeScanType); ok {
		return rows.ColumnTypeScanType(i)
	}
	return reflect.TypeOf(new(interface{})).Elem()
}
func (r *contextRows) HasNextResultSet() bool {
	if rows, ok := r.Rows.(driver.RowsNextResultSet); ok {
		return rows.HasNextResultSet()
	}
	return false
}
func (r *contextRows) NextResultSet() error {
	if rows, ok := r.Rows.(driver.RowsNextResultSet); ok {
		return rows.NextResultSet()
	}
	return io.EOF
}
