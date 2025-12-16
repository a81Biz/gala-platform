package ports

import (
	"context"
	"io"
)

type PutObjectInput struct {
	ObjectKey   string
	ContentType string
	Reader      io.Reader
	Size        int64
}

type PutObjectOutput struct {
	ObjectKey string
	Size      int64
}

type StorageProvider interface {
	PutObject(ctx context.Context, in PutObjectInput) (PutObjectOutput, error)
	GetObject(ctx context.Context, objectKey string) (io.ReadCloser, string, int64, error) // body, mime, size
	DeleteObject(ctx context.Context, objectKey string) error
}
