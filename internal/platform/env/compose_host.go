package env

import (
	"os"
	"strings"
)

// RemapComposeDialHost maps Docker Compose service hostnames to 127.0.0.1 when the process
// runs on the host machine. Those names only resolve on the compose bridge network.
//
// Override per integration with GOPEDIA_*_HOST env vars (see DialQdrantHost, DialTypeDBHost,
// and GOPEDIA_POSTGRES_HOST in postgres.go).
func RemapComposeDialHost(host string) string {
	host = strings.TrimSpace(host)
	if host == "" {
		return ""
	}
	if runningInsideContainer() {
		return host
	}
	switch strings.ToLower(host) {
	case "postgres_db", "postgres", "qdrant", "typedb":
		return "127.0.0.1"
	default:
		return host
	}
}

func runningInsideContainer() bool {
	_, err := os.Stat("/.dockerenv")
	return err == nil
}

// DialQdrantHost returns the host for Qdrant gRPC client connections.
func DialQdrantHost() string {
	if v := strings.TrimSpace(os.Getenv("GOPEDIA_QDRANT_HOST")); v != "" {
		return v
	}
	return RemapComposeDialHost(strings.TrimSpace(os.Getenv("QDRANT_HOST")))
}

// DialTypeDBHost returns the host for TypeDB TCP checks when TYPEDB_HOST is set.
// Empty if TYPEDB_HOST is unset (callers may default to 127.0.0.1 for health checks).
func DialTypeDBHost() string {
	if v := strings.TrimSpace(os.Getenv("GOPEDIA_TYPEDB_HOST")); v != "" {
		return v
	}
	h := strings.TrimSpace(os.Getenv("TYPEDB_HOST"))
	if h == "" {
		return ""
	}
	return RemapComposeDialHost(h)
}
