package processor

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"syscall"

	"github.com/jackc/pgx/v5/pgxpool"

	contracts "gala/internal/contracts/renderer/v0"
	"gala/internal/ports"
	"gala/internal/worker/renderer"
	"gala/internal/worker/util"
)

type Deps struct {
	Pool         *pgxpool.Pool
	Renderer     renderer.Client
	StorageRoot  string
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

type parsedJob struct {
	TemplateID   string
	Inputs       map[string]string
	Params       map[string]any
	MergedParams map[string]any
	HasEnvelope  bool
}

func (p *Processor) ProcessJob(ctx context.Context, jobID string) error {
	var paramsJSON string
	if err := p.pool.QueryRow(ctx, `SELECT params_json FROM jobs WHERE id=$1`, jobID).Scan(&paramsJSON); err != nil {
		e := fmt.Errorf("job not found: %w", err)
		p.failJob(jobID, e)
		return e
	}

	j, err := p.parseJobEnvelope(ctx, paramsJSON)
	if err != nil {
		p.failJob(jobID, err)
		return err
	}

	_, _ = p.pool.Exec(ctx, `UPDATE jobs SET status='RUNNING', started_at=NOW(), error_text=NULL WHERE id=$1`, jobID)

	videoKey := fmt.Sprintf("renders/%s/hello.mp4", jobID)
	thumbKey := fmt.Sprintf("renders/%s/hello.jpg", jobID)

	if j.HasEnvelope && len(j.Inputs) > 0 {
		inputPaths, err := p.materializeInputs(ctx, jobID, j.Inputs)
		if err != nil {
			p.failJob(jobID, err)
			return err
		}

		specV1 := map[string]any{
			"job_id":      jobID,
			"template_id": j.TemplateID,
			"inputs":      inputPaths,
			"params":      j.MergedParams,
			"output": map[string]any{
				"video_object_key": videoKey,
				"thumb_object_key": thumbKey,
			},
		}

		if err := p.renderer.RenderV1(specV1); err != nil {
			p.failJob(jobID, err)
			return err
		}
	} else {
		spec := contracts.RendererSpec{JobID: jobID, Params: j.MergedParams}
		spec.Output.VideoObjectKey = videoKey
		spec.Output.ThumbObjectKey = thumbKey

		if err := p.renderer.Render(spec); err != nil {
			p.failJob(jobID, err)
			return err
		}
	}

	videoAssetID, _, err := p.registerAssetFromLocalFile(ctx, "render_output", "video/mp4", videoKey)
	if err != nil {
		p.failJob(jobID, err)
		return err
	}
	thumbAssetID, _, err := p.registerAssetFromLocalFile(ctx, "thumbnail", "image/jpeg", thumbKey)
	if err != nil {
		p.failJob(jobID, err)
		return err
	}

	outputID := util.NewID("out")
	_, err = p.pool.Exec(ctx,
		`INSERT INTO job_outputs (id, job_id, variant, video_asset_id, thumbnail_asset_id)
		 VALUES ($1,$2,1,$3,$4)`,
		outputID, jobID, videoAssetID, thumbAssetID,
	)
	if err != nil {
		p.failJob(jobID, err)
		return err
	}

	p.maybeCleanupJobFolder(jobID)

	_, _ = p.pool.Exec(ctx, `UPDATE jobs SET status='DONE', finished_at=NOW() WHERE id=$1`, jobID)
	return nil
}

func (p *Processor) parseJobEnvelope(ctx context.Context, paramsJSON string) (*parsedJob, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(paramsJSON), &raw); err != nil {
		return nil, fmt.Errorf("invalid params_json: %w", err)
	}

	j := &parsedJob{
		Inputs:       map[string]string{},
		Params:       map[string]any{},
		MergedParams: map[string]any{},
	}

	if v, ok := raw["template_id"].(string); ok && strings.TrimSpace(v) != "" {
		j.HasEnvelope = true
		j.TemplateID = strings.TrimSpace(v)

		if pm, ok := raw["params"].(map[string]any); ok && pm != nil {
			j.Params = pm
		}

		if im, ok := raw["inputs"].(map[string]any); ok && im != nil {
			for k, vv := range im {
				if s, ok := vv.(string); ok && strings.TrimSpace(s) != "" {
					j.Inputs[k] = strings.TrimSpace(s)
				}
			}
		}

		var defaultsBytes []byte
		if err := p.pool.QueryRow(ctx,
			`SELECT COALESCE(defaults, '{}'::jsonb) FROM templates WHERE id=$1 AND deleted_at IS NULL`,
			j.TemplateID,
		).Scan(&defaultsBytes); err != nil {
			return nil, fmt.Errorf("template not found: %s", j.TemplateID)
		}

		defaults := map[string]any{}
		_ = json.Unmarshal(defaultsBytes, &defaults)

		for k, v := range defaults {
			j.MergedParams[k] = v
		}
		for k, v := range j.Params {
			j.MergedParams[k] = v
		}

		if t, ok := j.MergedParams["text"].(string); !ok || strings.TrimSpace(t) == "" {
			return nil, fmt.Errorf("params.text is required (after defaults merge)")
		}
		return j, nil
	}

	for k, v := range raw {
		j.MergedParams[k] = v
	}
	if t, ok := j.MergedParams["text"].(string); !ok || strings.TrimSpace(t) == "" {
		return nil, fmt.Errorf("params.text is required")
	}
	return j, nil
}

