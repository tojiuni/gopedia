// Phloem gRPC server: ingests markdown from Root into Rhizome (PostgreSQL, Qdrant).
package main

import (
	"context"
	"log/slog"
	"net"
	"os"
	"strconv"
	"strings"

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
)

func main() {
	env.LoadDotenv()
	slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: slog.LevelInfo})))

	grpcAddr := getEnv("GOPEDIA_PHLOEM_GRPC_ADDR", ":50051")
	ctx := context.Background()

	// PostgreSQL (optional)
	var pgPool *pgxpool.Pool
	pgConn := env.PostgresConnString()
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
	if strings.TrimSpace(os.Getenv("QDRANT_HOST")) != "" {
		qHost := env.DialQdrantHost()
		port := 6334
		if p, err := strconv.Atoi(getEnv("QDRANT_GRPC_PORT", getEnv("QDRANT_PORT", "6334"))); err == nil {
			port = p
		}
		client, err := qdrant.NewClient(&qdrant.Config{
			Host: qHost,
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
	// Code domain pipeline (Python, Go source files via tree-sitter).
	repoRoot := getEnv("GOPEDIA_REPO_ROOT", ".")
	pythonBin := getEnv("GOPEDIA_PYTHON", "python3")
	codeParser := &toc.CodeTOCParser{
		Lang:      "python", // overridden per-request by domain/code.go detectLangFromTitle
		RepoRoot:  repoRoot,
		PythonBin: pythonBin,
	}
	phloem.Register(domain.Code, domain.NewCodePipeline(
		codeParser,
		&chunker.CodeChunker{Parser: codeParser},
		defaultSink,
		idGen,
	))
	slog.Info("code pipeline registered", "repo_root", repoRoot, "python", pythonBin)
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
