package sqleak

import (
	"database/sql/driver"
	"time"
)

var (
	_ driver.Driver        = (*monitoredDriver)(nil)
	_ driver.DriverContext = (*monitoredDriver)(nil)
)

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

	return newMonitoredConn(conn, d.timeout), nil
}

func (d *monitoredDriver) OpenConnector(name string) (driver.Connector, error) {
	connector, err := d.driver.(driver.DriverContext).OpenConnector(name)
	if err != nil {
		return nil, err
	}

	return newMonitoredConnector(connector, d), nil
}
