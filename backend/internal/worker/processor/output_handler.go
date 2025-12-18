package processor

import (
	"context"
	"fmt"
	"os"
	"path/filepath"

	"github.com/jackc/pgx/v5/pgxpool"

	"gala/internal/ports"
	"gala/internal/worker/util"
)

type OutputHandler struct {
	pool         *pgxpool.Pool
	sp           ports.StorageProvider
	storageRoot  string
	cleanupLocal bool
}

func NewOutputHandler(pool *pgxpool.Pool, sp ports.StorageProvider, storageRoot string, cleanupLocal bool) *OutputHandler {
	return &OutputHandler{
		pool:         pool,
		sp:           sp,
		storageRoot:  storageRoot,
		cleanupLocal: cleanupLocal,
	}
}

type RegisterOutputsRequest struct {
	JobID           string
	OutputKeys      *OutputKeys
	UsedV1          bool
	CaptionsEnabled bool
}

type OutputResult struct {
	OutputID        string
	VideoAssetID    string
	ThumbAssetID    string
	CaptionsAssetID string
}

// RegisterOutputs sube y registra todos los outputs generados
func (oh *OutputHandler) RegisterOutputs(ctx context.Context, req RegisterOutputsRequest) (*OutputResult, error) {
	result := &OutputResult{
		OutputID: util.NewID("out"),
	}

	// Registrar video
	videoAssetID, _, err := oh.registerAsset(ctx, "render_output", "video/mp4", req.OutputKeys.Video)
	if err != nil {
		return nil, fmt.Errorf("failed to register video: %w", err)
	}
	result.VideoAssetID = videoAssetID

	// Registrar thumbnail
	thumbAssetID, _, err := oh.registerAsset(ctx, "thumbnail", "image/jpeg", req.OutputKeys.Thumb)
	if err != nil {
		return nil, fmt.Errorf("failed to register thumbnail: %w", err)
	}
	result.ThumbAssetID = thumbAssetID

	// Registrar captions si aplica
	if req.UsedV1 && req.CaptionsEnabled && req.OutputKeys.Captions != "" {
		if oh.captionsFileExists(req.OutputKeys.Captions) {
			captionsAssetID, _, err := oh.registerAsset(ctx, "captions", "text/vtt", req.OutputKeys.Captions)
			if err != nil {
				return nil, fmt.Errorf("failed to register captions: %w", err)
			}
			result.CaptionsAssetID = captionsAssetID
		}
	}

	return result, nil
}

func (oh *OutputHandler) captionsFileExists(captionsKey string) bool {
	localPath := filepath.Join(oh.storageRoot, captionsKey)
	_, err := os.Stat(localPath)
	return err == nil
}

func (oh *OutputHandler) registerAsset(ctx context.Context, kind, mime, objectKey string) (assetID string, size int64, err error) {
	// Obtener archivo local
	localPath := filepath.Join(oh.storageRoot, objectKey)
	st, err := os.Stat(localPath)
	if err != nil {
		return "", 0, fmt.Errorf("asset file not found: %w", err)
	}

	f, err := os.Open(localPath)
	if err != nil {
		return "", 0, fmt.Errorf("failed to open asset: %w", err)
	}
	defer f.Close()

	// Subir a storage
	uploadResult, err := oh.sp.PutObject(ctx, ports.PutObjectInput{
		ObjectKey:   objectKey,
		ContentType: mime,
		Reader:      f,
		Size:        st.Size(),
	})
	if err != nil {
		return "", 0, fmt.Errorf("failed to upload asset: %w", err)
	}

	// Registrar en DB
	assetID = util.NewID("ast")
	_, err = oh.pool.Exec(ctx,
		`INSERT INTO assets (id, kind, provider, object_key, mime, size_bytes)
		 VALUES ($1,$2,$3,$4,$5,$6)`,
		assetID, kind, oh.sp.Provider(), uploadResult.ObjectKey, mime, uploadResult.Size,
	)
	if err != nil {
		return "", 0, fmt.Errorf("failed to register asset in DB: %w", err)
	}

	// Limpiar archivo local si corresponde
	oh.maybeCleanupFile(objectKey)

	return assetID, uploadResult.Size, nil
}

func (oh *OutputHandler) maybeCleanupFile(objectKey string) {
	if !oh.cleanupLocal || oh.sp.Provider() != "gdrive" {
		return
	}
	_ = os.Remove(filepath.Join(oh.storageRoot, objectKey))
}
