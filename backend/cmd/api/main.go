package main

import (
	"context"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/distributed-workflow-engine/backend/internal/api"
	"github.com/distributed-workflow-engine/backend/internal/config"
	"github.com/distributed-workflow-engine/backend/internal/engine"
	"github.com/distributed-workflow-engine/backend/internal/queue"
	"github.com/distributed-workflow-engine/backend/internal/store"
	"github.com/distributed-workflow-engine/backend/migrations"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.Load()
	if err != nil {
		logger.Error("failed to load config", "error", err)
		os.Exit(1)
	}

	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	pool, err := pgxpool.New(ctx, cfg.DatabaseURL)
	if err != nil {
		logger.Error("failed to connect to database", "error", err)
		os.Exit(1)
	}
	defer pool.Close()

	if err := pool.Ping(ctx); err != nil {
		logger.Error("database not reachable", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to database")

	// Apply database migrations
	if err := store.RunMigrations(ctx, pool, migrations.FS, logger); err != nil {
		logger.Error("failed to run migrations", "error", err)
		os.Exit(1)
	}

	redisOpts, err := redis.ParseURL(cfg.RedisURL)
	if err != nil {
		logger.Error("failed to parse redis URL", "error", err)
		os.Exit(1)
	}
	rdb := redis.NewClient(redisOpts)
	defer rdb.Close()

	if err := rdb.Ping(ctx).Err(); err != nil {
		logger.Error("redis not reachable", "error", err)
		os.Exit(1)
	}
	logger.Info("connected to redis")

	jobStore := store.NewJobStore(pool)
	workerStore := store.NewWorkerStore(pool)
	workflowStore := store.NewWorkflowStore(pool)
	q := queue.NewRedisQueue(rdb)

	workflowEngine := engine.NewWorkflowEngine(workflowStore, jobStore, q, logger)
	router := api.NewRouter(jobStore, workerStore, workflowStore, q, workflowEngine, logger)

	// Start background workflow engine poller
	go func() {
		workflowEngine.Run(ctx)
	}()

	// Start background retry scheduler
	retryScheduler := engine.NewRetryScheduler(jobStore, q, 1*time.Second, 100, logger)
	go func() {
		if err := retryScheduler.Run(ctx); err != nil {
			logger.Error("retry scheduler error", "error", err)
		}
	}()

	// Start background worker supervisor
	supervisor := engine.NewWorkerSupervisor(workerStore, q, 10*time.Second, cfg.WorkerDeadTimeout, logger)
	go func() {
		if err := supervisor.Run(ctx); err != nil {
			logger.Error("worker supervisor error", "error", err)
		}
	}()

	srv := &http.Server{
		Addr:         fmt.Sprintf(":%d", cfg.APIPort),
		Handler:      router,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 30 * time.Second,
		IdleTimeout:  60 * time.Second,
	}

	go func() {
		logger.Info("API server starting", "port", cfg.APIPort)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			logger.Error("server error", "error", err)
			os.Exit(1)
		}
	}()

	<-ctx.Done()
	logger.Info("shutting down API server")

	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()

	if err := srv.Shutdown(shutdownCtx); err != nil {
		logger.Error("server shutdown error", "error", err)
	}
}
