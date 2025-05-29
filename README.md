# sqleak - Go SQL Driver to Detect Resource Leaks

sqleak is a Go SQL driver that wraps around existing SQL drivers to detect resource leaks such as unclosed statements, rows and transactions. It provides a way to ensure that all resources are properly closed, helping to prevent memory leaks and exhausted connection pools in applications.

## Go Get

    go get github.com/saiko-tech/sqleak

## Features

- Detects unclosed resources
  - statements
  - rows
  - transactions
  - :information_source: connections are not tracked as they may be long-lived
- Logs warnings with stack traces if resources are not closed within a specified timeout

## Example

```go
package main

import (
    "log"
    "time"

    "github.com/saiko-tech/sqleak"
    _ "github.com/saiko-tech/sqleak/sqlite3" // replace with your SQL driver
)

func main() {
	db, err := sqleak.Open("sqlite3", ":memory:",
		sqleak.WithTimeout(100*time.Millisecond), // set low timeout for demonstration
	)
	if err != nil {
		log.Fatalf("failed to open DB: %v", err)
	}
	defer db.Close()

	_, err = db.Exec("CREATE TABLE example (id INTEGER PRIMARY KEY, value TEXT)")
	if err != nil {
		log.Fatalf("failed to create table: %v", err)
	}

	// Intentionally leak a connection by not closing rows
	rows, err := db.Query("SELECT value FROM example")
	if err != nil {
		log.Fatalf("query failed: %v", err)
	}

	// Wait to trigger leak detection, triggering a warn log message
	time.Sleep(200 * time.Millisecond)

	// Clean up
	_ = rows.Close()
}
```

The above example will print something like the following:

```
2025/05/29 16:19:31 likely resource leak detected: Stmt not closed within 100ms after opening:
<stack trace>
2025/05/29 16:19:31 likely resource leak detected: Rows not closed within 100ms after opening:
<stack trace>
```

As you can see it notifies about both the unclosed statement and rows, along with a stack trace to help identify where the leak originated.

