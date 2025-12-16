package main

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/redis/go-redis/v9"
)

//
// -------------------- Models internos --------------------
//

type RendererSpec struct {
	JobID  string         `json:"job_id"`
	Params map[string]any `json:"params"`
	Output struct {
		VideoObjectKey string `json:"video_object_key"`
		ThumbObjectKey string `json:"thumb_object_key"`
	} `json:"output"`
}

//
// -------------------- Main --------------------
//

func main() {
	ctx := context.Background()

	dbURL := mustEnv("DATABASE_URL")
	redisAddr := mustEnv("REDIS_ADDR")
	rendererBaseURL := mustEnv("RENDERER_HTTP_BASEURL")
	storageRoot := env("STORAGE_LOCAL_ROOT", "/data")

	pool, err := pgxpool.New(ctx, dbURL)
	if err != nil {
		panic(err)
	}
	defer pool.Close()

	rdb := redis.NewClient(&redis.Options{Addr: redisAddr})
	defer rdb.Close()

	fmt.Println("GALA Worker started")

	for {
		jobID, err := popJob(ctx, rdb)
		if err != nil {
			fmt.Println("queue error:", err)
			time.Sleep(1 * time.Second)
			continue
		}
		if jobID == "" {
			continue
		}

		if err := processJob(ctx, pool, rendererBaseURL, storageRoot, jobID); err != nil {
			fmt.Println("job failed:", jobID, err)
		}
	}
}

//
// -------------------- Job loop --------------------
//

func popJob(ctx context.Context, rdb *redis.Client) (string, error) {
	// BRPOP bloqueante
	res, err := rdb.BRPop(ctx, 0, "gala:jobs").Result()
	if err != nil {
		return "", err
	}
	if len(res) < 2 {
		return "", nil
	}
	return res[1], nil
}

func processJob(
	ctx context.Context,
	pool *pgxpool.Pool,
	rendererBaseURL string,
	storageRoot string,
	jobID string,
) error {

	// Leer params del job
	var paramsJSON string
	err := pool.QueryRow(ctx,
		`SELECT params_json FROM jobs WHERE id=$1`,
		jobID,
	).Scan(&paramsJSON)
	if err != nil {
		return fmt.Errorf("job not found: %w", err)
	}

	var params map[string]any
	_ = json.Unmarshal([]byte(paramsJSON), &params)

	// Marcar RUNNING
	_, _ = pool.Exec(ctx,
		`UPDATE jobs SET status='RUNNING', started_at=NOW() WHERE id=$1`,
		jobID,
	)

	// Construir spec (Hello Render v0)
	spec := RendererSpec{
		JobID:  jobID,
		Params: params,
	}
	spec.Output.VideoObjectKey = fmt.Sprintf("renders/%s/hello.mp4", jobID)
	spec.Output.ThumbObjectKey = fmt.Sprintf("renders/%s/hello.jpg", jobID)

	// Llamar renderer
	if err := callRenderer(rendererBaseURL, spec); err != nil {
		_, _ = pool.Exec(ctx,
			`UPDATE jobs SET status='FAILED', finished_at=NOW() WHERE id=$1`,
			jobID,
		)
		return err
	}

	// Registrar outputs como assets
	videoAssetID, videoSize, err := registerAsset(
		ctx,
		pool,
		storageRoot,
		"render_output",
		"video/mp4",
		spec.Output.VideoObjectKey,
	)
	if err != nil {
		failJob(pool, jobID)
		return err
	}

	thumbAssetID, thumbSize, err := registerAsset(
		ctx,
		pool,
		storageRoot,
		"thumbnail",
		"image/jpeg",
		spec.Output.ThumbObjectKey,
	)
	if err != nil {
		failJob(pool, jobID)
		return err
	}

	// Registrar job_output
	outputID := newID("out")
	_, err = pool.Exec(ctx,
		`INSERT INTO job_outputs (id, job_id, variant, video_asset_id, thumbnail_asset_id)
		 VALUES ($1,$2,1,$3,$4)`,
		outputID, jobID, videoAssetID, thumbAssetID,
	)
	if err != nil {
		failJob(pool, jobID)
		return err
	}

	// Marcar DONE
	_, _ = pool.Exec(ctx,
		`UPDATE jobs SET status='DONE', finished_at=NOW() WHERE id=$1`,
		jobID,
	)

	fmt.Printf(
		"job DONE %s (video=%d bytes, thumb=%d bytes)\n",
		jobID, videoSize, thumbSize,
	)

	return nil
}

//
// -------------------- Renderer --------------------
//

func callRenderer(baseURL string, spec RendererSpec) error {
	body, _ := json.Marshal(spec)

	req, err := http.NewRequest(
		"POST",
		baseURL+"/render",
		bytes.NewReader(body),
	)
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 5 * time.Minute}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 300 {
		return fmt.Errorf("renderer returned status %s", resp.Status)
	}
	return nil
}

//
// -------------------- Assets helpers --------------------
//

func registerAsset(
	ctx context.Context,
	pool *pgxpool.Pool,
	storageRoot string,
	kind string,
	mime string,
	objectKey string,
) (assetID string, size int64, err error) {

	path := filepath.Join(storageRoot, objectKey)
	st, err := os.Stat(path)
	if err != nil {
		return "", 0, err
	}

	assetID = newID("ast")
	size = st.Size()

	_, err = pool.Exec(ctx,
		`INSERT INTO assets (id, kind, provider, object_key, mime, size_bytes)
		 VALUES ($1,$2,'localfs',$3,$4,$5)`,
		assetID, kind, objectKey, mime, size,
	)
	if err != nil {
		return "", 0, err
	}

	return assetID, size, nil
}

func failJob(pool *pgxpool.Pool, jobID string) {
	_, _ = pool.Exec(
		context.Background(),
		`UPDATE jobs SET status='FAILED', finished_at=NOW() WHERE id=$1`,
		jobID,
	)
}

//
// -------------------- Helpers --------------------
//

func env(k, def string) string {
	v := os.Getenv(k)
	if v == "" {
		return def
	}
	return v
}

func mustEnv(k string) string {
	v := os.Getenv(k)
	if v == "" {
		panic("missing env: " + k)
	}
	return v
}

func newID(prefix string) string {
	return fmt.Sprintf("%s_%d", prefix, time.Now().UnixNano())
}
