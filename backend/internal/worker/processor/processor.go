package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	contracts "gala/internal/contracts/renderer/v0"
	"gala/internal/ports"
	"gala/internal/worker/renderer"
	"gala/internal/worker/util"
)

type Deps struct {
	Pool        *pgxpool.Pool
	Renderer    renderer.Client
	StorageRoot string
	SP          ports.StorageProvider
}

type Processor struct {
	pool        *pgxpool.Pool
	renderer    renderer.Client
	storageRoot string
	sp          ports.StorageProvider
}

func New(d Deps) *Processor {
	return &Processor{
		pool:        d.Pool,
		renderer:    d.Renderer,
		storageRoot: d.StorageRoot,
		sp:          d.SP,
	}
}

func (p *Processor) ProcessJob(ctx context.Context, jobID string) error {
	var paramsJSON string
	if err := p.pool.QueryRow(ctx, `SELECT params_json FROM jobs WHERE id=$1`, jobID).Scan(&paramsJSON); err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	var params map[string]any
	_ = json.Unmarshal([]byte(paramsJSON), &params)

	_, _ = p.pool.Exec(ctx, `UPDATE jobs SET status='RUNNING', started_at=NOW() WHERE id=$1`, jobID)

	spec := contracts.RendererSpec{JobID: jobID, Params: params}
	spec.Output.VideoObjectKey = fmt.Sprintf("renders/%s/hello.mp4", jobID)
	spec.Output.ThumbObjectKey = fmt.Sprintf("renders/%s/hello.jpg", jobID)

	if err := p.renderer.Render(spec); err != nil {
		p.failJob(jobID)
		return err
	}

	// sube a provider (localfs o gdrive) y registra asset apuntando al object_key del provider
	videoAssetID, videoSize, err := p.registerAssetFromLocalFile(ctx,
		"render_output",
		"video/mp4",
		spec.Output.VideoObjectKey,
	)
	if err != nil {
		p.failJob(jobID)
		return err
	}

	thumbAssetID, thumbSize, err := p.registerAssetFromLocalFile(ctx,
		"thumbnail",
		"image/jpeg",
		spec.Output.ThumbObjectKey,
	)
	if err != nil {
		p.failJob(jobID)
		return err
	}

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

	_, _ = p.pool.Exec(ctx, `UPDATE jobs SET status='DONE', finished_at=NOW() WHERE id=$1`, jobID)
	fmt.Printf("job DONE %s (video=%d bytes, thumb=%d bytes)\n", jobID, videoSize, thumbSize)
	return nil
}

func (p *Processor) registerAssetFromLocalFile(
	ctx context.Context,
	kind string,
	mime string,
	localObjectKey string, // ruta “lógica” usada por renderer en /data
) (assetID string, size int64, err error) {

	localPath := filepath.Join(p.storageRoot, localObjectKey)
	st, err := os.Stat(localPath)
	if err != nil {
		return "", 0, err
	}
	f, err := os.Open(localPath)
	if err != nil {
		return "", 0, err
	}
	defer f.Close()

	// subimos al provider actual (localfs o gdrive)
	out, err := p.sp.PutObject(ctx, ports.PutObjectInput{
		ObjectKey:   localObjectKey,
		ContentType: mime,
		Reader:      f,
		Size:        st.Size(),
	})
	if err != nil {
		return "", 0, err
	}

	assetID = util.NewID("ast")
	size = out.Size

	_, err = p.pool.Exec(ctx,
		`INSERT INTO assets (id, kind, provider, object_key, mime, size_bytes)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		assetID, kind, p.sp.Provider(), out.ObjectKey, mime, size,
	)
	if err != nil {
		return "", 0, err
	}

	return assetID, size, nil
}

func (p *Processor) failJob(jobID string) {
	_, _ = p.pool.Exec(context.Background(), `UPDATE jobs SET status='FAILED', finished_at=NOW() WHERE id=$1`, jobID)
}

var _ = time.Now
