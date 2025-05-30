package sqleak

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"errors"
	"time"
)

var (
	_ driver.Pinger             = (*monitoredConn)(nil)
	_ driver.Execer             = (*monitoredConn)(nil) // nolint
	_ driver.ExecerContext      = (*monitoredConn)(nil)
	_ driver.Queryer            = (*monitoredConn)(nil) // nolint
	_ driver.QueryerContext     = (*monitoredConn)(nil)
	_ driver.Conn               = (*monitoredConn)(nil)
	_ driver.ConnPrepareContext = (*monitoredConn)(nil)
	_ driver.ConnBeginTx        = (*monitoredConn)(nil)
	_ driver.SessionResetter    = (*monitoredConn)(nil)
	_ driver.NamedValueChecker  = (*monitoredConn)(nil)
)

type monitoredConn struct {
	driver.Conn
	timeout time.Duration
}

func newMonitoredConn(conn driver.Conn, timeout time.Duration) *monitoredConn {
	return &monitoredConn{
		Conn:    conn,
		timeout: timeout,
	}
}

func (mc *monitoredConn) Ping(ctx context.Context) (err error) {
	pinger, ok := mc.Conn.(driver.Pinger)
	if !ok {
		// Driver doesn't implement, nothing to do
		return nil
	}

	return pinger.Ping(ctx)
}

func (mc *monitoredConn) Exec(query string, args []driver.Value) (driver.Result, error) {
	execer, ok := mc.Conn.(driver.Execer) // nolint
	if !ok {
		return nil, driver.ErrSkip
	}

	return execer.Exec(query, args)
}

func (mc *monitoredConn) ExecContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Result, error) {
	execer, ok := mc.Conn.(driver.ExecerContext)
	if !ok {
		return nil, driver.ErrSkip
	}

	return execer.ExecContext(ctx, query, args)
}

func (mc *monitoredConn) Query(query string, args []driver.Value) (driver.Rows, error) {
	queryer, ok := mc.Conn.(driver.Queryer) // nolint
	if !ok {
		return nil, driver.ErrSkip
	}

	rows, err := queryer.Query(query, args)
	if err != nil {
		return nil, err
	}

	return newMonitoredRows(rows, mc.timeout), nil
}

func (mc *monitoredConn) QueryContext(ctx context.Context, query string, args []driver.NamedValue) (driver.Rows, error) {
	queryer, ok := mc.Conn.(driver.QueryerContext)
	if !ok {
		return nil, driver.ErrSkip
	}

	rows, err := queryer.QueryContext(ctx, query, args)
	if err != nil {
		return nil, err
	}

	return newMonitoredRows(rows, mc.timeout), nil
}

func (mc *monitoredConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := mc.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}

	return newMonitoredStmt(stmt, mc), nil
}

func (mc *monitoredConn) PrepareContext(ctx context.Context, query string) (stmt driver.Stmt, err error) {
	if preparer, ok := mc.Conn.(driver.ConnPrepareContext); ok {
		if stmt, err = preparer.PrepareContext(ctx, query); err != nil {
			return nil, err
		}
	} else {
		if stmt, err = mc.Conn.Prepare(query); err != nil {
			return nil, err
		}

		select {
		default:
		case <-ctx.Done():
			stmt.Close()
			return nil, ctx.Err()
		}
	}

	return newMonitoredStmt(stmt, mc), nil
}

func (mc *monitoredConn) Begin() (driver.Tx, error) {
	tx, err := mc.Conn.Begin()
	if err != nil {
		return nil, err
	}

	return newMonitoredTx(tx, mc.timeout), nil
}

func (mc *monitoredConn) BeginTx(ctx context.Context, opts driver.TxOptions) (driver.Tx, error) {
	if ciCtx, is := mc.Conn.(driver.ConnBeginTx); is {
		tx, err := ciCtx.BeginTx(ctx, opts)
		if err != nil {
			return nil, err
		}

		return newMonitoredTx(tx, mc.timeout), nil
	}

	// Check the transaction level. If the transaction level is non-default
	// then return an error here as the BeginTx driver value is not supported.
	if opts.Isolation != driver.IsolationLevel(sql.LevelDefault) {
		return nil, errors.New("sql: driver does not support non-default isolation level")
	}

	// If a read-only transaction is requested return an error as the
	// BeginTx driver value is not supported.
	if opts.ReadOnly {
		return nil, errors.New("sql: driver does not support read-only transactions")
	}

	tx, err := mc.Conn.Begin()
	if err != nil {
		return nil, err
	}

	return newMonitoredTx(tx, mc.timeout), nil
}

func (mc *monitoredConn) ResetSession(ctx context.Context) (err error) {
	sessionResetter, ok := mc.Conn.(driver.SessionResetter)
	if !ok {
		// Driver does not implement, there is nothing to do.
		return nil
	}

	return sessionResetter.ResetSession(ctx)
}

func (mc *monitoredConn) CheckNamedValue(namedValue *driver.NamedValue) error {
	namedValueChecker, ok := mc.Conn.(driver.NamedValueChecker)
	if !ok {
		return driver.ErrSkip
	}

	return namedValueChecker.CheckNamedValue(namedValue)
}

func (mc *monitoredConn) Raw() driver.Conn {
	return mc.Conn
}
