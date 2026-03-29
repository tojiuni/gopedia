// Package api registers Fuego HTTP routes for Gopedia (ingest/search via Python subprocess).
package api

import (
	"context"
	"fmt"
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
	"gopedia/internal/runner"
)

// IngestRequest is the JSON body for POST /api/ingest.
type IngestRequest struct {
	Path string `json:"path"`
}

// IngestResponse reports subprocess outcome.
type IngestResponse struct {
	OK     bool   `json:"ok"`
	Stdout string `json:"stdout,omitempty"`
	Stderr string `json:"stderr,omitempty"`
	Error  string `json:"error,omitempty"`
}

// SearchResponse is the JSON body for GET /api/search.
type SearchResponse struct {
	Markdown string `json:"markdown"`
	Stderr   string `json:"stderr,omitempty"`
	Error    string `json:"error,omitempty"`
}

// HealthResponse is returned by GET /api/health.
type HealthResponse struct {
	Status string `json:"status"`
}

// Register mounts /api routes on the given Fuego server using Python subprocesses.
func Register(s *fuego.Server, py *runner.Runner) {
	api := fuego.Group(s, "/api")

	fuego.Get(api, "/health", func(c fuego.ContextNoBody) (HealthResponse, error) {
		return HealthResponse{Status: "ok"}, nil
	},
		fuego.OptionSummary("Liveness check"),
	)

	fuego.Get(api, "/search", func(c fuego.ContextNoBody) (SearchResponse, error) {
		q := strings.TrimSpace(c.QueryParam("q"))
		if q == "" {
			return SearchResponse{}, fuego.BadRequestError{
				Title:  "Bad Request",
				Detail: "missing query parameter q",
			}
		}
		ctx, cancel := context.WithTimeout(c, 5*time.Minute)
		defer cancel()
		out, stderr, err := py.RunModule(ctx, "flows.xylem_flow.cli", "search", "--query", q, "--format", "markdown")
		resp := SearchResponse{
			Markdown: strings.TrimSpace(string(out)),
			Stderr:   string(stderr),
		}
		if err != nil {
			resp.Error = err.Error()
			slog.Warn("search subprocess failed", "err", err, "stderr", resp.Stderr)
			return resp, fmt.Errorf("python search: %w", err)
		}
		return resp, nil
	},
		fuego.OptionSummary("Semantic search + rich context (markdown)"),
	)

	fuego.Post(api, "/ingest", func(c fuego.ContextWithBody[IngestRequest]) (IngestResponse, error) {
		body, err := c.Body()
		if err != nil {
			return IngestResponse{OK: false, Error: err.Error()}, err
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
			OK:     err == nil,
			Stdout: string(out),
			Stderr: string(stderr),
		}
		if err != nil {
			resp.Error = err.Error()
			slog.Warn("ingest subprocess failed", "err", err, "stderr", resp.Stderr)
			return resp, fmt.Errorf("python ingest: %w", err)
		}
		return resp, nil
	},
		fuego.OptionSummary("Ingest markdown path via Root → Phloem"),
	)
}

// Run creates a Fuego server on addr (e.g. "127.0.0.1:8787"), registers routes, and blocks until shutdown.
func Run(addr string) error {
	// Initialize Phloem gRPC Server in the background
	grpcAddr := getEnv("GOPEDIA_PHLOEM_GRPC_ADDR", ":50051")
	go startPhloemGRPC(grpcAddr)

	py, err := runner.NewRunner()
	if err != nil {
		return err
	}
	s := fuego.NewServer(fuego.WithAddr(addr))
	Register(s, py)
	return s.Run()
}

func startPhloemGRPC(grpcAddr string) {
	ctx := context.Background()

	// PostgreSQL (optional)
	var pgPool *pgxpool.Pool
	pgConn := pgConnString()
	if pgConn != "" {
		pool, err := pgxpool.New(ctx, pgConn)
		if err != nil {
			slog.Warn("postgres connect failed, continuing without PG", "err", err)
		} else {
			defer pool.Close()
			pgPool = pool
			slog.Info("postgres connected")
		}
	}

	// Qdrant (optional)
	var qdrantClient *qdrant.Client
	if getEnv("QDRANT_HOST", "") != "" {
		port := 6334
		if p, err := strconv.Atoi(getEnv("QDRANT_GRPC_PORT", getEnv("QDRANT_PORT", "6334"))); err == nil {
			port = p
		}
		client, err := qdrant.NewClient(&qdrant.Config{
			Host: getEnv("QDRANT_HOST", "localhost"),
			Port: port,
		})
		if err != nil {
			slog.Warn("qdrant connect failed, continuing without Qdrant", "err", err)
		} else {
			qdrantClient = client
			slog.Info("qdrant connected")
		}
	}
	if qdrantClient != nil {
		collection := getEnv("QDRANT_COLLECTION", "gopedia_markdown")
		if err := ontologyso.EnsureQdrantCollection(ctx, qdrantClient, collection, ontologyso.DefaultVectorSize); err != nil {
			slog.Warn("qdrant ensure collection failed", "err", err)
		} else {
			slog.Info("qdrant collection ready", "collection", collection)
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

	emb := embedder.NewOpenAI()
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

func pgConnString() string {
	host := getEnv("POSTGRES_HOST", "")
	if host == "" {
		return ""
	}
	port := getEnv("POSTGRES_PORT", "5432")
	user := getEnv("POSTGRES_USER", "")
	pass := getEnv("POSTGRES_PASSWORD", "")
	db := getEnv("POSTGRES_DB", "gopedia")
	sslmode := getEnv("POSTGRES_SSLMODE", "disable")
	if user == "" {
		return ""
	}
	return "postgres://" + user + ":" + pass + "@" + host + ":" + port + "/" + db + "?sslmode=" + sslmode
}

func getEnv(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}
