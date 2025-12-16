package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"syscall"
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
	// Feature flag: when enabled, worker cleans up local staging under StorageRoot
	// after upload+DB insert have succeeded.
	CleanupLocal bool
	SP           ports.StorageProvider
}

type Processor struct {
	pool         *pgxpool.Pool
	renderer     renderer.Client
	storageRoot  string
	cleanupLocal bool
	sp           ports.StorageProvider
}

func New(d Deps) *Processor {
	return &Processor{
		pool:         d.Pool,
		renderer:     d.Renderer,
		storageRoot:  d.StorageRoot,
		cleanupLocal: d.CleanupLocal,
		sp:           d.SP,
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

	// Best-effort: after the assets were uploaded+recorded, try to remove the job folder
	// (renders/<jobId>) ONLY if it's empty. If leftover files exist for any reason, keep it.
	p.maybeCleanupJobFolder(jobID)

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

	// Cleanup local file only when:
	// - feature flag enabled
	// - provider is gdrive (if provider is localfs, local file IS the storage)
	p.maybeCleanupLocalFile(localObjectKey)

	return assetID, size, nil
}

func (p *Processor) maybeCleanupLocalFile(localObjectKey string) {
	if !p.cleanupLocal {
		fmt.Println("cleanup skipped reason=flag_off")
		return
	}
	if p.sp.Provider() != "gdrive" {
		fmt.Println("cleanup skipped reason=provider_not_gdrive provider=" + p.sp.Provider())
		return
	}

	localPath := filepath.Join(p.storageRoot, localObjectKey)
	if err := os.Remove(localPath); err != nil {
		if os.IsNotExist(err) {
			fmt.Println("cleanup ok (already missing) path=" + localPath)
			return
		}
		fmt.Println("cleanup failed path=" + localPath + " err=" + err.Error())
		return
	}
	fmt.Println("cleanup ok path=" + localPath)
}

func (p *Processor) maybeCleanupJobFolder(jobID string) {
	if !p.cleanupLocal {
		return
	}
	if p.sp.Provider() != "gdrive" {
		return
	}

	jobDir := filepath.Join(p.storageRoot, "renders", jobID)
	err := os.Remove(jobDir)
	if err == nil {
		fmt.Println("cleanup ok dir=" + jobDir)
		return
	}
	if os.IsNotExist(err) {
		fmt.Println("cleanup ok (dir missing) dir=" + jobDir)
		return
	}
	// If not empty, keep it (this is desired).
	if errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST) {
		fmt.Println("cleanup skipped reason=dir_not_empty dir=" + jobDir)
		return
	}
	fmt.Println("cleanup failed dir=" + jobDir + " err=" + err.Error())
}

func (p *Processor) failJob(jobID string) {
	_, _ = p.pool.Exec(context.Background(), `UPDATE jobs SET status='FAILED', finished_at=NOW() WHERE id=$1`, jobID)
}

var _ = time.Now
