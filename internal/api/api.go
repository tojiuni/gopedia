// Package api registers Fuego HTTP routes for Gopedia (ingest/search via Python subprocess).
package api

import (
	"context"
	"encoding/json"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-fuego/fuego"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/qdrant/go-client/qdrant"
	"github.com/redis/go-redis/v9"
	"google.golang.org/grpc"
	"google.golang.org/grpc/reflection"

	identityso "gopedia/core/identity_so"
	ontologyso "gopedia/core/ontology_so"
	pb "gopedia/core/proto/gen/go"
	"gopedia/internal/phloem"
	"gopedia/internal/phloem/chunker"
	"gopedia/internal/phloem/domain"
	"gopedia/internal/phloem/embedder"
	"gopedia/internal/phloem/sink"
	"gopedia/internal/phloem/toc"
	"gopedia/internal/platform/env"
	"gopedia/internal/platform/logging"
	"gopedia/internal/runner"
)

// IngestRequest is the JSON body for POST /api/ingest.
type IngestRequest struct {
	Path string `json:"path"`
}

// IngestResponse reports subprocess outcome.
type IngestResponse struct {
	OK        bool      `json:"ok"`
	Stdout    string    `json:"stdout,omitempty"`
	Stderr    string    `json:"stderr,omitempty"`
	Error     string    `json:"error,omitempty"`
	Failure   *APIError `json:"failure,omitempty"`
	RequestID string    `json:"request_id,omitempty"`
}

// SearchResponse is the JSON body for GET /api/search.
type SearchResponse struct {
	Markdown  string          `json:"markdown,omitempty"`
	Results   json.RawMessage `json:"results,omitempty"`
	Stderr    string          `json:"stderr,omitempty"`
	Error     string          `json:"error,omitempty"`
	Failure   *APIError       `json:"failure,omitempty"`
	RequestID string          `json:"request_id,omitempty"`
}

// HealthResponse is returned by GET /api/health.
type HealthResponse struct {
	Status string `json:"status"`
}

