package sqleak

import (
	"context"
	"database/sql/driver"
	"errors"
)

var (
	_ driver.Stmt              = (*monitoredStmt)(nil)
	_ driver.StmtExecContext   = (*monitoredStmt)(nil)
	_ driver.StmtQueryContext  = (*monitoredStmt)(nil)
	_ driver.NamedValueChecker = (*monitoredStmt)(nil)
)

type monitoredStmt struct {
	driver.Stmt
	monitor       *monitor
	monitoredConn *monitoredConn
}

func newMonitoredStmt(stmt driver.Stmt, mc *monitoredConn) *monitoredStmt {
	return &monitoredStmt{
		Stmt:          stmt,
		monitor:       newMonitor(mc.timeout, "Stmt"),
		monitoredConn: mc,
	}
}

func (s *monitoredStmt) Close() error {
	s.monitor.markClosed()

	return s.Stmt.Close()
}

func (s *monitoredStmt) Query(args []driver.Value) (driver.Rows, error) {
	rows, err := s.Stmt.Query(args)
	if err != nil {
		return nil, err
	}

	return newMonitoredRows(rows, s.monitor.timeout), nil
}

// Copied from stdlib database/sql package: src/database/sql/ctxutil.go.
func namedValueToValue(named []driver.NamedValue) ([]driver.Value, error) {
	dargs := make([]driver.Value, len(named))
	for n, param := range named {
		if len(param.Name) > 0 {
			return nil, errors.New("sql: driver does not support the use of Named Parameters")
		}
		dargs[n] = param.Value
	}
	return dargs, nil
}

func (s *monitoredStmt) ExecContext(ctx context.Context, args []driver.NamedValue) (result driver.Result, err error) {
	if execer, ok := s.Stmt.(driver.StmtExecContext); ok {
		return execer.ExecContext(ctx, args)
	}

	// StmtExecContext.ExecContext is not permitted to return ErrSkip. fall back to Exec.
	var dargs []driver.Value
	if dargs, err = namedValueToValue(args); err != nil {
		return nil, err
	}

	select {
	default:
	case <-ctx.Done():
		return nil, ctx.Err()
	}

	return s.Stmt.Exec(dargs) //nolint:staticcheck
}

func (s *monitoredStmt) QueryContext(ctx context.Context, args []driver.NamedValue) (rows driver.Rows, err error) {
	if query, ok := s.Stmt.(driver.StmtQueryContext); ok {
		if rows, err = query.QueryContext(ctx, args); err != nil {
			return nil, err
		}
	} else {
		// StmtQueryContext.QueryContext is not permitted to return ErrSkip. fall back to Query.
		var dargs []driver.Value
		if dargs, err = namedValueToValue(args); err != nil {
			return nil, err
		}

		select {
		default:
		case <-ctx.Done():
			return nil, ctx.Err()
		}

		if rows, err = s.Stmt.Query(dargs); err != nil { //nolint:staticcheck
			return nil, err
		}
	}

	return newMonitoredRows(rows, s.monitor.timeout), nil
}

func (s *monitoredStmt) CheckNamedValue(namedValue *driver.NamedValue) error {
	namedValueChecker, ok := s.Stmt.(driver.NamedValueChecker)
	if !ok {
		// Fallback to the connection's named value checker.
		//
		// The [database/sql] package checks for value checkers in the following order,
		// stopping at the first found match: Stmt.NamedValueChecker, Conn.NamedValueChecker,
		// Stmt.ColumnConverter, [DefaultParameterConverter].
		//
		// Since otelsql implements the NamedValueChecker for both Stmt and Conn, the
		// fallback logic in the Go is not working.
		// Source: https://go.googlesource.com/go/+/refs/tags/go1.22.2/src/database/sql/convert.go#128
		//
		// This is a workaround to make sure the named value checker is checked on the connection level after
		// the statement level.
		return s.monitoredConn.CheckNamedValue(namedValue)
	}

	return namedValueChecker.CheckNamedValue(namedValue)
}
