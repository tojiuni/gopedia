package api

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"log/slog"
	"strings"
	"sync"
	"time"

	"github.com/go-fuego/fuego"
	"github.com/google/uuid"

	"gopedia/internal/runner"
)

type ingestJob struct {
	ID        string
	Status    string // queued, running, completed, failed
	Path      string
	CreatedAt time.Time
	StartedAt *time.Time
	EndedAt   *time.Time
	Result    map[string]any
	Failure   *APIError
}

type jobRegistry struct {
	mu          sync.Mutex
	jobs        map[string]*ingestJob
	idempotency map[string]string // key -> jobID
}

func newJobRegistry() *jobRegistry {
	return &jobRegistry{
		jobs:        make(map[string]*ingestJob),
		idempotency: make(map[string]string),
	}
}

func idempotencyKeyForIngest(headerKey, path string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(path)))
	body := hex.EncodeToString(h[:8])
	if headerKey != "" {
		return "hk:" + headerKey
	}
	return "path:" + body
}

func registerJobRoutes(api *fuego.Server, py *runner.Runner, reg *jobRegistry) {
	fuego.Post(api, "/ingest/jobs", func(c fuego.ContextWithBody[IngestJobRequest]) (IngestJobCreateResponse, error) {
		reqID := newRequestID(c)
		body, err := c.Body()
		if err != nil {
			return IngestJobCreateResponse{}, fuego.BadRequestError{Title: "Bad Request", Detail: err.Error()}
		}
		path := strings.TrimSpace(body.Path)
		if path == "" {
			return IngestJobCreateResponse{}, fuego.BadRequestError{Title: "Bad Request", Detail: "json field path is required"}
		}
		idemHeader := strings.TrimSpace(c.Header("Idempotency-Key"))
		idem := idempotencyKeyForIngest(idemHeader, path)

		reg.mu.Lock()
		if existingID, ok := reg.idempotency[idem]; ok {
			j := reg.jobs[existingID]
			reg.mu.Unlock()
			if j != nil {
				return IngestJobCreateResponse{JobID: j.ID, Status: j.Status, RequestID: reqID}, nil
			}
		}
		id := uuid.NewString()
		j := &ingestJob{
			ID:        id,
			Status:    "queued",
			Path:      path,
			CreatedAt: time.Now().UTC(),
		}
		reg.jobs[id] = j
		reg.idempotency[idem] = id
		reg.mu.Unlock()

		go runIngestJob(context.Background(), reg, id, path, py, reqID)

		return IngestJobCreateResponse{JobID: id, Status: "queued", RequestID: reqID}, nil
	},
		fuego.OptionSummary("Create async ingest job (poll GET /api/jobs/{id})"),
	)

	fuego.Get(api, "/jobs/{id}", func(c fuego.ContextNoBody) (IngestJobStatusResponse, error) {
		reqID := newRequestID(c)
		id := strings.TrimSpace(c.PathParam("id"))
		if id == "" {
			return IngestJobStatusResponse{}, fuego.NotFoundError{}
		}
		reg.mu.Lock()
		j, ok := reg.jobs[id]
		reg.mu.Unlock()
		if !ok {
			return IngestJobStatusResponse{}, fuego.NotFoundError{}
		}
		out := IngestJobStatusResponse{
			JobID:     j.ID,
			Status:    j.Status,
			RequestID: reqID,
		}
		if !j.CreatedAt.IsZero() {
			out.CreatedAt = j.CreatedAt.Format(time.RFC3339Nano)
		}
		if j.StartedAt != nil {
			out.StartedAt = j.StartedAt.Format(time.RFC3339Nano)
		}
		if j.EndedAt != nil {
			out.EndedAt = j.EndedAt.Format(time.RFC3339Nano)
		}
		if len(j.Result) > 0 {
			out.Result = j.Result
		}
		if j.Failure != nil {
			f := *j.Failure
			if f.RequestID == "" {
				f.RequestID = reqID
			}
			out.Failure = &f
		}
		return out, nil
	},
		fuego.OptionSummary("Poll ingest job status"),
	)
}

func runIngestJob(ctx context.Context, reg *jobRegistry, jobID, path string, py *runner.Runner, reqID string) {
	reg.mu.Lock()
	j, ok := reg.jobs[jobID]
	if !ok {
		reg.mu.Unlock()
		return
	}
	now := time.Now().UTC()
	j.Status = "running"
	j.StartedAt = &now
	reg.mu.Unlock()

	runCtx, cancel := context.WithTimeout(ctx, 30*time.Minute)
	defer cancel()

	out, stderr, err := py.RunModule(runCtx, "property.root_props.run", path)
	end := time.Now().UTC()

	reg.mu.Lock()
	j, ok = reg.jobs[jobID]
	if !ok {
		reg.mu.Unlock()
		return
	}
	j.EndedAt = &end
	if err != nil {
		j.Status = "failed"
		j.Failure = &APIError{
			Code:      "PYTHON_INGEST_FAILED",
			Message:   err.Error(),
			Details:   map[string]any{"stderr": string(stderr)},
			Retryable: false,
			RequestID: reqID,
		}
		slog.Warn("ingest job failed", "job_id", jobID, "request_id", reqID, "err", err, "stderr", string(stderr))
	} else {
		j.Status = "completed"
		j.Result = map[string]any{
			"stdout": string(out),
			"stderr": string(stderr),
		}
		slog.Info("ingest job completed", "job_id", jobID, "request_id", reqID, "path", j.Path)
	}
	reg.mu.Unlock()
}