// Register mounts /api routes on the given Fuego server using Python subprocesses.
func Register(s *fuego.Server, py *runner.Runner) {
	reg := newJobRegistry()
	api := fuego.Group(s, "/api")

	fuego.Get(api, "/health", func(c fuego.ContextNoBody) (HealthResponse, error) {
		reqID := newRequestID(c)
		started := time.Now()
		slog.Info("api request", "event", "api.health.request", "request_id", reqID, "method", "GET", "path", "/api/health")
		defer func() {
			slog.Info("api request", "event", "api.health.success", "request_id", reqID, "method", "GET", "path", "/api/health", "status", 200, "latency_ms", time.Since(started).Milliseconds())
		}()
		return HealthResponse{Status: "ok"}, nil
	},
		fuego.OptionSummary("Liveness check"),
	)

	fuego.Get(api, "/health/deps", func(c fuego.ContextNoBody) (HealthDepsResponse, error) {
		reqID := newRequestID(c)
		started := time.Now()
		slog.Info("api request", "event", "api.health_deps.request", "request_id", reqID, "method", "GET", "path", "/api/health/deps")
		deps := map[string]DepStatus{
			"postgres": checkPostgres(),
			"qdrant":   checkQdrant(),
			"typedb":   checkTypeDB(),
			"phloem":   checkPhloemGRPC(),
		}
		overall := "ok"
		for _, d := range deps {
			if d.Status == "error" {
				overall = "degraded"
				break
			}
		}
		slog.Info(
			"api request",
			"event", "api.health_deps.success",
			"request_id", reqID,
			"method", "GET",
			"path", "/api/health/deps",
			"status", 200,
			"overall_status", overall,
			"latency_ms", time.Since(started).Milliseconds(),
		)
		return HealthDepsResponse{Status: overall, Deps: deps, RequestID: reqID}, nil
	},
		fuego.OptionSummary("Dependency health (Postgres, Qdrant, TypeDB, Phloem gRPC)"),
	)

	fuego.Get(api, "/search", func(c fuego.ContextNoBody) (SearchResponse, error) {
		reqID := newRequestID(c)
		started := time.Now()
		q := strings.TrimSpace(c.QueryParam("q"))
		format := strings.ToLower(strings.TrimSpace(c.QueryParam("format")))
		if format == "" {
			format = "markdown"
		}
		detail := strings.TrimSpace(c.QueryParam("detail"))
		fields := strings.TrimSpace(c.QueryParam("fields"))
		projectID := strings.TrimSpace(c.QueryParam("project_id"))
		slog.Info(
			"api request",
			"event", "api.search.request",
			"request_id", reqID,
			"method", "GET",
			"path", "/api/search",
			"format", format,
			"query_len", len([]rune(q)),
			"project_id", projectID,
			"detail", detail,
			"fields", fields,
		)
		if q == "" {
			slog.Info("api request", "event", "api.search.bad_request", "request_id", reqID, "status", 400, "detail", "missing query parameter q")
			return SearchResponse{}, fuego.BadRequestError{
				Title:  "Bad Request",
				Detail: "missing query parameter q",
			}
		}
		var resultKeys []string
		if format == "json" {
			keys, perr := parseSearchResultFields(detail, fields)
			if perr != nil {
				slog.Info("api request", "event", "api.search.bad_request", "request_id", reqID, "status", 400, "detail", perr.Error())
				return SearchResponse{}, fuego.BadRequestError{
					Title:  "Bad Request",
					Detail: perr.Error(),
				}
			}
			resultKeys = keys
		}
		ctx, cancel := context.WithTimeout(c, 5*time.Minute)
		defer cancel()
		outFmt := "markdown"
		if format == "json" {
			outFmt = "json"
		}
		args := []string{"search", "--query", q, "--format", outFmt}
		if pid := projectID; pid != "" {
			if _, err := strconv.ParseInt(pid, 10, 64); err != nil {
				slog.Info("api request", "event", "api.search.bad_request", "request_id", reqID, "status", 400, "detail", "invalid project_id")
				return SearchResponse{}, fuego.BadRequestError{
					Title:  "Bad Request",
					Detail: "invalid project_id (expected integer)",
				}
			}
			args = append(args, "--project-id", pid)
		}
		if topK := strings.TrimSpace(c.QueryParam("top_k")); topK != "" {
			if n, err := strconv.Atoi(topK); err == nil && n > 0 && n <= 100 {
				args = append(args, "--limit", topK)
			}
		}
		if strings.ToLower(strings.TrimSpace(c.QueryParam("reranker"))) == "true" {
			args = append(args, "--reranker")
		}
		if rm := strings.TrimSpace(c.QueryParam("reranker_model")); rm != "" {
			args = append(args, "--reranker-model", rm)
		}
		out, stderr, err := py.RunModule(ctx, "flows.xylem_flow.cli", args...)
		resp := SearchResponse{
			Stderr:    string(stderr),
			RequestID: reqID,
		}
		if err != nil {
			resp.Error = err.Error()
			resp.Failure = newAPIError("PYTHON_SEARCH_FAILED", err.Error(), false, reqID, map[string]any{"stderr": string(stderr)})
			slog.Warn("search subprocess failed", "err", err, "stderr", resp.Stderr, "request_id", reqID)
			slog.Info("api request", "event", "api.search.failed", "request_id", reqID, "status", 200, "latency_ms", time.Since(started).Milliseconds())
			return resp, nil
		}
		if format == "json" {
			var raw []map[string]any
			if uerr := json.Unmarshal(out, &raw); uerr != nil {
				resp.Error = uerr.Error()
				resp.Failure = newAPIError("SEARCH_JSON_PARSE", uerr.Error(), false, reqID, map[string]any{"stderr": string(stderr)})
				slog.Warn("search json parse failed", "err", uerr, "request_id", reqID)
				slog.Info("api request", "event", "api.search.failed", "request_id", reqID, "status", 200, "latency_ms", time.Since(started).Milliseconds())
				return resp, nil
			}
			hits := normalizeSearchHits(raw)
			encoded, merr := marshalSearchResults(hits, resultKeys)
			if merr != nil {
				resp.Error = merr.Error()
				resp.Failure = newAPIError("SEARCH_RESULT_ENCODE", merr.Error(), false, reqID, map[string]any{"stderr": string(stderr)})
				slog.Warn("search results encode failed", "err", merr, "request_id", reqID)
				slog.Info("api request", "event", "api.search.failed", "request_id", reqID, "status", 200, "latency_ms", time.Since(started).Milliseconds())
				return resp, nil
			}
			resp.Results = encoded
			slog.Info("api request", "event", "api.search.success", "request_id", reqID, "status", 200, "format", "json", "result_count", len(hits), "latency_ms", time.Since(started).Milliseconds())
			return resp, nil
		}
		resp.Markdown = strings.TrimSpace(string(out))
		slog.Info("api request", "event", "api.search.success", "request_id", reqID, "status", 200, "format", "markdown", "markdown_len", len([]rune(resp.Markdown)), "latency_ms", time.Since(started).Milliseconds())
		return resp, nil
	},
		fuego.OptionSummary("Semantic search (markdown or format=json; optional detail/fields for JSON)"),
		fuego.OptionQuery("q", "Search query text", fuego.ParamString(), fuego.ParamRequired()),
		fuego.OptionQuery("format", "Response format: markdown (default) or json", fuego.ParamString()),
		fuego.OptionQuery("detail", "JSON result preset: full, standard, summary (format=json only)", fuego.ParamString()),
		fuego.OptionQuery("fields", "Comma-separated sparse result keys; overrides detail (format=json only)", fuego.ParamString()),
		fuego.OptionQuery("project_id", "Optional project filter (integer)", fuego.ParamInteger()),
		fuego.OptionQuery("top_k", "Max hits (1-100); passed as --limit to search CLI", fuego.ParamString()),
		fuego.OptionQuery("reranker", "Set to true to enable reranker", fuego.ParamString()),
		fuego.OptionQuery("reranker_model", "Reranker model name when reranker is enabled", fuego.ParamString()),
	)

	fuego.Get(api, "/restore", func(c fuego.ContextNoBody) (RestoreResponse, error) {
		reqID := newRequestID(c)
		started := time.Now()
		l1ID := strings.TrimSpace(c.QueryParam("l1_id"))
		l2ID := strings.TrimSpace(c.QueryParam("l2_id"))
		format := strings.ToLower(strings.TrimSpace(c.QueryParam("format")))
		if format == "" {
			format = "markdown"
		}
		slog.Info(
			"api request",
			"event", "api.restore.request",
			"request_id", reqID,
			"method", "GET",
			"path", "/api/restore",
			"l1_id", l1ID,
			"l2_id", l2ID,
			"format", format,
		)
		if (l1ID == "" && l2ID == "") || (l1ID != "" && l2ID != "") {
			slog.Info("api request", "event", "api.restore.bad_request", "request_id", reqID, "status", 400, "detail", "exactly one of l1_id or l2_id is required")
			return RestoreResponse{}, fuego.BadRequestError{
				Title:  "Bad Request",
				Detail: "exactly one of l1_id or l2_id is required",
			}
		}
		if format != "markdown" && format != "json" {
			slog.Info("api request", "event", "api.restore.bad_request", "request_id", reqID, "status", 400, "detail", "invalid format")
			return RestoreResponse{}, fuego.BadRequestError{
				Title:  "Bad Request",
				Detail: "invalid format (use markdown or json)",
			}
		}

		ctx, cancel := context.WithTimeout(c, 5*time.Minute)
		defer cancel()

		args := []string{"restore", "--format", format}
		if l1ID != "" {
			args = append(args, "--l1-id", l1ID)
		}
		if l2ID != "" {
			args = append(args, "--l2-id", l2ID)
		}

		out, stderr, err := py.RunModule(ctx, "flows.xylem_flow.cli", args...)
		resp := RestoreResponse{
			Stderr:    string(stderr),
			RequestID: reqID,
		}
		if err != nil {
			resp.Error = err.Error()
			resp.Failure = newAPIError("PYTHON_RESTORE_FAILED", err.Error(), false, reqID, map[string]any{"stderr": string(stderr)})
			slog.Warn("restore subprocess failed", "err", err, "stderr", resp.Stderr, "request_id", reqID)
			slog.Info("api request", "event", "api.restore.failed", "request_id", reqID, "status", 200, "latency_ms", time.Since(started).Milliseconds())
			return resp, nil
		}

		if format == "json" {
			var raw map[string]any
			if uerr := json.Unmarshal(out, &raw); uerr != nil {
				resp.Error = uerr.Error()
				resp.Failure = newAPIError("RESTORE_JSON_PARSE", uerr.Error(), false, reqID, map[string]any{"stderr": string(stderr)})
				slog.Warn("restore json parse failed", "err", uerr, "request_id", reqID)
				slog.Info("api request", "event", "api.restore.failed", "request_id", reqID, "status", 200, "latency_ms", time.Since(started).Milliseconds())
				return resp, nil
			}
			encoded, merr := json.Marshal(raw)
			if merr != nil {
				resp.Error = merr.Error()
				resp.Failure = newAPIError("RESTORE_RESULT_ENCODE", merr.Error(), false, reqID, map[string]any{"stderr": string(stderr)})
				slog.Warn("restore results encode failed", "err", merr, "request_id", reqID)
				slog.Info("api request", "event", "api.restore.failed", "request_id", reqID, "status", 200, "latency_ms", time.Since(started).Milliseconds())
				return resp, nil
			}
			resp.Result = encoded
			slog.Info("api request", "event", "api.restore.success", "request_id", reqID, "status", 200, "format", "json", "result_keys", len(raw), "latency_ms", time.Since(started).Milliseconds())
			return resp, nil
		}

		resp.Markdown = strings.TrimSpace(string(out))
		slog.Info("api request", "event", "api.restore.success", "request_id", reqID, "status", 200, "format", "markdown", "markdown_len", len([]rune(resp.Markdown)), "latency_ms", time.Since(started).Milliseconds())
		return resp, nil
	},
		fuego.OptionSummary("Restore content by l1_id or l2_id (markdown or format=json)"),
		fuego.OptionQuery("l1_id", "knowledge_l1.id UUID (exactly one of l1_id/l2_id required)", fuego.ParamString()),
		fuego.OptionQuery("l2_id", "knowledge_l2.id UUID (exactly one of l1_id/l2_id required)", fuego.ParamString()),
		fuego.OptionQuery("format", "Response format: markdown (default) or json", fuego.ParamString()),
	)

	fuego.Post(api, "/ingest", func(c fuego.ContextWithBody[IngestRequest]) (IngestResponse, error) {
		reqID := newRequestID(c)
		started := time.Now()
		log := logging.Default()
		body, err := c.Body()
		if err != nil {
			return IngestResponse{OK: false, Error: err.Error(), Failure: newAPIError("BAD_BODY", err.Error(), false, reqID, nil), RequestID: reqID}, nil
		}
		path := strings.TrimSpace(body.Path)
		if path == "" {
			return IngestResponse{}, fuego.BadRequestError{
				Title:  "Bad Request",
				Detail: "json field path is required",
			}
		}
		ctx, cancel := context.WithTimeout(c, 30*time.Minute)
		defer cancel()
		out, stderr, err := py.RunModule(ctx, "property.root_props.run", path)
		resp := IngestResponse{
			OK:        err == nil,
			Stdout:    string(out),
			Stderr:    string(stderr),
			RequestID: reqID,
		}
		if err != nil {
			resp.Error = err.Error()
			resp.Failure = newAPIError("PYTHON_INGEST_FAILED", err.Error(), false, reqID, map[string]any{"stderr": string(stderr)})
			log.LogIngest(logging.IngestLog{
				RequestID:  reqID,
				SourcePath: path,
				DurationMs: time.Since(started).Milliseconds(),
				Stdout:     resp.Stdout,
				Stderr:     resp.Stderr,
				Err:        err,
			})
			return resp, nil
		}
		log.LogIngest(logging.IngestLog{
			RequestID:  reqID,
			SourcePath: path,
			DurationMs: time.Since(started).Milliseconds(),
			Stdout:     resp.Stdout,
			Stderr:     resp.Stderr,
		})
		return resp, nil
	},
		fuego.OptionSummary("Ingest markdown path via Root → Phloem"),
	)

	registerJobRoutes(api, py, reg)
}

