package processor

type RendererSpec struct {
	JobID  string         `json:"job_id"`
	Params map[string]any `json:"params"`
	Output struct {
		VideoObjectKey string `json:"video_object_key"`
		ThumbObjectKey string `json:"thumb_object_key"`
	} `json:"output"`
}
