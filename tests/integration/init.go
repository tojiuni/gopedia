// Package integration provides DB initialization helpers for tests.
// PostgreSQL: documents table; Qdrant: collection. TypeDB is initialized via Python (typedb_init.py).
package integration

import (
	"context"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qdrant/go-client/qdrant"

	"gopedia/core/ontology_so"
)

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

// InitPostgres runs the DDL at ddlPath (e.g. core/ontology_so/postgres_ddl.sql) against the given connStr.
// Creates documents table if not exists. Returns error on failure.
func InitPostgres(ctx context.Context, connStr string, ddlPath string) error {
	ddl, err := os.ReadFile(ddlPath)
	if err != nil {
		return err
	}
	pool, err := pgxpool.New(ctx, connStr)
	if err != nil {
		return err
	}
	defer pool.Close()
	_, err = pool.Exec(ctx, string(ddl))
	return err
}

// InitQdrant ensures the collection exists; creates it if not. Uses ontologyso.EnsureQdrantCollection.
func InitQdrant(ctx context.Context, host string, port int, collection string, vectorSize uint64) error {
	if vectorSize == 0 {
		vectorSize = ontologyso.DefaultVectorSize
	}
	client, err := qdrant.NewClient(&qdrant.Config{Host: host, Port: port})
	if err != nil {
		return err
	}
	return ontologyso.EnsureQdrantCollection(ctx, client, collection, vectorSize)
}

// InitPostgresFromEnv initializes Postgres using env POSTGRES_*. ddlPath is path to postgres_ddl.sql (relative to repo root).
// If repoRoot is empty, it is auto-detected by walking up from the current directory to find core/ontology_so/postgres_ddl.sql.
func InitPostgresFromEnv(ctx context.Context, repoRoot string, ddlPath string) error {
	host := getEnv("POSTGRES_HOST", "")
	user := getEnv("POSTGRES_USER", "")
	if host == "" || user == "" {
		return nil
	}
	if repoRoot == "" || repoRoot == "." {
		if r, err := findRepoRoot(ddlPath); err == nil {
			repoRoot = r
		}
	}
	port := getEnv("POSTGRES_PORT", "5432")
	pass := getEnv("POSTGRES_PASSWORD", "")
	db := getEnv("POSTGRES_DB", "gopedia")
	sslmode := getEnv("POSTGRES_SSLMODE", "disable")
	connStr := "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + db + "?sslmode=" + sslmode
	path := filepath.Join(repoRoot, ddlPath)
	return InitPostgres(ctx, connStr, path)
}

// findRepoRoot walks up from the current directory to find a directory containing the given relPath (e.g. core/ontology_so/postgres_ddl.sql).
func findRepoRoot(relPath string) (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		p := filepath.Join(dir, relPath)
		if _, err := os.Stat(p); err == nil {
			return dir, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}

// InitQdrantFromEnv initializes Qdrant using env QDRANT_*.
func InitQdrantFromEnv(ctx context.Context) error {
	host := getEnv("QDRANT_HOST", "")
	if host == "" {
		return nil
	}
	port := 6334
	if p, err := strconv.Atoi(getEnv("QDRANT_GRPC_PORT", getEnv("QDRANT_PORT", "6334"))); err == nil {
		port = p
	}
	collection := getEnv("QDRANT_COLLECTION", "gopedia_markdown")
	return InitQdrant(ctx, host, port, collection, ontologyso.DefaultVectorSize)
}

// InitAllFromEnv runs InitPostgresFromEnv and InitQdrantFromEnv. repoRoot is the repo root path.
func InitAllFromEnv(ctx context.Context, repoRoot string) error {
	if ctx == nil {
		ctx = context.Background()
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	if err := InitPostgresFromEnv(ctx, repoRoot, "core/ontology_so/postgres_ddl.sql"); err != nil {
		return err
	}
	return InitQdrantFromEnv(ctx)
}
