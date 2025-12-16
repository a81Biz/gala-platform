# API Contract v0 — Plataforma GALA

**Base URL (local):** `http://localhost:8080`
**Formato:** JSON (`application/json`)
**Autenticación (v0):** Ninguna (solo local). En v1 se agrega auth.

---

## Convenciones generales

### Respuesta estándar de error

```json
{
  "error": {
    "code": "string",
    "message": "string",
    "details": { "any": "json" }
  }
}
```

### Estados de Job (v0)

* `QUEUED`
* `RUNNING`
* `DONE`
* `FAILED`

### Providers de storage (v0)

* `localfs` (implementación inicial)

---

## 1) Health

### GET `/health`

**200**

```json
{ "status": "ok", "service": "gala-api", "version": "0.1.0" }
```

---

## 2) Assets (archivos pesados)

> En v0 soportamos **upload simple** (el archivo viaja al API).
> En v1 se cambia a `init-upload/finalize` para upload directo al provider.

### POST `/assets`

**Content-Type:** `multipart/form-data`
**Campos:**

* `file` (binary, required)
* `kind` (string, required)
  Valores sugeridos: `source_video | avatar_input | music | render_output | thumbnail | overlay | background`
* `label` (string, optional)

**201**

```json
{
  "asset": {
    "id": "ast_01J...",
    "kind": "source_video",
    "provider": "localfs",
    "object_key": "assets/ast_01J.../original.mp4",
    "mime": "video/mp4",
    "size_bytes": 12345678,
    "checksum": "sha256:...",
    "label": "trend-source",
    "created_at": "2025-12-15T00:00:00Z"
  }
}
```

### GET `/assets/{assetId}`

**200**

```json
{
  "asset": {
    "id": "ast_01J...",
    "kind": "music",
    "provider": "localfs",
    "object_key": "assets/ast_01J.../audio.mp3",
    "mime": "audio/mpeg",
    "size_bytes": 555000,
    "checksum": "sha256:...",
    "label": "bgm-01",
    "created_at": "2025-12-15T00:00:00Z"
  }
}
```

### GET `/assets/{assetId}/url`

Devuelve una URL para descargar/stream (en local puede ser URL directa del API como proxy).

**200**

```json
{
  "asset_id": "ast_01J...",
  "url": "http://localhost:8080/assets/ast_01J.../content",
  "expires_at": "2025-12-15T00:30:00Z"
}
```

### GET `/assets/{assetId}/content`

Sirve el binario del asset (stream).

* **200**: contenido binario
* **404**: no existe

### DELETE `/assets/{assetId}`

**204** (sin body)

Errores típicos:

* `ASSET_NOT_FOUND` (404)
* `ASSET_IN_USE` (409) si está ligado a jobs/modelos (v0 opcional)

---

## 3) Models (avatares)

### POST `/models`

**Body**

```json
{
  "stage_name": "Model A",
  "tags": ["fashion", "tiktok"],
  "preset": {
    "aspect_ratio": "9:16",
    "default_bg_asset_id": "ast_...",
    "default_overlay_style": "minimal"
  },
  "asset_ids": ["ast_...", "ast_..."]
}
```

**201**

```json
{
  "model": {
    "id": "mdl_01J...",
    "stage_name": "Model A",
    "tags": ["fashion", "tiktok"],
    "preset": { "aspect_ratio": "9:16" },
    "asset_ids": ["ast_...", "ast_..."],
    "created_at": "2025-12-15T00:00:00Z"
  }
}
```

### GET `/models`

**Query (opcionales):**

* `q` (string) búsqueda por nombre/tag
* `tag` (string)

**200**

```json
{
  "models": [
    { "id": "mdl_01J...", "stage_name": "Model A", "tags": ["fashion"], "created_at": "..." }
  ]
}
```

### GET `/models/{modelId}`

**200**

```json
{
  "model": {
    "id": "mdl_01J...",
    "stage_name": "Model A",
    "tags": ["fashion"],
    "preset": { "aspect_ratio": "9:16" },
    "asset_ids": ["ast_..."],
    "created_at": "..."
  }
}
```

### PATCH `/models/{modelId}`

**Body (parcial)**

```json
{
  "stage_name": "Model A (v2)",
  "tags": ["fashion", "beauty"],
  "preset": { "default_overlay_style": "bold" }
}
```

**200** (devuelve `model` actualizado)

### DELETE `/models/{modelId}`

**204**

Errores típicos:

* `MODEL_NOT_FOUND` (404)
* `MODEL_IN_USE` (409) si tiene jobs ligados

---

## 4) Templates (plantillas)

### POST `/templates`

**Body**

```json
{
  "type": "hello_render",
  "name": "Hello Render v0",
  "duration_ms": 7000,
  "format": { "width": 1080, "height": 1920, "fps": 30 },
  "params_schema": {
    "text": { "type": "string", "max": 80 },
    "bg_asset_id": { "type": "string", "nullable": true }
  },
  "defaults": {
    "text": "GALA",
    "bg_asset_id": null
  }
}
```

