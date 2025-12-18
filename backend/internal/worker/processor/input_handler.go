package processor

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"gala/internal/ports"
)

type InputHandler struct {
	pool        *pgxpool.Pool
	sp          ports.StorageProvider
	storageRoot string
}

func NewInputHandler(pool *pgxpool.Pool, sp ports.StorageProvider, storageRoot string) *InputHandler {
	return &InputHandler{
		pool:        pool,
		sp:          sp,
		storageRoot: storageRoot,
	}
}

// Materialize descarga y guarda todos los inputs localmente
func (ih *InputHandler) Materialize(ctx context.Context, jobID string, inputs map[string]string) (map[string]string, error) {
	baseDir := filepath.Join(ih.storageRoot, "jobs", jobID, "inputs")
	if err := os.MkdirAll(baseDir, 0o755); err != nil {
		return nil, fmt.Errorf("failed to create inputs directory: %w", err)
	}

	materializedPaths := make(map[string]string)

	for inputName, assetID := range inputs {
		assetID = strings.TrimSpace(assetID)
		if assetID == "" {
			continue
		}

		localPath, err := ih.materializeInput(ctx, baseDir, inputName, assetID)
		if err != nil {
			return nil, err
		}

		materializedPaths[inputName] = localPath
	}

	return materializedPaths, nil
}

func (ih *InputHandler) materializeInput(ctx context.Context, baseDir, inputName, assetID string) (string, error) {
	// Obtener metadata del asset
	asset, err := ih.fetchAsset(ctx, assetID)
	if err != nil {
		return "", fmt.Errorf("input asset not found input=%s asset_id=%s: %w", inputName, assetID, err)
	}

	// Descargar del storage
	rc, err := ih.downloadAsset(ctx, asset.ObjectKey, inputName, assetID)
	if err != nil {
		return "", err
	}
	defer rc.Close()

	// Guardar localmente
	localPath, err := ih.saveToLocal(baseDir, inputName, asset.Mime, rc)
	if err != nil {
		return "", fmt.Errorf("failed to save input locally input=%s: %w", inputName, err)
	}

	return localPath, nil
}

type assetMetadata struct {
	ObjectKey string
	Mime      string
}

func (ih *InputHandler) fetchAsset(ctx context.Context, assetID string) (*assetMetadata, error) {
	var objectKey, mime string
	err := ih.pool.QueryRow(ctx, 
		`SELECT object_key, mime FROM assets WHERE id=$1`, 
		assetID,
	).Scan(&objectKey, &mime)

	if err != nil {
		return nil, err
	}

	return &assetMetadata{
		ObjectKey: objectKey,
		Mime:      mime,
	}, nil
}

func (ih *InputHandler) downloadAsset(ctx context.Context, objectKey, inputName, assetID string) (io.ReadCloser, error) {
	rc, _, _, err := ih.sp.GetObject(ctx, objectKey)
	if err != nil {
		return nil, fmt.Errorf("download input failed input=%s asset_id=%s: %w", inputName, assetID, err)
	}
	return rc, nil
}

func (ih *InputHandler) saveToLocal(baseDir, inputName, mime string, rc io.Reader) (string, error) {
	ext := ExtFromMime(mime)
	filename := SanitizeFilename(inputName) + ext
	localPath := filepath.Join(baseDir, filename)

	f, err := os.Create(localPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	if _, err := io.Copy(f, rc); err != nil {
		return "", err
	}

	return localPath, nil
}
