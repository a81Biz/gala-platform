package httpkit

import (
	"net/http"
	"strings"
)

type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAgeSeconds    int
	DebugHeader      bool // agrega X-CORS-Debug para validar rápido en dev
}

func CORS(opt CORSOptions) func(http.Handler) http.Handler {
	if len(opt.AllowedMethods) == 0 {
		opt.AllowedMethods = []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"}
	}
	if len(opt.AllowedHeaders) == 0 {
		opt.AllowedHeaders = []string{"Content-Type", "Authorization", "Accept"}
	}
	if opt.MaxAgeSeconds == 0 {
		opt.MaxAgeSeconds = 600
	}

	allowedMethods := strings.Join(opt.AllowedMethods, ", ")
	allowedHeaders := strings.Join(opt.AllowedHeaders, ", ")
	exposedHeaders := strings.Join(opt.ExposedHeaders, ", ")

	allowedOrigins := normalizeList(opt.AllowedOrigins)

	isAllowedOrigin := func(origin string) bool {
		if origin == "" {
			return false
		}
		for _, o := range allowedOrigins {
			if o == "*" || o == origin {
				return true
			}
		}
		return false
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			allowed := isAllowedOrigin(origin)

			if opt.DebugHeader {
				w.Header().Set("X-CORS-Debug", "origin="+origin+" allowed="+boolToStr(allowed))
			}

			if origin != "" && allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Add("Vary", "Origin")

				w.Header().Set("Access-Control-Allow-Methods", allowedMethods)
				w.Header().Set("Access-Control-Allow-Headers", allowedHeaders)
				w.Header().Set("Access-Control-Max-Age", intToString(opt.MaxAgeSeconds))

				if exposedHeaders != "" {
					w.Header().Set("Access-Control-Expose-Headers", exposedHeaders)
				}
				if opt.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

func normalizeList(in []string) []string {
	out := make([]string, 0, len(in))
	for _, s := range in {
		s = strings.TrimSpace(s)
		if s != "" {
			out = append(out, s)
		}
	}
	return out
}

func boolToStr(v bool) string {
	if v {
		return "true"
	}
	return "false"
}

func intToString(v int) string {
	if v == 0 {
		return "0"
	}
	neg := v < 0
	if neg {
		v = -v
	}
	var b [32]byte
	i := len(b)
	for v > 0 {
		i--
		b[i] = byte('0' + (v % 10))
		v /= 10
	}
	if neg {
		i--
		b[i] = '-'
	}
	return string(b[i:])
}
