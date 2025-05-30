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
	timeout  time.Duration
	stack    []byte
	closed   bool
	resource string
}

func (m *monitor) markClosed() {
	m.closed = true
}

func newMonitor(timeout time.Duration, resource string) *monitor {
	buf := stackPool.Get().(*[]byte)

	n := runtime.Stack(*buf, false)

	mon := &monitor{
		timeout:  timeout,
		stack:    (*buf)[:n],
		closed:   false,
		resource: resource,
	}

	time.AfterFunc(mon.timeout, func() {
		if !mon.closed {
			log.Printf("likely resource leak detected: %s not closed within %s after opening:\n%s", mon.resource, mon.timeout, string(mon.stack))
		}

		stackPool.Put(&mon.stack)
	})

	return mon
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
