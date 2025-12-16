package ports

import (
	"context"
	"io"
	"time"
)

type PutObjectInput struct {
	ObjectKey   string
	ContentType string
	Reader      io.Reader
	Size        int64
}

type PutObjectOutput struct {
	// En localfs será el mismo object_key.
	// En gdrive será el fileId real (para poder leer/stream después).
	ObjectKey string
	Size      int64
}

type SignedURLOutput struct {
	URL       string
	ExpiresAt time.Time
}

// StorageProvider: implementaciones (localfs, gdrive, s3, etc.)
type StorageProvider interface {
	Provider() string

	PutObject(ctx context.Context, in PutObjectInput) (PutObjectOutput, error)
	GetObject(ctx context.Context, objectKey string) (rc io.ReadCloser, contentType string, size int64, err error)
	DeleteObject(ctx context.Context, objectKey string) error

	// v0: opcional. (API hoy puede seguir usando /assets/{id}/content)
	GetSignedURL(ctx context.Context, objectKey string, expiresIn time.Duration) (SignedURLOutput, error)
}
