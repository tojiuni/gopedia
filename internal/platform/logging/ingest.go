package logging

import (
	"os"
	"strings"
)

// IngestLog holds context used by ingest request logging.
type IngestLog struct {
	RequestID  string
	SourcePath string
	DurationMs int64
	Stdout     string
	Stderr     string
	Err        error
}

// LogIngest emits summary fields at info/warn and full payload at debug.
func (l *Logger) LogIngest(in IngestLog) {
	sourceKind := detectSourceKind(in.SourcePath)
	fileCount, docCount := summarizeIngestStdout(in.Stdout)

	args := []any{
		"request_id", in.RequestID,
		"source_path", strings.TrimSpace(in.SourcePath),
		"source_kind", sourceKind,
		"duration_ms", in.DurationMs,
		"stdout_len", len(in.Stdout),
		"stderr_len", len(in.Stderr),
		"ingested_file_count", fileCount,
		"ingested_doc_count", docCount,
	}

	if in.Err != nil {
		l.Warn("api request", append([]any{"event", "api.ingest.failed"}, append(args, "err", in.Err)...)...)
		l.Debug("api.ingest.failed.full", "request_id", in.RequestID, "source_path", strings.TrimSpace(in.SourcePath), "stdout", in.Stdout, "stderr", in.Stderr)
		return
	}

	l.Info("api request", append([]any{"event", "api.ingest.success"}, args...)...)
	l.Debug("api.ingest.success.full", "request_id", in.RequestID, "source_path", strings.TrimSpace(in.SourcePath), "stdout", in.Stdout, "stderr", in.Stderr)
}

func summarizeIngestStdout(stdout string) (fileCount int, docCount int) {
	for _, line := range strings.Split(stdout, "\n") {
		trimmed := strings.TrimSpace(line)
		if strings.HasPrefix(trimmed, "[md]") || strings.HasPrefix(trimmed, "[code]") {
			fileCount++
		}
		if strings.Contains(trimmed, "OK -> doc_id=") {
			docCount++
		}
	}
	return fileCount, docCount
}

func detectSourceKind(path string) string {
	trimmed := strings.TrimSpace(path)
	if trimmed == "" {
		return "unknown"
	}
	info, err := os.Stat(trimmed)
	if err != nil {
		return "unknown"
	}
	if info.IsDir() {
		return "dir"
	}
	if info.Mode().IsRegular() {
		return "file"
	}
	return "unknown"
}
