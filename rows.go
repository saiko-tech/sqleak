package sqleak

import (
	"database/sql/driver"
	"io"
	"time"
)

var (
	_ driver.Rows                           = (*monitoredRows)(nil)
	_ driver.RowsNextResultSet              = (*monitoredRows)(nil)
	_ driver.RowsColumnTypeDatabaseTypeName = (*monitoredRows)(nil)
	_ driver.RowsColumnTypeLength           = (*monitoredRows)(nil)
	_ driver.RowsColumnTypeNullable         = (*monitoredRows)(nil)
	_ driver.RowsColumnTypePrecisionScale   = (*monitoredRows)(nil)
)

type monitoredRows struct {
	driver.Rows
	monitor *monitor
}

func newMonitoredRows(rows driver.Rows, timeout time.Duration) *monitoredRows {
	return &monitoredRows{
		Rows:    rows,
		monitor: newMonitor(timeout, "Rows"),
	}
}

func (r *monitoredRows) ColumnTypePrecisionScale(index int) (precision, scale int64, ok bool) {
	if v, ok := r.Rows.(driver.RowsColumnTypePrecisionScale); ok {
		return v.ColumnTypePrecisionScale(index)
	}

	return 0, 0, false
}

func (r *monitoredRows) ColumnTypeNullable(index int) (nullable, ok bool) {
	if v, ok := r.Rows.(driver.RowsColumnTypeNullable); ok {
		return v.ColumnTypeNullable(index)
	}

	return false, false
}

func (r *monitoredRows) ColumnTypeLength(index int) (length int64, ok bool) {
	if v, ok := r.Rows.(driver.RowsColumnTypeLength); ok {
		return v.ColumnTypeLength(index)
	}

	return 0, false
}

func (r *monitoredRows) ColumnTypeDatabaseTypeName(index int) string {
	if v, ok := r.Rows.(driver.RowsColumnTypeDatabaseTypeName); ok {
		return v.ColumnTypeDatabaseTypeName(index)
	}

	return ""
}

func (r *monitoredRows) HasNextResultSet() bool {
	if v, ok := r.Rows.(driver.RowsNextResultSet); ok {
		return v.HasNextResultSet()
	}

	return false
}

func (r *monitoredRows) NextResultSet() error {
	if v, ok := r.Rows.(driver.RowsNextResultSet); ok {
		return v.NextResultSet()
	}

	return io.EOF
}

func (r *monitoredRows) Close() error {
	r.monitor.markClosed()

	return r.Rows.Close()
}

func (r *monitoredRows) Next(dest []driver.Value) (err error) {
	return r.Rows.Next(dest)
}