The full output will look like this:
```
2025/05/29 16:19:31 likely resource leak detected: Stmt not closed within 100ms after opening:
goroutine 6 [running]:
github.com/saiko-tech/sqleak.newMonitor(0x5f5e100, {0x6b0ca8, 0x4})
	/home/markus/dev/saiko-tech/sqleak/sqleak.go:47 +0x57
github.com/saiko-tech/sqleak.(*monitoredConn).Prepare(0xc0000100a8, {0x6b6524?, 0x688878?})
	/home/markus/dev/saiko-tech/sqleak/sqleak.go:133 +0x56
database/sql.ctxDriverPrepare({0x6f4fb0, 0x8671e0}, {0x6f4d98?, 0xc0000100a8?}, {0x6b6524?, 0x19?})
	/usr/lib/go/src/database/sql/ctxutil.go:17 +0x62
database/sql.(*DB).queryDC.func2()
	/usr/lib/go/src/database/sql/sql.go:1808 +0x45
database/sql.withLock({0x6f4bb0, 0xc000032180}, 0xc0000d9be8)
	/usr/lib/go/src/database/sql/sql.go:3574 +0x71
database/sql.(*DB).queryDC(0x1?, {0x6f4fb0, 0x8671e0}, {0x0, 0x0}, 0xc000032180, 0xc000026400, {0x6b6524, 0x19}, {0x0, ...})
	/usr/lib/go/src/database/sql/sql.go:1807 +0x2b1
database/sql.(*DB).query(0xc0000ae5b0, {0x6f4fb0, 0x8671e0}, {0x6b6524, 0x19}, {0x0, 0x0, 0x0}, 0xe8?)
	/usr/lib/go/src/database/sql/sql.go:1764 +0xfc
database/sql.(*DB).QueryContext.func1(0xd0?)
	/usr/lib/go/src/database/sql/sql.go:1742 +0x4f
database/sql.(*DB).retry(0xc0000d9e08?, 0xc0000d9e08)
	/usr/lib/go/src/database/sql/sql.go:1576 +0x42
database/sql.(*DB).QueryContext(0xc0000ae4e0?, {0x6f4fb0?, 0x8671e0?}, {0x6b6524?, 0x6b1657?}, {0x0?, 0x6f44c0?, 0xc000010090?})
	/usr/lib/go/src/database/sql/sql.go:1741 +0xc5
database/sql.(*DB).Query(0x6b12ac?, {0x6b6524?, 0x6b1657?}, {0x0?, 0xc000064740?, 0x1?})
	/usr/lib/go/src/database/sql/sql.go:1755 +0x3a
github.com/saiko-tech/sqleak_test.Example()
	/home/markus/dev/saiko-tech/sqleak/sqleak_test.go:105 +0x14f
github.com/saiko-tech/sqleak_test.TestRunExample(0xc0000e4380?)
	/home/markus/dev/saiko-tech/sqleak/sqleak_test.go:132 +0xf
testing.tRunner(0xc0000e4380, 0x6c01a8)
	/usr/lib/go/src/testing/testing.go:1792 +0xf4
created by testing.(*T).Run in goroutine 1
	/usr/lib/go/src/testing/testing.go:1851 +0x413
2025/05/29 16:19:31 likely resource leak detected: Rows not closed within 100ms after opening:
goroutine 6 [running]:
github.com/saiko-tech/sqleak.newMonitor(0x5f5e100, {0x6b0ca4, 0x4})
	/home/markus/dev/saiko-tech/sqleak/sqleak.go:47 +0x57
github.com/saiko-tech/sqleak.(*monitoredStmt).Query(0xc000010108, {0x8671e0?, 0xc0000d9a68?, 0x5356b5?})
	/home/markus/dev/saiko-tech/sqleak/sqleak.go:97 +0x59
database/sql.ctxDriverStmtQuery({0x6f4fb0, 0x8671e0}, {0x6f5020, 0xc000010108}, {0x8671e0, 0x0, 0x0?})
	/usr/lib/go/src/database/sql/ctxutil.go:94 +0x1ca
database/sql.rowsiFromStatement({0x6f4fb0, 0x8671e0}, {0x6f4d98, 0xc0000100a8}, 0xc000090b80, {0x0, 0x0, 0x0})
	/usr/lib/go/src/database/sql/sql.go:2848 +0x14f
database/sql.(*DB).queryDC(0x1?, {0x6f4fb0, 0x8671e0}, {0x0, 0x0}, 0xc000032180, 0xc000026400, {0x6b6524, 0x19}, {0x0, ...})
	/usr/lib/go/src/database/sql/sql.go:1816 +0x36c
database/sql.(*DB).query(0xc0000ae5b0, {0x6f4fb0, 0x8671e0}, {0x6b6524, 0x19}, {0x0, 0x0, 0x0}, 0xe8?)
	/usr/lib/go/src/database/sql/sql.go:1764 +0xfc
database/sql.(*DB).QueryContext.func1(0xd0?)
	/usr/lib/go/src/database/sql/sql.go:1742 +0x4f
database/sql.(*DB).retry(0xc0000d9e08?, 0xc0000d9e08)
	/usr/lib/go/src/database/sql/sql.go:1576 +0x42
database/sql.(*DB).QueryContext(0xc0000ae4e0?, {0x6f4fb0?, 0x8671e0?}, {0x6b6524?, 0x6b1657?}, {0x0?, 0x6f44c0?, 0xc000010090?})
	/usr/lib/go/src/database/sql/sql.go:1741 +0xc5
database/sql.(*DB).Query(0x6b12ac?, {0x6b6524?, 0x6b1657?}, {0x0?, 0xc000064740?, 0x1?})
	/usr/lib/go/src/database/sql/sql.go:1755 +0x3a
github.com/saiko-tech/sqleak_test.Example()
	/home/markus/dev/saiko-tech/sqleak/sqleak_test.go:105 +0x14f
github.com/saiko-tech/sqleak_test.TestRunExample(0xc0000e4380?)
	/home/markus/dev/saiko-tech/sqleak/sqleak_test.go:132 +0xf
testing.tRunner(0xc0000e4380, 0x6c01a8)
	/usr/lib/go/src/testing/testing.go:1792 +0xf4
created by testing.(*T).Run in goroutine 1
	/usr/lib/go/src/testing/testing.go:1851 +0x413
```