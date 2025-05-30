package sqleak

import (
	"context"
	"database/sql/driver"
	"io"
)

var _ driver.Connector = (*monitoredConnector)(nil)
var _ io.Closer = (*monitoredConnector)(nil)

type monitoredConnector struct {
	driver.Connector
	driver *monitoredDriver
}

func newMonitoredConnector(connector driver.Connector, driver *monitoredDriver) *monitoredConnector {
	return &monitoredConnector{
		Connector: connector,
		driver:    driver,
	}
}

func (c *monitoredConnector) Connect(ctx context.Context) (driver.Conn, error) {
	conn, err := c.Connector.Connect(ctx)
	if err != nil {
		return nil, err
	}

	return newMonitoredConn(conn, c.driver.timeout), nil
}

func (c *monitoredConnector) Driver() driver.Driver {
	return c.driver
}

func (c *monitoredConnector) Close() error {
	// database/sql uses a type assertion to check if connectors implement io.Closer.
	// The type assertion does not pass through to monitoredConnector.Connector, so we explicitly implement it here.
	if closer, ok := c.Connector.(interface{ Close() error }); ok {
		return closer.Close()
	}
	return nil
}