func (p *Processor) materializeInputs(ctx context.Context, jobID string, inputs map[string]string) (map[string]string, error) {
	out := map[string]string{}

	baseDir := filepath.Join(p.storageRoot, "jobs", jobID, "inputs")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, err
	}

	for inputName, assetID := range inputs {
		assetID = strings.TrimSpace(assetID)
		if assetID == "" {
			continue
		}

		var objectKey, mime string
		err := p.pool.QueryRow(ctx, `SELECT object_key, mime FROM assets WHERE id=$1`, assetID).Scan(&objectKey, &mime)
		if err != nil {
			return nil, fmt.Errorf("input asset not found input=%s asset_id=%s", inputName, assetID)
		}

		rc, _, _, err := p.sp.GetObject(ctx, objectKey)
		if err != nil {
			return nil, fmt.Errorf("download input failed input=%s asset_id=%s: %w", inputName, assetID, err)
		}
		defer rc.Close()

		ext := extFromMime(mime)
		filename := sanitizeFilename(inputName) + ext
		localPath := filepath.Join(baseDir, filename)

		f, err := os.Create(localPath)
		if err != nil {
			return nil, err
		}
		if _, err := io.Copy(f, rc); err != nil {
			_ = f.Close()
			return nil, err
		}
		_ = f.Close()

		out[inputName] = localPath
	}

	return out, nil
}

func sanitizeFilename(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "..", "")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, " ", "_")
	if s == "" {
		return "input"
	}
	return s
}

func extFromMime(mime string) string {
	mime = strings.ToLower(strings.TrimSpace(mime))
	switch mime {
	case "image/jpeg", "image/jpg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "audio/wav", "audio/x-wav":
		return ".wav"
	case "audio/mpeg", "audio/mp3":
		return ".mp3"
	case "video/mp4":
		return ".mp4"
	default:
		return ""
	}
}

func (p *Processor) registerAssetFromLocalFile(ctx context.Context, kind string, mime string, localObjectKey string) (assetID string, size int64, err error) {
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

	p.maybeCleanupLocalFile(localObjectKey)
	return assetID, size, nil
}

func (p *Processor) maybeCleanupLocalFile(localObjectKey string) {
	if !p.cleanupLocal {
		return
	}
	if p.sp.Provider() != "gdrive" {
		return
	}
	_ = os.Remove(filepath.Join(p.storageRoot, localObjectKey))
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
	if err == nil || os.IsNotExist(err) {
		return
	}
	if errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST) {
		return
	}
}

func (p *Processor) failJob(jobID string, cause error) {
	msg := ""
	if cause != nil {
		msg = cause.Error()
		if len(msg) > 2000 {
			msg = msg[:2000]
		}
	}
	_, _ = p.pool.Exec(context.Background(),
		`UPDATE jobs SET status='FAILED', finished_at=NOW(), error_text=$2 WHERE id=$1`,
		jobID, msg,
	)
}
