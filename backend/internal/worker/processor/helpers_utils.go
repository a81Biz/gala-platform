package processor

import (
	"fmt"
	"strings"
)

// OutputKeys contiene las claves de objeto para los outputs
type OutputKeys struct {
	Video    string
	Thumb    string
	Captions string
}

// GenerateOutputKeys crea las claves de objeto para los outputs del job
func GenerateOutputKeys(jobID string, captionsEnabled bool) *OutputKeys {
	keys := &OutputKeys{
		Video: fmt.Sprintf("renders/%s/hello.mp4", jobID),
		Thumb: fmt.Sprintf("renders/%s/hello.jpg", jobID),
	}

	if captionsEnabled {
		keys.Captions = fmt.Sprintf("renders/%s/captions.vtt", jobID)
	}

	return keys
}

// IsTruthy evalúa si un valor debe considerarse verdadero
func IsTruthy(v any) bool {
	switch t := v.(type) {
	case bool:
		return t
	case float64:
		return t == 1
	case int:
		return t == 1
	case int64:
		return t == 1
	case string:
		s := strings.TrimSpace(strings.ToLower(t))
		return s == "1" || s == "true" || s == "yes" || s == "on"
	default:
		return false
	}
}

// NullIfEmpty retorna nil si el string está vacío, útil para campos nullable en DB
func NullIfEmpty(s string) any {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	return s
}

// SanitizeFilename limpia un nombre de archivo de caracteres peligrosos
func SanitizeFilename(s string) string {
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

// ExtFromMime retorna la extensión de archivo apropiada para un MIME type
func ExtFromMime(mime string) string {
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
	case "text/vtt":
		return ".vtt"
	default:
		return ""
	}
}
