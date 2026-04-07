package env

import (
	"log/slog"
	"os"
	"path/filepath"

	"github.com/joho/godotenv"
)

// LoadDotenv loads the first `.env` file found in the working directory or a parent directory.
// It does not override variables already set in the process environment.
func LoadDotenv() {
	dir, err := os.Getwd()
	if err != nil {
		return
	}
	for range 12 {
		path := filepath.Join(dir, ".env")
		if fi, statErr := os.Stat(path); statErr == nil && !fi.IsDir() {
			if loadErr := godotenv.Load(path); loadErr != nil {
				slog.Warn("dotenv: load failed", "path", path, "err", loadErr)
			} else {
				slog.Info("dotenv: loaded", "path", path)
			}
			return
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
}
