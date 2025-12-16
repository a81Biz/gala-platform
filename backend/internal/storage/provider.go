package storage

import "gala/internal/ports"

// Provider is the storage contract used across API and Worker.
// It is an alias to ports.StorageProvider to keep call-sites simple.
type Provider = ports.StorageProvider