// Run creates a Fuego server on addr (e.g. "127.0.0.1:8787"), registers routes, and blocks until shutdown.
func Run(addr string) error {
	py, err := runner.NewRunner()
	if err != nil {
		return err
	}
	if err := py.ValidatePython(); err != nil {
		slog.Warn("python environment check failed — ingest/search subprocesses may not work", "err", err)
	} else {
		slog.Info("python environment ok", "binary", py.Python)
	}

	// PostgreSQL: when configured (POSTGRES_USER set), require a working pool before serving.
	// Fail fast so Phloem RegisterProject / Rhizome sink are not left without PG.
	var pgPool *pgxpool.Pool
	pgConn := env.PostgresConnString()
	if rawHost := strings.TrimSpace(os.Getenv("POSTGRES_HOST")); pgConn != "" && rawHost != "" && env.PostgresDialHost() != rawHost {
		slog.Info("postgres: POSTGRES_HOST is a Docker-only DNS name; using dial host on the machine (published DB port)", "POSTGRES_HOST", rawHost, "dial_host", env.PostgresDialHost())
	}
	if pgConn != "" {
		pool, err := pgxpool.New(context.Background(), pgConn)
		if err != nil {
			slog.Error("postgres connection failed — fix POSTGRES_* or start Postgres", "err", err)
			os.Exit(1)
		}
		pingCtx, pingCancel := context.WithTimeout(context.Background(), 5*time.Second)
		err = pool.Ping(pingCtx)
		pingCancel()
		if err != nil {
			pool.Close()
			slog.Error("postgres ping failed — fix POSTGRES_* or start Postgres", "err", err)
			os.Exit(1)
		}
		pgPool = pool
		slog.Info("postgres connected")
	}

	// Initialize Phloem gRPC Server in the background using the same Runner.
	grpcAddr := getEnv("GOPEDIA_PHLOEM_GRPC_ADDR", ":50051")
	go startPhloemGRPC(grpcAddr, py, pgPool)
	// Fuego defaults Read/WriteTimeout to 30s; ingest/search subprocesses can run much longer.
	const httpLongTimeout = 40 * time.Minute
	s := fuego.NewServer(
		fuego.WithAddr(addr),
		func(sv *fuego.Server) {
			sv.Server.ReadTimeout = httpLongTimeout
			sv.Server.ReadHeaderTimeout = httpLongTimeout
			sv.Server.WriteTimeout = httpLongTimeout
			sv.Server.IdleTimeout = httpLongTimeout
		},
	)
	Register(s, py)
	return s.Run()
}

