package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"gala/internal/worker/renderer"
	"gala/internal/worker/util"
)

type Deps struct {
	Pool        *pgxpool.Pool
	Renderer    renderer.Client
	StorageRoot string
}

type Processor struct {
	pool        *pgxpool.Pool
	renderer    renderer.Client
	storageRoot string
}

func New(d Deps) *Processor {
	return &Processor{
		pool:        d.Pool,
		renderer:    d.Renderer,
		storageRoot: d.StorageRoot,
	}
}

func (p *Processor) ProcessJob(ctx context.Context, jobID string) error {
	// 1) Leer params del job
	var paramsJSON string
	if err := p.pool.QueryRow(ctx,
		`SELECT params_json FROM jobs WHERE id=$1`,
		jobID,
	).Scan(&paramsJSON); err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	var params map[string]any
	_ = json.Unmarshal([]byte(paramsJSON), &params)

	// 2) RUNNING
	_, _ = p.pool.Exec(ctx,
		`UPDATE jobs SET status='RUNNING', started_at=NOW() WHERE id=$1`,
		jobID,
	)

	// 3) Construir spec (Hello Render v0)
	spec := RendererSpec{
		JobID:  jobID,
		Params: params,
	}
	spec.Output.VideoObjectKey = fmt.Sprintf("renders/%s/hello.mp4", jobID)
	spec.Output.ThumbObjectKey = fmt.Sprintf("renders/%s/hello.jpg", jobID)

	// 4) Llamar renderer
	if err := p.renderer.Render(spec); err != nil {
		p.failJob(jobID)
		return err
	}

	// 5) Registrar outputs como assets
	videoAssetID, videoSize, err := p.registerAsset(
		ctx,
		"render_output",
		"video/mp4",
		spec.Output.VideoObjectKey,
	)
	if err != nil {
		p.failJob(jobID)
		return err
	}

	thumbAssetID, thumbSize, err := p.registerAsset(
		ctx,
		"thumbnail",
		"image/jpeg",
		spec.Output.ThumbObjectKey,
	)
	if err != nil {
		p.failJob(jobID)
		return err
	}

	// 6) Registrar job_output
	outputID := util.NewID("out")
	_, err = p.pool.Exec(ctx,
		`INSERT INTO job_outputs (id, job_id, variant, video_asset_id, thumbnail_asset_id)
		 VALUES ($1,$2,1,$3,$4)`,
		outputID, jobID, videoAssetID, thumbAssetID,
	)
	if err != nil {
		p.failJob(jobID)
		return err
	}

	// 7) DONE
	_, _ = p.pool.Exec(ctx,
		`UPDATE jobs SET status='DONE', finished_at=NOW() WHERE id=$1`,
		jobID,
	)

	fmt.Printf("job DONE %s (video=%d bytes, thumb=%d bytes)\n", jobID, videoSize, thumbSize)
	return nil
}

func (p *Processor) registerAsset(
	ctx context.Context,
	kind string,
	mime string,
	objectKey string,
) (assetID string, size int64, err error) {

	path := filepath.Join(p.storageRoot, objectKey)
	st, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}

	assetID = util.NewID("ast")
	size = st.Size()

	_, err = p.pool.Exec(ctx,
		`INSERT INTO assets (id, kind, provider, object_key, mime, size_bytes)
		 VALUES ($1,$2,'localfs',$3,$4,$5)`,
		assetID, kind, objectKey, mime, size,
	)
	if err != nil {
		return "", 0, err
	}

	return assetID, size, nil
}

func (p *Processor) failJob(jobID string) {
	_, _ = p.pool.Exec(
		context.Background(),
		`UPDATE jobs SET status='FAILED', finished_at=NOW() WHERE id=$1`,
		jobID,
	)
}

var _ = time.Now // evita “unused” si cambias logs
