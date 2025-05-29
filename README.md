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
