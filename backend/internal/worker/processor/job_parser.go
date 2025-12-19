package processor

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"
)

type ParsedJob struct {
	TemplateID   string
	Inputs       map[string]string
	Params       map[string]any
	MergedParams map[string]any
	HasEnvelope  bool
}

func (j *ParsedJob) UsedV1() bool {
	// Regla de negocio: si hay envelope (template_id), es v1.
	// No depender de inputs para evitar fallback silencioso.
	return j.HasEnvelope
}

func (j *ParsedJob) CaptionsEnabled() bool {
	return IsTruthy(j.MergedParams["captions"])
}

func (j *ParsedJob) NeedsInputMaterialization() bool {
	// Si es v1, los inputs son asset IDs y deben materializarse a paths locales.
	return j.HasEnvelope
}

type JobParser struct {
	pool *pgxpool.Pool
}

func NewJobParser(pool *pgxpool.Pool) *JobParser {
	return &JobParser{pool: pool}
}

func (jp *JobParser) Parse(ctx context.Context, paramsJSON string) (*ParsedJob, error) {
	var raw map[string]any
	if err := json.Unmarshal([]byte(paramsJSON), &raw); err != nil {
		return nil, fmt.Errorf("invalid params_json: %w", err)
	}

	j := &ParsedJob{
		Inputs:       make(map[string]string),
		Params:       make(map[string]any),
		MergedParams: make(map[string]any),
	}

	// Detectar si usa envelope (v1) o formato legacy (v0)
	if templateID, ok := raw["template_id"].(string); ok && strings.TrimSpace(templateID) != "" {
		return jp.parseEnvelopeFormat(ctx, raw, j, strings.TrimSpace(templateID))
	}

	return jp.parseLegacyFormat(raw, j)
}

func (jp *JobParser) parseEnvelopeFormat(ctx context.Context, raw map[string]any, j *ParsedJob, templateID string) (*ParsedJob, error) {
	j.HasEnvelope = true
	j.TemplateID = templateID

	// Extraer params del envelope
	if pm, ok := raw["params"].(map[string]any); ok && pm != nil {
		j.Params = pm
	}

	// Extraer inputs del envelope
	if im, ok := raw["inputs"].(map[string]any); ok && im != nil {
		for k, vv := range im {
			if s, ok := vv.(string); ok && strings.TrimSpace(s) != "" {
				j.Inputs[k] = strings.TrimSpace(s)
			}
		}
	}

	// Obtener defaults del template
	defaults, err := jp.fetchTemplateDefaults(ctx, templateID)
	if err != nil {
		return nil, err
	}

	// Merge: defaults -> params del job
	j.MergedParams = mergeMaps(defaults, j.Params)

	// Validar campo requerido
	if !hasValidText(j.MergedParams) {
		return nil, fmt.Errorf("params.text is required (after defaults merge)")
	}

	return j, nil
}

func (jp *JobParser) parseLegacyFormat(raw map[string]any, j *ParsedJob) (*ParsedJob, error) {
	// En formato legacy, todos los campos van directo a MergedParams
	for k, v := range raw {
		j.MergedParams[k] = v
	}

	if !hasValidText(j.MergedParams) {
		return nil, fmt.Errorf("params.text is required")
	}

	return j, nil
}

func (jp *JobParser) fetchTemplateDefaults(ctx context.Context, templateID string) (map[string]any, error) {
	var defaultsBytes []byte
	err := jp.pool.QueryRow(ctx,
		`SELECT COALESCE(defaults, '{}'::jsonb) FROM templates WHERE id=$1 AND deleted_at IS NULL`,
		templateID,
	).Scan(&defaultsBytes)
	if err != nil {
		return nil, fmt.Errorf("template not found: %s", templateID)
	}

	defaults := make(map[string]any)
	if err := json.Unmarshal(defaultsBytes, &defaults); err != nil {
		return nil, fmt.Errorf("invalid template defaults: %w", err)
	}

	return defaults, nil
}

func hasValidText(params map[string]any) bool {
	t, ok := params["text"].(string)
	return ok && strings.TrimSpace(t) != ""
}

func mergeMaps(base, override map[string]any) map[string]any {
	result := make(map[string]any)
	for k, v := range base {
		result[k] = v
	}
	for k, v := range override {
		result[k] = v
	}
	return result
}
