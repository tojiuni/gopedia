package env

import (
	"os"
	"strings"
)

// PostgresConnString builds a postgres:// URL for pgx when Rhizome/Phloem should use Postgres.
// Returns empty string when POSTGRES_USER is unset (opt-out: no DB).
//
// If POSTGRES_HOST is empty but POSTGRES_USER is set, host defaults to 127.0.0.1 so local
// CLI runs (without Docker service DNS names) still connect when .env has user/password/db.
//
// Docker Compose service names (e.g. postgres_db) are not resolvable on the host; when not
// running inside a container, those names are mapped to 127.0.0.1 so the published port works.
// Override with GOPEDIA_POSTGRES_HOST if you need a different dial host.
func PostgresConnString() string {
	user := strings.TrimSpace(os.Getenv("POSTGRES_USER"))
	if user == "" {
		return ""
	}
	host := dialPostgresHost()
	port := getenvDefault("POSTGRES_PORT", "5432")
	pass := os.Getenv("POSTGRES_PASSWORD")
	db := getenvDefault("POSTGRES_DB", "gopedia")
	sslmode := getenvDefault("POSTGRES_SSLMODE", "disable")
	return "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + db + "?sslmode=" + sslmode
}

// PostgresDialHost returns the TCP host used in PostgresConnString (after remapping).
func PostgresDialHost() string {
	return dialPostgresHost()
}

func dialPostgresHost() string {
	if v := strings.TrimSpace(os.Getenv("GOPEDIA_POSTGRES_HOST")); v != "" {
		return v
	}
	host := strings.TrimSpace(os.Getenv("POSTGRES_HOST"))
	if host == "" {
		return "127.0.0.1"
	}
	return RemapComposeDialHost(host)
}

func getenvDefault(key, def string) string {
	if v := strings.TrimSpace(os.Getenv(key)); v != "" {
		return v
	}
	return def
}
