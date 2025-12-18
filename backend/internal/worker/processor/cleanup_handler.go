package processor

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"

	"gala/internal/ports"
)

type Cleanup struct {
	storageRoot  string
	cleanupLocal bool
	sp           ports.StorageProvider
}

func NewCleanup(storageRoot string, cleanupLocal bool, sp ports.StorageProvider) *Cleanup {
	return &Cleanup{
		storageRoot:  storageRoot,
		cleanupLocal: cleanupLocal,
		sp:           sp,
	}
}

// CleanupJob limpia los archivos temporales del job
func (c *Cleanup) CleanupJob(jobID string) {
	if !c.shouldCleanup() {
		return
	}

	// Solo limpiar la carpeta de renders, no otras carpetas del job
	jobDir := filepath.Join(c.storageRoot, "renders", jobID)
	
	err := os.Remove(jobDir)
	if err == nil || os.IsNotExist(err) {
		return
	}

	// Ignorar errores de directorio no vacío
	if errors.Is(err, syscall.ENOTEMPTY) || errors.Is(err, syscall.EEXIST) {
		return
	}
}

func (c *Cleanup) shouldCleanup() bool {
	return c.cleanupLocal && c.sp.Provider() == "gdrive"
}
