package worker

import (
	"context"
	"fmt"
	"time"

	"gala/internal/worker/processor"
	"gala/internal/worker/queue"
	"gala/internal/worker/renderer"
)

func Run(ctx context.Context, d Deps) error {
	q := queue.NewRedisQueue(d.RDB, d.QueueName)
	rc := renderer.NewHTTPClient(d.RendererBaseURL)

	p := processor.New(processor.Deps{
		Pool:        d.Pool,
		Renderer:    rc,
		StorageRoot: d.StorageRoot,
	})

	for {
		jobID, err := q.Pop(ctx)
		if err != nil {
			fmt.Println("queue error:", err)
			time.Sleep(1 * time.Second)
			continue
		}
		if jobID == "" {
			continue
		}

		if err := p.ProcessJob(ctx, jobID); err != nil {
			fmt.Println("job failed:", jobID, err)
		}
	}
}
