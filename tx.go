package sqleak

import (
	"database/sql/driver"
	"time"
)

var _ driver.Tx = (*monitoredTx)(nil)

type monitoredTx struct {
	driver.Tx
	monitor *monitor
}

func newMonitoredTx(tx driver.Tx, timeout time.Duration) *monitoredTx {
	return &monitoredTx{
		Tx:      tx,
		monitor: newMonitor(timeout, "Tx"),
	}
}

func (mt *monitoredTx) Commit() error {
	mt.monitor.markClosed()

	return mt.Tx.Commit()
}

func (mt *monitoredTx) Rollback() error {
	mt.monitor.markClosed()

	return mt.Tx.Rollback()
}
