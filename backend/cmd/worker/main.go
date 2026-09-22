package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/distributed-workflow-engine/backend/internal/config"
	"github.com/distributed-workflow-engine/backend/internal/engine"
	"github.com/distributed-workflow-engine/backend/internal/queue"
	"github.com/distributed-workflow-engine/backend/internal/store"
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
	q := queue.NewRedisQueue(rdb)

	w := engine.NewWorker(
		cfg.WorkerID,
		jobStore,
		workerStore,
		q,
		cfg.PollInterval,
		cfg.JobTimeout,
		cfg.VisibilityTimeout,
		cfg.HeartbeatInterval,
		cfg.Concurrency,
		logger,
	)
	w.RegisterHandler("send_email", engine.HandleSendEmail)
	w.RegisterHandler("generate_report", engine.HandleGenerateReport)
	w.RegisterHandler("webhook_delivery", engine.HandleWebhookDelivery)

	logger.Info("worker starting", "id", cfg.WorkerID)
	if err := w.Run(ctx); err != nil {
		logger.Error("worker error", "error", err)
		os.Exit(1)
	}
	logger.Info("worker stopped")
}
