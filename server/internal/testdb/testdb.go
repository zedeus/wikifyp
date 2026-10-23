// Package testdb starts a throwaway PostgreSQL in Docker for a test package.
//
// Each test binary gets its own container, started on the first call to New
// and removed when the tests finish, so packages never share state. Each call
// to New creates a fresh database inside that container, so tests within a
// package are isolated from each other too.
//
// Packages using it must route teardown through Main:
//
//	func TestMain(m *testing.M) { os.Exit(testdb.Main(m)) }
//
// Docker is the only requirement; no running Postgres or network is needed.
package testdb

import (
	"context"
	"database/sql"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"testing"
	"time"

	_ "github.com/jackc/pgx/v5/stdlib"
)

const image = "postgres:17-alpine"

var shared struct {
	mu        sync.Mutex
	container string  // docker container ID, "" until started
	port      string  // host port mapped to 5432
	admin     *sql.DB // connection to the default database, used to create others
	counter   int
	startErr  error
}

// Main runs the package's tests and removes the shared container afterwards.
func Main(m *testing.M) int {
	code := m.Run()
	shared.mu.Lock()
	defer shared.mu.Unlock()
	if shared.admin != nil {
		shared.admin.Close()
	}
	if shared.container != "" {
		exec.Command("docker", "rm", "-f", shared.container).Run()
	}
	return code
}

// New returns an open connection to a fresh, empty database inside the
// package's throwaway Postgres container, starting the container if needed.
// The connection is closed via t.Cleanup; the database itself is discarded
// with the container.
func New(t testing.TB) *sql.DB {
	t.Helper()
	// The lock covers container startup, the counter and CREATE DATABASE
	// (Postgres refuses concurrent creation from the same template); the
	// open and ping below run unlocked so parallel tests don't serialize
	// on them.
	shared.mu.Lock()
	if shared.container == "" && shared.startErr == nil {
		shared.startErr = start()
	}
	if shared.startErr != nil {
		startErr := shared.startErr
		shared.mu.Unlock()
		t.Fatalf("testdb: %v (tests need Docker to start a throwaway Postgres)", startErr)
	}
	shared.counter++
	name := fmt.Sprintf("test_%d", shared.counter)
	_, createErr := shared.admin.Exec("CREATE DATABASE " + name)
	shared.mu.Unlock()

	if createErr != nil {
		t.Fatalf("testdb: create database %s: %v", name, createErr)
	}

	db, err := sql.Open("pgx", dsn(name))
	if err != nil {
		t.Fatalf("testdb: open %s: %v", name, err)
	}
	t.Cleanup(func() { db.Close() })
	if err := db.Ping(); err != nil {
		t.Fatalf("testdb: ping %s: %v", name, err)
	}
	return db
}

// start launches the container and waits until Postgres accepts connections.
// Called with shared.mu held.
func start() error {
	cmd := exec.Command("docker", "run", "-d", "--rm",
		// The label makes any leaked container (a package skipping
		// testdb.Main) easy to find and prune.
		"--label", "wikifyp-testdb=1",
		"-e", "POSTGRES_USER=wikifyp",
		"-e", "POSTGRES_PASSWORD=wikifyp",
		"-p", "127.0.0.1:0:5432",
		image,
		// Durability off: the data dies with the container anyway.
		"-c", "fsync=off", "-c", "synchronous_commit=off", "-c", "full_page_writes=off",
	)
	// The container ID is the only thing on stdout; pull progress on a
	// machine without the image lands on stderr and must not pollute it.
	var stdout, stderr strings.Builder
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("docker run %s: %v: %s", image, err, strings.TrimSpace(stderr.String()))
	}
	shared.container = strings.TrimSpace(stdout.String())

	out, err := exec.Command("docker", "port", shared.container, "5432/tcp").Output()
	if err != nil {
		return fmt.Errorf("docker port: %w", err)
	}
	// Output looks like "127.0.0.1:49153" (possibly one line per address
	// family). Split on the last colon so an IPv6 address would not break.
	addr, _, _ := strings.Cut(strings.TrimSpace(string(out)), "\n")
	i := strings.LastIndexByte(addr, ':')
	if i < 0 {
		return fmt.Errorf("docker port: unexpected output %q", out)
	}
	shared.port = addr[i+1:]

	admin, err := sql.Open("pgx", dsn("wikifyp"))
	if err != nil {
		return fmt.Errorf("open admin connection: %w", err)
	}
	// Postgres restarts once during initdb, so pings fail transiently.
	ctx, cancel := context.WithTimeout(context.Background(), time.Minute)
	defer cancel()
	for {
		if err = admin.PingContext(ctx); err == nil {
			shared.admin = admin
			return nil
		}
		select {
		case <-ctx.Done():
			admin.Close()
			return fmt.Errorf("postgres did not become ready: %v", err)
		case <-time.After(100 * time.Millisecond):
		}
	}
}

func dsn(database string) string {
	return fmt.Sprintf("postgres://wikifyp:wikifyp@127.0.0.1:%s/%s?sslmode=disable", shared.port, database)
}
