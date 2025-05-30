package sqleak_test

import (
	"log"
	"strings"
	"testing"
	"time"

	_ "github.com/mattn/go-sqlite3"

	"github.com/saiko-tech/sqleak"
)

func TestConnectionLeakDetection(t *testing.T) {
	var logOutput strings.Builder
	log.SetOutput(&logOutput)
	defer log.SetOutput(nil) // reset after test

	db, err := sqleak.Open("sqlite3", ":memory:",
		sqleak.WithTimeout(100*time.Millisecond), // set low timeout for test
	)
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	defer db.Close()

	// Create a table
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	// Intentionally don't close rows to simulate a leak
	rows, err := db.Query("SELECT name FROM test")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Let timeout elapse
	time.Sleep(200 * time.Millisecond)

	if !strings.Contains(logOutput.String(), "likely resource leak detected") {
		t.Error("expected leak warning in log but didn't find one")
	}

	// Now close the rows to clean up
	_ = rows.Close()
}

func TestProperClosePreventsLeakWarning(t *testing.T) {
	var logOutput strings.Builder
	log.SetOutput(&logOutput)
	defer log.SetOutput(nil) // reset after test

	db, err := sqleak.Open("sqlite3", ":memory:",
		sqleak.WithTimeout(100*time.Millisecond),
	)
	if err != nil {
		t.Fatalf("failed to open DB: %v", err)
	}
	defer db.Close()

	// Create a table
	_, err = db.Exec("CREATE TABLE test (id INTEGER PRIMARY KEY, name TEXT)")
	if err != nil {
		t.Fatalf("failed to create table: %v", err)
	}

	rows, err := db.Query("SELECT name FROM test")
	if err != nil {
		t.Fatalf("query failed: %v", err)
	}

	// Properly close to avoid leak
	_ = rows.Close()

	// Wait to ensure monitor would have triggered
	time.Sleep(200 * time.Millisecond)

	if strings.Contains(logOutput.String(), "likely resource leak detected") {
		t.Error("did not expect leak warning, but found one:\n", logOutput.String())
	}
}

/*
Example demonstrates how to use sqleak to monitor for connection leaks.
It intentionally leaks a connection by not closing the rows,
which will trigger a warning log message after the timeout.
*/
func Example() {
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

func TestExample(t *testing.T) {
	var logOutput strings.Builder
	log.SetOutput(&logOutput)
	defer log.SetOutput(nil) // reset after test

	// Run the example function to see if it logs a leak warning
	Example()

	// Check if the log contains the expected leak warning
	if !strings.Contains(logOutput.String(), "likely resource leak detected") {
		t.Error("expected leak warning in log but didn't find one")
	}

	t.Logf("Log output: %s", logOutput.String())
}
