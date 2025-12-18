package processor

import (
	"context"

	contracts "gala/internal/contracts/renderer/v0"
	"gala/internal/worker/renderer"
)

type RendererAdapter struct {
	client renderer.Client
}

func NewRendererAdapter(client renderer.Client) *RendererAdapter {
	return &RendererAdapter{client: client}
}

type RenderRequest struct {
	JobID      string
	ParsedJob  *ParsedJob
	InputPaths map[string]string
	OutputKeys *OutputKeys
}

// Render adapta entre v0 y v1 del renderer según el tipo de job
func (ra *RendererAdapter) Render(ctx context.Context, req RenderRequest) error {
	if req.ParsedJob.UsedV1() {
		return ra.renderV1(req)
	}
	return ra.renderV0(req)
}

func (ra *RendererAdapter) renderV1(req RenderRequest) error {
	outBlock := map[string]any{
		"video_object_key": req.OutputKeys.Video,
		"thumb_object_key": req.OutputKeys.Thumb,
	}

	if req.ParsedJob.CaptionsEnabled() {
		outBlock["captions_object_key"] = req.OutputKeys.Captions
	}

	specV1 := map[string]any{
		"job_id":      req.JobID,
		"template_id": req.ParsedJob.TemplateID,
		"inputs":      req.InputPaths,
		"params":      req.ParsedJob.MergedParams,
		"output":      outBlock,
	}

	return ra.client.RenderV1(specV1)
}

func (ra *RendererAdapter) renderV0(req RenderRequest) error {
	spec := contracts.RendererSpec{
		JobID:  req.JobID,
		Params: req.ParsedJob.MergedParams,
	}
	spec.Output.VideoObjectKey = req.OutputKeys.Video
	spec.Output.ThumbObjectKey = req.OutputKeys.Thumb

	return ra.client.Render(spec)
}
