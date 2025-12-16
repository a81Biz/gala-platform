package localfs

import (
    "context"
    "fmt"
    "io"
    "mime"
    "net/http"
    "os"
    "path/filepath"
    "time"

    "gala/internal/ports"
)

// LocalFS implements ports.StorageProvider using the local filesystem.
// It stores objects under a configured root directory.
type LocalFS struct {
    root string
}

func New(root string) *LocalFS {
    return &LocalFS{root: root}
}

func (l *LocalFS) Provider() string { return "localfs" }

func (l *LocalFS) PutObject(ctx context.Context, in ports.PutObjectInput) (ports.PutObjectOutput, error) {
    if in.ObjectKey == "" {
        return ports.PutObjectOutput{}, fmt.Errorf("object_key is required")
    }

    dst := filepath.Join(l.root, filepath.FromSlash(in.ObjectKey))
    if err := os.MkdirAll(filepath.Dir(dst), 0o755); err != nil {
        return ports.PutObjectOutput{}, err
    }

    outF, err := os.Create(dst)
    if err != nil {
        return ports.PutObjectOutput{}, err
    }
    defer outF.Close()

    n, err := io.Copy(outF, in.Reader)
    if err != nil {
        return ports.PutObjectOutput{}, err
    }

    return ports.PutObjectOutput{ObjectKey: in.ObjectKey, Size: n}, nil
}

func (l *LocalFS) GetObject(ctx context.Context, objectKey string) (rc io.ReadCloser, contentType string, size int64, err error) {
    p := filepath.Join(l.root, filepath.FromSlash(objectKey))
    f, err := os.Open(p)
    if err != nil {
        return nil, "", 0, err
    }

    st, statErr := f.Stat()
    if statErr == nil {
        size = st.Size()
    }

    // Prefer extension-based type. If empty, sniff first bytes.
    contentType = mime.TypeByExtension(filepath.Ext(p))
    if contentType == "" {
        buf := make([]byte, 512)
        n, _ := f.Read(buf)
        _, _ = f.Seek(0, 0)
        contentType = http.DetectContentType(buf[:n])
    }

    return f, contentType, size, nil
}

func (l *LocalFS) DeleteObject(ctx context.Context, objectKey string) error {
    p := filepath.Join(l.root, filepath.FromSlash(objectKey))
    return os.Remove(p)
}

func (l *LocalFS) GetSignedURL(ctx context.Context, objectKey string, expiresIn time.Duration) (ports.SignedURLOutput, error) {
    // v0: local provider has no real signed URLs; API currently serves /assets/{id}/content.
    return ports.SignedURLOutput{URL: "", ExpiresAt: time.Now().UTC().Add(expiresIn)}, nil
}
