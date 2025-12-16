package localfs

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"io"
	"mime"
	"os"
	"path/filepath"
	"strings"

	"gala/internal/ports"
)

type Provider struct {
	Root string // /data
}

func New(root string) *Provider { return &Provider{Root: root} }

func (p *Provider) abs(objectKey string) string {
	objectKey = strings.TrimPrefix(objectKey, "/")
	return filepath.Join(p.Root, objectKey)
}

func (p *Provider) PutObject(ctx context.Context, in ports.PutObjectInput) (ports.PutObjectOutput, error) {
	path := p.abs(in.ObjectKey)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		return ports.PutObjectOutput{}, err
	}

	f, err := os.Create(path)
	if err != nil {
		return ports.PutObjectOutput{}, err
	}
	defer f.Close()

	// Copy + hash (opcional)
	h := sha256.New()
	n, err := io.Copy(io.MultiWriter(f, h), in.Reader)
	if err != nil {
		return ports.PutObjectOutput{}, err
	}
	_ = hex.EncodeToString(h.Sum(nil)) // lo usaremos cuando guardemos checksum si quieres

	return ports.PutObjectOutput{ObjectKey: in.ObjectKey, Size: n}, nil
}

func (p *Provider) GetObject(ctx context.Context, objectKey string) (io.ReadCloser, string, int64, error) {
	path := p.abs(objectKey)
	f, err := os.Open(path)
	if err != nil {
		return nil, "", 0, err
	}
	st, err := f.Stat()
	if err != nil {
		_ = f.Close()
		return nil, "", 0, err
	}
	ext := filepath.Ext(path)
	m := mime.TypeByExtension(ext)
	if m == "" {
		m = "application/octet-stream"
	}
	return f, m, st.Size(), nil
}

func (p *Provider) DeleteObject(ctx context.Context, objectKey string) error {
	return os.Remove(p.abs(objectKey))
}
