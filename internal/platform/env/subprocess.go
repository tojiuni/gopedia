package env

import (
	"os"
	"strings"
)

// AppendSubprocessHostOverrides returns a copy of base env with POSTGRES_HOST, QDRANT_HOST,
// and TYPEDB_HOST replaced so Python subprocesses (xylem, ingest) use the same dial addresses
// as the Go server when running on the host (Compose DNS names are not resolvable outside the bridge).
// Keys are removed only when a replacement is applied.
func AppendSubprocessHostOverrides(base []string) []string {
	out := base
	if PostgresConnString() != "" {
		out = removeEnvKeys(out, "POSTGRES_HOST")
		out = append(out, "POSTGRES_HOST="+PostgresDialHost())
	}
	if strings.TrimSpace(os.Getenv("QDRANT_HOST")) != "" {
		out = removeEnvKeys(out, "QDRANT_HOST")
		out = append(out, "QDRANT_HOST="+DialQdrantHost())
	}
	if strings.TrimSpace(os.Getenv("TYPEDB_HOST")) != "" {
		if h := DialTypeDBHost(); h != "" {
			out = removeEnvKeys(out, "TYPEDB_HOST")
			out = append(out, "TYPEDB_HOST="+h)
		}
	}
	return out
}

func removeEnvKeys(base []string, keys ...string) []string {
	drop := make(map[string]struct{}, len(keys))
	for _, k := range keys {
		drop[k] = struct{}{}
	}
	var out []string
	for _, e := range base {
		name := strings.SplitN(e, "=", 2)[0]
		if _, ok := drop[name]; ok {
			continue
		}
		out = append(out, e)
	}
	return out
}
