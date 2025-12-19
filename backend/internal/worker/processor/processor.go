package processor

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"gala/internal/ports"
	"gala/internal/worker/renderer"
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

	// Componentes internos
	jobParser       *JobParser
	inputHandler    *InputHandler
	outputHandler   *OutputHandler
	rendererAdapter *RendererAdapter
	cleanup         *Cleanup
}

func New(d Deps) *Processor {
	p := &Processor{
		pool:         d.Pool,
		renderer:     d.Renderer,
		storageRoot:  d.StorageRoot,
		cleanupLocal: d.CleanupLocal,
		sp:           d.SP,
	}

	// Inicializar componentes
	p.jobParser = NewJobParser(d.Pool)
	p.inputHandler = NewInputHandler(d.Pool, d.SP, d.StorageRoot)
	p.outputHandler = NewOutputHandler(d.Pool, d.SP, d.StorageRoot, d.CleanupLocal)
	p.rendererAdapter = NewRendererAdapter(d.Renderer)
	p.cleanup = NewCleanup(d.StorageRoot, d.CleanupLocal, d.SP)

	return p
}

// ProcessJob orquesta el flujo completo del job
func (p *Processor) ProcessJob(ctx context.Context, jobID string) error {
	// 1. Obtener y parsear el job
	paramsJSON, err := p.fetchJobParams(ctx, jobID)
	if err != nil {
		return p.failJob(ctx, jobID, err)
	}

	parsedJob, err := p.jobParser.Parse(ctx, paramsJSON)
	if err != nil {
		return p.failJob(ctx, jobID, err)
	}

	// ✅ Validación mínima y explícita para jobs v1 (envelope):
	// - Si hay template_id (HasEnvelope), es v1.
	// - No permitimos "fallback" silencioso a v0 por inputs vacíos.
	// - Mínimo requerido hoy para este template: avatar_image_asset_id.
	if parsedJob.HasEnvelope {
		avatarID := strings.TrimSpace(parsedJob.Inputs["avatar_image_asset_id"])
		if avatarID == "" {
			return p.failJob(ctx, jobID, fmt.Errorf("missing required input: inputs.avatar_image_asset_id"))
		}

		// Audio: opcional a nivel pipeline (no lo hacemos obligatorio aquí).
		// Peeero: si el job pide captions=1, entonces captions reales desde audio en el futuro
		// requiere voice_audio_asset_id. Hoy solo dejamos la base para no romper.
		//
		// Si quieres que esto falle ya cuando captions=1 y no hay audio, lo activamos explícitamente.
	}

	// 2. Marcar como running
	if err := p.markJobRunning(ctx, jobID); err != nil {
		return p.failJob(ctx, jobID, err)
	}

	// 3. Preparar keys de salida
	outputKeys := GenerateOutputKeys(jobID, parsedJob.CaptionsEnabled())

	// 4. Procesar inputs si es necesario
	var inputPaths map[string]string
	if parsedJob.NeedsInputMaterialization() {
		inputPaths, err = p.inputHandler.Materialize(ctx, jobID, parsedJob.Inputs)
		if err != nil {
			return p.failJob(ctx, jobID, err)
		}
	}

	// 5. Renderizar
	err = p.rendererAdapter.Render(ctx, RenderRequest{
		JobID:      jobID,
		ParsedJob:  parsedJob,
		InputPaths: inputPaths,
		OutputKeys: outputKeys,
	})
	if err != nil {
		return p.failJob(ctx, jobID, err)
	}

	// 6. Registrar outputs
	outputResult, err := p.outputHandler.RegisterOutputs(ctx, RegisterOutputsRequest{
		JobID:           jobID,
		OutputKeys:      outputKeys,
		UsedV1:          parsedJob.UsedV1(),
		CaptionsEnabled: parsedJob.CaptionsEnabled(),
	})
	if err != nil {
		return p.failJob(ctx, jobID, err)
	}

	// 7. Guardar resultado en DB
	if err := p.saveJobOutput(ctx, jobID, outputResult); err != nil {
		return p.failJob(ctx, jobID, err)
	}

	// 8. Limpiar archivos temporales
	p.cleanup.CleanupJob(jobID)

	// 9. Marcar como completado
	return p.markJobDone(ctx, jobID)
}

func (p *Processor) fetchJobParams(ctx context.Context, jobID string) (string, error) {
	var paramsJSON string
	err := p.pool.QueryRow(ctx,
		`SELECT params_json FROM jobs WHERE id=$1`,
		jobID,
	).Scan(&paramsJSON)
	if err != nil {
		return "", fmt.Errorf("job not found: %w", err)
	}
	return paramsJSON, nil
}

func (p *Processor) markJobRunning(ctx context.Context, jobID string) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE jobs SET status='RUNNING', started_at=NOW(), finished_at=NULL, error_text=NULL WHERE id=$1`,
		jobID,
	)
	return err
}

func (p *Processor) markJobDone(ctx context.Context, jobID string) error {
	_, err := p.pool.Exec(ctx,
		`UPDATE jobs SET status='DONE', finished_at=NOW() WHERE id=$1`,
		jobID,
	)
	return err
}

func (p *Processor) saveJobOutput(ctx context.Context, jobID string, result *OutputResult) error {
	_, err := p.pool.Exec(ctx,
		`INSERT INTO job_outputs (id, job_id, variant, video_asset_id, thumbnail_asset_id, captions_asset_id)
         VALUES ($1,$2,1,$3,$4,$5)`,
		result.OutputID,
		jobID,
		result.VideoAssetID,
		result.ThumbAssetID,
		NullIfEmpty(result.CaptionsAssetID),
	)
	return err
}

func (p *Processor) failJob(ctx context.Context, jobID string, cause error) error {
	msg := ""
	if cause != nil {
		msg = cause.Error()
		if len(msg) > 2000 {
			msg = msg[:2000]
		}
	}

	_, _ = p.pool.Exec(ctx,
		`UPDATE jobs SET status='FAILED', finished_at=NOW(), error_text=$2 WHERE id=$1`,
		jobID, msg,
	)

	return cause
}
