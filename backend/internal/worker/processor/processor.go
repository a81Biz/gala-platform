package processor

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"gala/internal/pkg/errors"
	"gala/internal/pkg/logger"
	"gala/internal/ports"
	"gala/internal/worker/renderer"
)

type Deps struct {
	Pool         *pgxpool.Pool
	Renderer     renderer.Client
	StorageRoot  string
	CleanupLocal bool
	SP           ports.StorageProvider
	Log          *logger.Logger
}

type Processor struct {
	pool         *pgxpool.Pool
	renderer     renderer.Client
	storageRoot  string
	cleanupLocal bool
	sp           ports.StorageProvider
	log          *logger.Logger

	// Componentes internos
	jobParser       *JobParser
	inputHandler    *InputHandler
	outputHandler   *OutputHandler
	rendererAdapter *RendererAdapter
	cleanup         *Cleanup
}

func New(d Deps) *Processor {
	log := d.Log
	if log == nil {
		log = logger.NewDefault()
	}
	log = log.WithComponent("processor")

	p := &Processor{
		pool:         d.Pool,
		renderer:     d.Renderer,
		storageRoot:  d.StorageRoot,
		cleanupLocal: d.CleanupLocal,
		sp:           d.SP,
		log:          log,
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
	log := p.log.FromContext(ctx).WithJobID(jobID)

	// 1. Obtener y parsear el job
	log.Debug("fetching job params")
	paramsJSON, err := p.fetchJobParams(ctx, jobID)
	if err != nil {
		return p.failJob(ctx, jobID, errors.Wrap(err, "processor.fetch", "failed to fetch job params"))
	}

	log.Debug("parsing job params")
	parsedJob, err := p.jobParser.Parse(ctx, paramsJSON)
	if err != nil {
		return p.failJob(ctx, jobID, errors.WrapWithCode(err, errors.CodeValidation, "processor.parse", "failed to parse job params"))
	}

	// Validación para jobs v1
	if parsedJob.HasEnvelope {
		avatarID := strings.TrimSpace(parsedJob.Inputs["avatar_image_asset_id"])
		if avatarID == "" {
			return p.failJob(ctx, jobID, errors.ValidationField("inputs.avatar_image_asset_id", "missing required input"))
		}
		log.Debug("v1 job validated", "template_id", parsedJob.TemplateID)
	}

	// 2. Marcar como running
	log.Debug("marking job as running")
	if err := p.markJobRunning(ctx, jobID); err != nil {
		return p.failJob(ctx, jobID, errors.Wrap(err, "processor.status", "failed to mark job as running"))
	}

	// 3. Preparar keys de salida
	outputKeys := GenerateOutputKeys(jobID, parsedJob.CaptionsEnabled())
	log.Debug("output keys generated",
		"video", outputKeys.Video,
		"thumb", outputKeys.Thumb,
		"captions", outputKeys.Captions,
	)

	// 4. Procesar inputs si es necesario
	var inputPaths map[string]string
	if parsedJob.NeedsInputMaterialization() {
		log.Debug("materializing inputs")
		inputPaths, err = p.inputHandler.Materialize(ctx, jobID, parsedJob.Inputs)
		if err != nil {
			return p.failJob(ctx, jobID, errors.Wrap(err, "processor.inputs", "failed to materialize inputs"))
		}
		log.Debug("inputs materialized", "count", len(inputPaths))
	}

	// 5. Renderizar
	log.Info("starting render",
		"v1", parsedJob.UsedV1(),
		"captions", parsedJob.CaptionsEnabled(),
	)
	err = p.rendererAdapter.Render(ctx, RenderRequest{
		JobID:      jobID,
		ParsedJob:  parsedJob,
		InputPaths: inputPaths,
		OutputKeys: outputKeys,
	})
	if err != nil {
		return p.failJob(ctx, jobID, errors.Wrap(err, "processor.render", "render failed"))
	}
	log.Debug("render completed")

	// 6. Registrar outputs
	log.Debug("registering outputs")
	outputResult, err := p.outputHandler.RegisterOutputs(ctx, RegisterOutputsRequest{
		JobID:           jobID,
		OutputKeys:      outputKeys,
		UsedV1:          parsedJob.UsedV1(),
		CaptionsEnabled: parsedJob.CaptionsEnabled(),
	})
	if err != nil {
		return p.failJob(ctx, jobID, errors.Wrap(err, "processor.outputs", "failed to register outputs"))
	}
	log.Debug("outputs registered",
		"video_asset", outputResult.VideoAssetID,
		"thumb_asset", outputResult.ThumbAssetID,
	)

	// 7. Guardar resultado en DB
	log.Debug("saving job output")
	if err := p.saveJobOutput(ctx, jobID, outputResult); err != nil {
		return p.failJob(ctx, jobID, errors.Wrap(err, "processor.save", "failed to save job output"))
	}

	// 8. Limpiar archivos temporales
	p.cleanup.CleanupJob(jobID)
	log.Debug("cleanup completed")

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
	log := p.log.FromContext(ctx).WithJobID(jobID)

	msg := ""
	if cause != nil {
		msg = cause.Error()
		if len(msg) > 2000 {
			msg = msg[:2000]
		}

		// Log with error details
		var galaErr *errors.Error
		if errors.As(cause, &galaErr) {
			log.Error("job failed",
				"code", string(galaErr.Code),
				"op", galaErr.Op,
				"message", galaErr.Message,
			)
		} else {
			log.Error("job failed", "error", msg)
		}
	}

	_, _ = p.pool.Exec(ctx,
		`UPDATE jobs SET status='FAILED', finished_at=NOW(), error_text=$2 WHERE id=$1`,
		jobID, msg,
	)

	return cause
}
