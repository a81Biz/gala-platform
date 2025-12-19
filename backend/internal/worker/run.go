package worker

import (
	"context"
	"time"

	"gala/internal/pkg/logger"
	"gala/internal/worker/processor"
	"gala/internal/worker/queue"
	"gala/internal/worker/renderer"
)

func Run(ctx context.Context, d Deps) error {
	log := d.Log
	if log == nil {
		log = logger.NewDefault()
	}
	log = log.WithComponent("worker")

	q := queue.NewRedisQueue(d.RDB, d.QueueName)
	rc := renderer.NewHTTPClient(d.RendererBaseURL)

	p := processor.New(processor.Deps{
		Pool:         d.Pool,
		Renderer:     rc,
		StorageRoot:  d.StorageRoot,
		CleanupLocal: d.CleanupLocal,
		SP:           d.SP,
		Log:          log,
	})

	for {
		select {
		case <-ctx.Done():
			log.Info("worker context canceled, stopping")
			return ctx.Err()
		default:
		}

		// Use a separate context with timeout for queue operations
		popCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
		jobID, err := q.Pop(popCtx)
		cancel()

		if err != nil {
			// Check if it's a context cancellation
			if ctx.Err() != nil {
				log.Info("worker stopping due to context cancellation")
				return ctx.Err()
			}

			log.Warn("queue pop error, retrying",
				"error", err.Error(),
			)
			time.Sleep(1 * time.Second)
			continue
		}

		if jobID == "" {
			continue
		}

		// Create a context for this job
		jobCtx := logger.ContextWithJobID(ctx, jobID)
		jobLog := log.WithJobID(jobID)

		jobLog.Info("processing job")
		startTime := time.Now()

		if err := p.ProcessJob(jobCtx, jobID); err != nil {
			jobLog.Error("job failed",
				"error", err.Error(),
				"duration_ms", time.Since(startTime).Milliseconds(),
			)
		} else {
			jobLog.Info("job completed",
				"duration_ms", time.Since(startTime).Milliseconds(),
			)
		}
	}
}
