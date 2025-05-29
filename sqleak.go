package sqleak

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"log"
	"runtime"
	"sync"
	"time"
)

type dsnConnector struct {
	dsn    string
	driver driver.Driver
}

func (c dsnConnector) Connect(_ context.Context) (driver.Conn, error) {
	return c.driver.Open(c.dsn)
}

func (c dsnConnector) Driver() driver.Driver {
	return c.driver
}

var stackPool = sync.Pool{
	New: func() interface{} {
		buf := make([]byte, 8*1024)
		return &buf
	},
}

type monitor struct {
	timeout time.Duration
	stack   []byte
	closed  bool
}

func (m *monitor) markClosed() {
	m.closed = true
}

func newMonitor(timeout time.Duration) *monitor {
	buf := stackPool.Get().(*[]byte)

	n := runtime.Stack(*buf, false)

	mon := &monitor{
		timeout: timeout,
		stack:   (*buf)[:n],
		closed:  false,
	}

	time.AfterFunc(mon.timeout, func() {
		if !mon.closed {
			log.Printf("likely connection leak detected: connection not closed within %s after opening:\n%s", mon.timeout, string(mon.stack))
		}

		stackPool.Put(&mon.stack)
	})

	return mon
}

type monitoredStmt struct {
	driver.Stmt
	monitor *monitor
}

func (ms *monitoredStmt) Close() error {
	ms.monitor.markClosed()

	return ms.Stmt.Close()
}

type monitoredRows struct {
	driver.Rows
	monitor *monitor
}

func (mr *monitoredRows) Close() error {
	mr.monitor.markClosed()

	return mr.Rows.Close()
}

func (ms *monitoredStmt) Query(args []driver.Value) (driver.Rows, error) {
	rows, err := ms.Stmt.Query(args)
	if err != nil {
		return nil, err
	}

	mRows := &monitoredRows{
		Rows:    rows,
		monitor: newMonitor(ms.monitor.timeout),
	}

	return mRows, nil
}

type monitoredTx struct {
	driver.Tx
	monitor *monitor
}

func (mt *monitoredTx) Commit() error {
	mt.monitor.markClosed()

	return mt.Tx.Commit()
}

func (mt *monitoredTx) Rollback() error {
	mt.monitor.markClosed()

	return mt.Tx.Rollback()
}

type monitoredConn struct {
	driver.Conn
	timeout time.Duration
}

func (mc *monitoredConn) Prepare(query string) (driver.Stmt, error) {
	stmt, err := mc.Conn.Prepare(query)
	if err != nil {
		return nil, err
	}

	mStmt := &monitoredStmt{
		Stmt:    stmt,
		monitor: newMonitor(mc.timeout),
	}

	return mStmt, nil
}

func (mc *monitoredConn) Begin() (driver.Tx, error) {
	tx, err := mc.Conn.Begin()
	if err != nil {
		return nil, err
	}

	mTx := &monitoredTx{
		Tx:      tx,
		monitor: newMonitor(mc.timeout),
	}

	return mTx, nil
}

type monitoredDriver struct {
	driver  driver.Driver
	timeout time.Duration
}

func newMonitoredDriver(d driver.Driver, timeout time.Duration) *monitoredDriver {
	if _, ok := d.(driver.DriverContext); ok {
		return &monitoredDriver{
			driver:  d,
			timeout: timeout,
		}
	}

	// Only implements driver.Driver
	return &monitoredDriver{
		driver:  struct{ driver.Driver }{d},
		timeout: timeout,
	}
}

func (d *monitoredDriver) Open(name string) (driver.Conn, error) {
	conn, err := d.driver.Open(name)
	if err != nil {
		return nil, err
	}

	mc := &monitoredConn{
		Conn:    conn,
		timeout: d.timeout,
	}

	return mc, err
}

func (d *monitoredDriver) OpenConnector(name string) (driver.Connector, error) {
	return d.driver.(driver.DriverContext).OpenConnector(name)
}

type Option func(*monitoredDriver)

func WithTimeout(timeout time.Duration) Option {
	return func(ld *monitoredDriver) {
		ld.timeout = timeout
	}
}

func WithDriverWrapper(f func(driver.Driver) driver.Driver) Option {
	return func(ld *monitoredDriver) {
		ld.driver = f(ld.driver)
	}
}

// Open is a wrapper over sql.Open with leak detection instrumentation.
func Open(driverName, dataSourceName string, opts ...Option) (*sql.DB, error) {
	db, err := sql.Open(driverName, dataSourceName)
	if err != nil {
		return nil, err
	}
	d := db.Driver()
	if err = db.Close(); err != nil {
		return nil, err
	}

	ld := newMonitoredDriver(d, 30*time.Second) // default timeout of 30 seconds, can be overridden by options

	for _, opt := range opts {
		opt(ld)
	}

	if _, ok := d.(driver.DriverContext); ok {
		connector, err := ld.OpenConnector(dataSourceName)
		if err != nil {
			return nil, err
		}
		return sql.OpenDB(connector), nil
	}

	return sql.OpenDB(dsnConnector{dsn: dataSourceName, driver: ld}), nil
}

func WrapDriver(d driver.Driver, opts ...Option) driver.Driver {
	ld := newMonitoredDriver(d, 30*time.Second) // default timeout of 30 seconds, can be overridden by options

	for _, opt := range opts {
		opt(ld)
	}

	return ld
}
