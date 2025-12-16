package gdrive

import (
    "context"
    "fmt"
    "io"
    "time"

    "gala/internal/ports"

    "google.golang.org/api/drive/v3"
    "google.golang.org/api/googleapi"
)

// Client implements ports.StorageProvider backed by Google Drive.
// ObjectKey is stored as the Drive fileId for retrieval/deletion.
// For uploads we use the provided ObjectKey as the Drive file Name.
type Client struct {
    srv      *drive.Service
    folderID string
}

func NewClient(srv *drive.Service, folderID string) *Client {
    return &Client{srv: srv, folderID: folderID}
}

func (c *Client) Provider() string { return "gdrive" }

func (c *Client) PutObject(ctx context.Context, in ports.PutObjectInput) (ports.PutObjectOutput, error) {
    if in.ObjectKey == "" {
        return ports.PutObjectOutput{}, fmt.Errorf("object_key is required")
    }

    file := &drive.File{Name: in.ObjectKey}
    if c.folderID != "" {
        file.Parents = []string{c.folderID}
    }

    call := c.srv.Files.Create(file)
    if in.ContentType != "" {
        call = call.Media(in.Reader, googleapi.ContentType(in.ContentType))
    } else {
        call = call.Media(in.Reader)
    }

    created, err := call.Context(ctx).Do()
    if err != nil {
        return ports.PutObjectOutput{}, fmt.Errorf("gdrive upload failed: %w", err)
    }

    // We return the Drive fileId as ObjectKey, so later Get/Delete use it.
    return ports.PutObjectOutput{ObjectKey: created.Id, Size: in.Size}, nil
}

func (c *Client) GetObject(ctx context.Context, objectKey string) (rc io.ReadCloser, contentType string, size int64, err error) {
    resp, err := c.srv.Files.Get(objectKey).
        SupportsAllDrives(true).
        Download()
    if err != nil {
        return nil, "", 0, err
    }

    contentType = resp.Header.Get("Content-Type")
    size = resp.ContentLength
    return resp.Body, contentType, size, nil
}

func (c *Client) DeleteObject(ctx context.Context, objectKey string) error {
    return c.srv.Files.Delete(objectKey).
        SupportsAllDrives(true).
        Context(ctx).
        Do()
}

func (c *Client) GetSignedURL(ctx context.Context, objectKey string, expiresIn time.Duration) (ports.SignedURLOutput, error) {
    // v0: we don't generate signed URLs for Drive in this iteration.
    return ports.SignedURLOutput{URL: "", ExpiresAt: time.Now().UTC().Add(expiresIn)}, nil
}
