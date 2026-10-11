package testdb

import (
	"os"
	"testing"
)

func TestMain(m *testing.M) {
	os.Exit(Main(m))
}

func TestNewGivesWorkingDatabase(t *testing.T) {
	db := New(t)
	var one int
	if err := db.QueryRow("SELECT 1").Scan(&one); err != nil {
		t.Fatalf("SELECT 1: %v", err)
	}
	if one != 1 {
		t.Fatalf("SELECT 1 = %d", one)
	}
}

// TestDatabasesAreIsolated checks that two calls to New return databases that
// cannot see each other's tables. Package-level isolation follows from each
// test binary starting its own container.
func TestDatabasesAreIsolated(t *testing.T) {
	first := New(t)
	second := New(t)

	if _, err := first.Exec("CREATE TABLE only_here (id int)"); err != nil {
		t.Fatalf("create table: %v", err)
	}
	var count int
	err := second.QueryRow(
		"SELECT count(*) FROM information_schema.tables WHERE table_name = 'only_here'",
	).Scan(&count)
	if err != nil {
		t.Fatalf("query second database: %v", err)
	}
	if count != 0 {
		t.Fatalf("table created in first database is visible in second")
	}
}