**201**

```json
{
  "template": {
    "id": "tpl_01J...",
    "type": "hello_render",
    "name": "Hello Render v0",
    "duration_ms": 7000,
    "format": { "width": 1080, "height": 1920, "fps": 30 },
    "params_schema": { "text": { "type": "string" } },
    "defaults": { "text": "GALA" },
    "created_at": "..."
  }
}
```

### GET `/templates`

**200**

```json
{ "templates": [ { "id": "tpl_01J...", "type": "hello_render", "name": "Hello Render v0" } ] }
```

### GET `/templates/{templateId}`

**200** `{ "template": { ... } }`

### PATCH `/templates/{templateId}`

**200** `{ "template": { ... } }`

### DELETE `/templates/{templateId}`

**204**

Errores típicos:

* `TEMPLATE_NOT_FOUND` (404)
* `TEMPLATE_IN_USE` (409)

---

## 5) Jobs (renderizado)

### POST `/jobs`

Crea un job de render por lote.

**Body**

```json
{
  "name": "Hello Batch 01",
  "template_id": "tpl_01J...",
  "model_ids": ["mdl_01J..."],
  "inputs": {
    "bg_asset_id": null,
    "music_asset_id": null,
    "overlay_asset_id": null
  },
  "params": {
    "text": "Hola TikTok — GALA v0"
  },
  "output": {
    "aspect_ratio": "9:16",
    "format": { "width": 1080, "height": 1920, "fps": 30 }
  },
  "batch": {
    "count": 1,
    "variants": []
  }
}
```

**201**

```json
{
  "job": {
    "id": "job_01J...",
    "name": "Hello Batch 01",
    "status": "QUEUED",
    "template_id": "tpl_01J...",
    "model_ids": ["mdl_01J..."],
    "params": { "text": "Hola TikTok — GALA v0" },
    "created_at": "2025-12-15T00:00:00Z"
  }
}
```

### GET `/jobs`

**Query opcionales:**

* `status=QUEUED|RUNNING|DONE|FAILED`
* `template_id=...`
* `model_id=...`

**200**

```json
{
  "jobs": [
    { "id": "job_01J...", "name": "Hello Batch 01", "status": "DONE", "created_at": "..." }
  ]
}
```

### GET `/jobs/{jobId}`

Incluye outputs (cuando existen) y logs básicos.

**200**

```json
{
  "job": {
    "id": "job_01J...",
    "name": "Hello Batch 01",
    "status": "DONE",
    "template_id": "tpl_01J...",
    "model_ids": ["mdl_01J..."],
    "params": { "text": "Hola TikTok — GALA v0" },
    "outputs": [
      {
        "variant": 1,
        "video_asset_id": "ast_01J_vid...",
        "thumbnail_asset_id": "ast_01J_th...",
        "captions_asset_id": "ast_01J_cap..."
      }
    ],
    "logs": [
      { "ts": "2025-12-15T00:00:03Z", "level": "info", "msg": "Renderer started" }
    ],
    "created_at": "...",
    "started_at": "...",
    "finished_at": "..."
  }
}
```

### POST `/jobs/{jobId}/cancel`

(v0 opcional) marca como cancelado si aún no corre.
**200**

```json
{ "job": { "id": "job_01J...", "status": "FAILED", "reason": "CANCELLED" } }
```

Errores típicos:

* `JOB_NOT_FOUND` (404)
* `INVALID_JOB_STATE` (409)

---

## 6) Renderer (interno, no público en v0)

En v0, el renderer puede operar de 2 formas:

1. **HTTP interno** (recomendado para desacoplar)
2. **exec script** desde worker (simple, pero más acoplado)

### (Si usas HTTP interno) POST `http://renderer:9000/render`

**Body (job_spec)**

```json
{
  "job_id": "job_01J...",
  "template_type": "hello_render",
  "input": {
    "bg_asset_object_key": null,
    "music_asset_object_key": null
  },
  "params": {
    "text": "Hola TikTok — GALA v0"
  },
  "output": {
    "video_object_key": "renders/job_01J.../v1.mp4",
    "thumb_object_key": "renders/job_01J.../v1.jpg",
    "captions_object_key": "renders/job_01J.../v1.srt",
    "format": { "width": 1080, "height": 1920, "fps": 30 }
  }
}
```

**200**

```json
{
  "status": "ok",
  "outputs": {
    "video_object_key": "renders/job_01J.../v1.mp4",
    "thumb_object_key": "renders/job_01J.../v1.jpg",
    "captions_object_key": "renders/job_01J.../v1.srt"
  },
  "metrics": { "render_ms": 4200 }
}
```

---

## Códigos de error sugeridos (v0)

* `VALIDATION_ERROR` (400)
* `NOT_FOUND` (404)
* `CONFLICT` (409)
* `INTERNAL_ERROR` (500)
* Específicos: `ASSET_NOT_FOUND`, `MODEL_NOT_FOUND`, `TEMPLATE_NOT_FOUND`, `JOB_NOT_FOUND`

---