package migrate

import (
	"context"
	"os"
	"testing"
	"testing/fstest"

	"github.com/zedeus/wikifyp/server/internal/testdb"
)

func TestMain(m *testing.M) {
	os.Exit(testdb.Main(m))
}

func file(body string) *fstest.MapFile {
	return &fstest.MapFile{Data: []byte(body)}
}

func TestApplyFSRunsMigrationsInOrder(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()

	// Multi-statement files, and 0002 depends on 0001's table existing.
	fsys := fstest.MapFS{
		"0001_create.sql": file(`
			CREATE TABLE things (id int PRIMARY KEY, name text);
			CREATE INDEX things_name ON things (name);
		`),
		"0002_seed.sql": file(`INSERT INTO things (id, name) VALUES (1, 'first');`),
	}
	if err := ApplyFS(ctx, db, fsys); err != nil {
		t.Fatalf("ApplyFS: %v", err)
	}

	var name string
	if err := db.QueryRow("SELECT name FROM things WHERE id = 1").Scan(&name); err != nil {
		t.Fatalf("read seeded row: %v", err)
	}
	if name != "first" {
		t.Errorf("seeded name = %q, want %q", name, "first")
	}

	var versions int
	if err := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&versions); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if versions != 2 {
		t.Errorf("schema_migrations has %d rows, want 2", versions)
	}
}

func TestApplyFSIsIdempotent(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()

	fsys := fstest.MapFS{
		"0001_create.sql": file(`CREATE TABLE once (id int);`),
	}
	for i := 0; i < 3; i++ {
		if err := ApplyFS(ctx, db, fsys); err != nil {
			t.Fatalf("ApplyFS run %d: %v", i+1, err)
		}
	}
}

func TestApplyFSAppliesOnlyPending(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()

	first := fstest.MapFS{
		"0001_create.sql": file(`CREATE TABLE grows (id int);`),
	}
	if err := ApplyFS(ctx, db, first); err != nil {
		t.Fatalf("ApplyFS first: %v", err)
	}

	both := fstest.MapFS{
		"0001_create.sql": file(`CREATE TABLE grows (id int);`), // would fail if re-run
		"0002_column.sql": file(`ALTER TABLE grows ADD COLUMN name text;`),
	}
	if err := ApplyFS(ctx, db, both); err != nil {
		t.Fatalf("ApplyFS second: %v", err)
	}

	if _, err := db.Exec("SELECT name FROM grows"); err != nil {
		t.Fatalf("new column missing: %v", err)
	}
}

func TestApplyFSFailureRollsBack(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()

	fsys := fstest.MapFS{
		"0001_broken.sql": file(`
			CREATE TABLE half (id int);
			THIS IS NOT SQL;
		`),
	}
	if err := ApplyFS(ctx, db, fsys); err == nil {
		t.Fatal("ApplyFS succeeded on broken SQL")
	}

	// Neither the table nor the version record may survive.
	var tables int
	if err := db.QueryRow(
		"SELECT count(*) FROM information_schema.tables WHERE table_name = 'half'",
	).Scan(&tables); err != nil {
		t.Fatalf("count tables: %v", err)
	}
	if tables != 0 {
		t.Error("half-applied migration left its table behind")
	}
	var versions int
	if err := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&versions); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if versions != 0 {
		t.Error("failed migration was recorded as applied")
	}
}

// TestApplyFSVersionIsTheNumericPrefix checks that renaming a migration's
// description does not re-run it: the version is the number, not the name.
func TestApplyFSVersionIsTheNumericPrefix(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()

	if err := ApplyFS(ctx, db, fstest.MapFS{
		"0001_create.sql": file(`CREATE TABLE renamed (id int);`),
	}); err != nil {
		t.Fatalf("ApplyFS: %v", err)
	}
	// Same version, new description, incompatible body: must be skipped.
	if err := ApplyFS(ctx, db, fstest.MapFS{
		"0001_create_table.sql": file(`CREATE TABLE renamed (id int);`),
	}); err != nil {
		t.Fatalf("ApplyFS after rename: %v", err)
	}
}

func TestApplyFSOrdersByVersionNotName(t *testing.T) {
	db := testdb.New(t)

	// Lexically "10" < "2", numerically 2 < 10; 10 depends on 2's table.
	fsys := fstest.MapFS{
		"10_seed.sql":  file(`INSERT INTO ordered (id) VALUES (1);`),
		"2_create.sql": file(`CREATE TABLE ordered (id int);`),
	}
	if err := ApplyFS(context.Background(), db, fsys); err != nil {
		t.Fatalf("ApplyFS: %v", err)
	}
}

func TestApplyFSRejectsBadNames(t *testing.T) {
	db := testdb.New(t)
	ctx := context.Background()

	bad := []fstest.MapFS{
		{"setup.sql": file(`SELECT 1;`)},                                  // no version
		{"x_setup.sql": file(`SELECT 1;`)},                                // non-numeric
		{"0001_a.sql": file(`SELECT 1;`), "001_b.sql": file(`SELECT 1;`)}, // duplicate version
	}
	for _, fsys := range bad {
		if err := ApplyFS(ctx, db, fsys); err == nil {
			t.Errorf("ApplyFS(%v) succeeded, want naming error", fsys)
		}
	}
}

func TestApplyEmbedded(t *testing.T) {
	db := testdb.New(t)
	if err := Apply(context.Background(), db); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var versions int
	if err := db.QueryRow("SELECT count(*) FROM schema_migrations").Scan(&versions); err != nil {
		t.Fatalf("count schema_migrations: %v", err)
	}
	if versions == 0 {
		t.Error("no embedded migrations were applied")
	}
}
