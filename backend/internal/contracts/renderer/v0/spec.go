package v0

// RendererSpec v0: contrato mínimo para el renderer HTTP.
// - job_id: identificador del job
// - params: parámetros libres (Hello Render usa params.text)
// - output: rutas (object keys) donde el renderer debe escribir en el storage compartido
type RendererSpec struct {
	JobID  string         `json:"job_id"`
	Params map[string]any `json:"params"`
	Output struct {
		VideoObjectKey string `json:"video_object_key"`
		ThumbObjectKey string `json:"thumb_object_key"`
	} `json:"output"`
}
