package storage

import (
	"context"
	"fmt"
	"os"

	"gala/internal/adapters/storage/gdrive"
	"gala/internal/adapters/storage/localfs"

	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
	drive "google.golang.org/api/drive/v3"
	"google.golang.org/api/option"
)

func NewProvider() (Provider, error) {
	provider := os.Getenv("STORAGE_PROVIDER")
	if provider == "" {
		provider = "localfs"
	}

	switch provider {
	case "localfs":
		root := mustEnv("STORAGE_LOCAL_ROOT")
		return localfs.New(root), nil

	case "gdrive":
		return newGDriveProvider()

	default:
		return nil, fmt.Errorf("unknown storage provider: %s", provider)
	}
}

func newGDriveProvider() (Provider, error) {
	ctx := context.Background()

	clientID := mustEnv("GDRIVE_CLIENT_ID")
	clientSecret := mustEnv("GDRIVE_CLIENT_SECRET")
	refreshToken := mustEnv("GDRIVE_REFRESH_TOKEN")
	folderID := os.Getenv("GDRIVE_FOLDER_ID")

	conf := &oauth2.Config{
		ClientID:     clientID,
		ClientSecret: clientSecret,
		Endpoint:     google.Endpoint,
		Scopes:       []string{drive.DriveFileScope},
	}

	tok := &oauth2.Token{RefreshToken: refreshToken}
	httpClient := conf.Client(ctx, tok)

	srv, err := drive.NewService(ctx, option.WithHTTPClient(httpClient))
	if err != nil {
		return nil, err
	}

	return gdrive.NewClient(srv, folderID), nil
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic("missing env: " + k)
	}
	return v
}
