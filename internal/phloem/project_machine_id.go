package phloem

import (
	"crypto/sha256"
	"path/filepath"
	"strings"
)

// ProjectMachineID returns a deterministic 64-bit identity for a project root path,
// in the same spirit as keyword_so (SHA-256 digest, first 8 bytes as int64).
// Stable across restarts so re-RegisterProject for the same path maps to the same machine_id.
func ProjectMachineID(rootPath string) int64 {
	root := filepath.Clean(strings.TrimSpace(rootPath))
	if root == "" || root == "." {
		return 0
	}
	sum := sha256.Sum256([]byte("gopedia:project:v1:" + root))
	id := int64(uint64(sum[0])<<56 | uint64(sum[1])<<48 | uint64(sum[2])<<40 | uint64(sum[3])<<32 |
		uint64(sum[4])<<24 | uint64(sum[5])<<16 | uint64(sum[6])<<8 | uint64(sum[7]))
	if id < 0 {
		id = -id
	}
	if id == 0 {
		return 1
	}
	return id
}