func startPhloemGRPC(grpcAddr string, py *runner.Runner, pgPool *pgxpool.Pool) {
	ctx := context.Background()

	if pgPool != nil {
		defer pgPool.Close()
	}

	// Qdrant (optional) — failure is a warning; search/vectors degrade without it.
	var qdrantClient *qdrant.Client
	rawQdrant := strings.TrimSpace(os.Getenv("QDRANT_HOST"))
	if rawQdrant != "" {
		qHost := env.DialQdrantHost()
		if qHost != rawQdrant && strings.TrimSpace(os.Getenv("GOPEDIA_QDRANT_HOST")) == "" {
			slog.Info("qdrant: compose hostname not resolvable on host; using published port", "QDRANT_HOST", rawQdrant, "dial_host", qHost)
		}
		port := 6334
		if p, err := strconv.Atoi(getEnv("QDRANT_GRPC_PORT", getEnv("QDRANT_PORT", "6334"))); err == nil {
			port = p
		}
		client, err := qdrant.NewClient(&qdrant.Config{
			Host: qHost,
			Port: port,
		})
		if err != nil {
			slog.Warn("qdrant not available — semantic search/embeddings may be limited", "err", err)
		} else {
			qdrantClient = client
			slog.Info("qdrant connected")
		}
	}
	// Embedder must be initialized before Qdrant collection so vector size is correct.
	var emb embedder.Embedder
	if os.Getenv("GOPEDIA_EMBEDDING_BACKEND") == "local" {
		emb = embedder.NewLocal()
		slog.Info("embedding backend: local", "addr", os.Getenv("LOCAL_EMBEDDING_ADDR"))
	} else {
		emb = embedder.NewOpenAI()
		slog.Info("embedding backend: openai", "model", os.Getenv("OPENAI_EMBEDDING_MODEL"))
	}

	if qdrantClient != nil {
		collection := getEnv("QDRANT_COLLECTION", "gopedia_markdown")
		if err := ontologyso.EnsureQdrantCollection(ctx, qdrantClient, collection, uint64(emb.VectorSize())); err != nil {
			slog.Warn("qdrant collection not ready — semantic search may fail until Qdrant is fixed", "collection", collection, "err", err)
		} else {
			slog.Info("qdrant collection ready", "collection", collection, "vector_size", emb.VectorSize())
		}
	}

	// TypeDB (optional) — HTTP stack does not embed a driver; TCP reachability only when configured.
	rawTypeDB := strings.TrimSpace(os.Getenv("TYPEDB_HOST"))
	if rawTypeDB != "" {
		th := env.DialTypeDBHost()
		if th != rawTypeDB && strings.TrimSpace(os.Getenv("GOPEDIA_TYPEDB_HOST")) == "" {
			slog.Info("typedb: compose hostname not resolvable on host; using published port", "TYPEDB_HOST", rawTypeDB, "dial_host", th)
		}
		tdPort := getEnv("TYPEDB_PORT", "1729")
		tdAddr := net.JoinHostPort(th, tdPort)
		conn, err := net.DialTimeout("tcp", tdAddr, 5*time.Second)
		if err != nil {
			slog.Warn("typedb not reachable — graph sync features may be unavailable", "addr", tdAddr, "err", err)
		} else {
			_ = conn.Close()
			slog.Info("typedb tcp ok", "addr", tdAddr)
		}
	}

	// Redis (optional) for Tuber keyword cache
	var redisClient *redis.Client
	if host := getEnv("REDIS_HOST", ""); host != "" {
		port := getEnv("REDIS_PORT", "6379")
		redisClient = redis.NewClient(&redis.Options{
			Addr: host + ":" + port,
		})
		slog.Info("redis configured", "addr", host+":"+port)
	}
	defaultSink := sink.NewDefaultSink(sink.SinkConfig{
		PGPool:   pgPool,
		Qdrant:   qdrantClient,
		Redis:    redisClient,
		Embedder: emb,
	})
	idGen := identityso.NewGenerator(identityso.WorkerIDFromEnv())
	phloem.Register(domain.Wiki, domain.NewWikiPipeline(
		toc.MarkdownTOCParser{},
		chunker.ByHeadingChunker{},
		defaultSink,
		idGen,
	))
	// Code domain pipeline (Python/Go via tree-sitter).
	codeParser := &toc.CodeTOCParser{
		Lang:      "python",
		RepoRoot:  py.RepoRoot,
		PythonBin: py.Python,
	}
	phloem.Register(domain.Code, domain.NewCodePipeline(
		codeParser,
		&chunker.CodeChunker{Parser: codeParser},
		defaultSink,
		idGen,
	))
	slog.Info("code pipeline registered", "repo_root", py.RepoRoot, "python", py.Python)
	server := phloem.NewServer(pgPool)

	lis, err := net.Listen("tcp", grpcAddr)
	if err != nil {
		slog.Error("listen failed", "addr", grpcAddr, "err", err)
		os.Exit(1)
	}
	defer lis.Close()

	grpcServer := grpc.NewServer()
	pb.RegisterPhloemServer(grpcServer, server)
	reflection.Register(grpcServer)

	slog.Info("phloem gRPC listening", "addr", grpcAddr)
	if err := grpcServer.Serve(lis); err != nil {
		slog.Error("serve failed", "err", err)
		os.Exit(1)
	}
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
