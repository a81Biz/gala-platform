## Diagrama de componentes

```mermaid
flowchart LR
  U[Usuario] -->|UI| FE[Frontend\nReact (minimal)]
  FE -->|REST| API[Backend API\nGo]

  API --> DB[(Postgres)]
  API --> Q[(Redis Queue)]

  W[Worker\nGo] -->|consume jobs| Q
  W -->|invoke render| R[Renderer Container\nFFmpeg + pipeline]
  R -->|store outputs| SP[Storage Provider\nGoogle Drive (init)\nSwappable: S3/GCS/MinIO]
  API -->|init/finalize upload\nget URLs| SP

  FE -->|direct upload (preferred)| SP
  FE -->|poll job status| API
  FE -->|stream/signed URL| API
  API -->|signed URL / proxy stream| SP

  subgraph Docker Local
    FE
    API
    DB
    Q
    W
    R
  end

  subgraph External
    SP
  end
```

---

## Flujo end-to-end (secuencia)

```mermaid
sequenceDiagram
  autonumber
  participant FE as React Frontend
  participant API as Go API
  participant SP as Storage Provider (Drive)
  participant DB as Postgres
  participant Q as Redis Queue
  participant W as Go Worker
  participant R as Renderer Container

  rect rgb(240,240,240)
  note over FE,SP: 1) Alta de assets pesados (directo a Storage)
  FE->>API: POST /assets/init-upload (metadata)
  API->>SP: Create upload session (resumable)
  SP-->>API: upload_session (url/headers)
  API-->>FE: upload_session
  FE->>SP: Upload file (resumable/chunks)
  FE->>API: POST /assets/finalize (session/fileId)
  API->>DB: Save Asset(provider, object_key, size, mime)
  DB-->>API: OK
  API-->>FE: Asset{id}
  end

  rect rgb(240,240,240)
  note over FE,DB: 2) Crear/editar Modelo y Plantilla
  FE->>API: POST /models (profile + asset_ids)
  API->>DB: Save Model
  API-->>FE: Model{id}

  FE->>API: POST /templates (type + params)
  API->>DB: Save Template
  API-->>FE: Template{id}
  end

  rect rgb(240,240,240)
  note over FE,R: 3) Generar contenido (Job)
  FE->>API: POST /jobs (model_ids, template_id, script, overlays, assets)
  API->>DB: Save Job(status=QUEUED)
  API->>Q: Enqueue(job_id)
  API-->>FE: Job{id,status=QUEUED}

  W->>Q: BRPOP/Consume job_id
  W->>DB: Load Job + Assets + Template + Model presets
  W->>R: Invoke render(job_spec.json)
  R->>SP: Write outputs (mp4, thumb, captions)
  R-->>W: Render result (output object_keys + logs)
  W->>DB: Update Job(status=DONE, output_asset_refs)
  end

  rect rgb(240,240,240)
  note over FE,SP: 4) Biblioteca y descarga
  FE->>API: GET /jobs/{id}
  API->>DB: Read Job
  API-->>FE: Job + output asset ids

  FE->>API: GET /assets/{id}/url
  API->>SP: Create download link (or stream token)
  SP-->>API: signed_url/stream_info
  API-->>FE: signed_url/stream_info
  FE->>SP: Download/Stream MP4
  end
```

---

## Diagrama de datos (ERD conceptual)

```mermaid
erDiagram
  MODEL ||--o{ MODEL_ASSET : has
  ASSET ||--o{ MODEL_ASSET : linked

  TEMPLATE ||--o{ JOB : used_by
  MODEL ||--o{ JOB_MODEL : participates
  JOB ||--o{ JOB_MODEL : includes

  JOB ||--o{ JOB_ASSET_IN : consumes
  ASSET ||--o{ JOB_ASSET_IN : input

  JOB ||--o{ JOB_ASSET_OUT : produces
  ASSET ||--o{ JOB_ASSET_OUT : output

  MODEL {
    string id
    string stage_name
    string tags
    json presets
    datetime created_at
  }

  TEMPLATE {
    string id
    string type
    int duration_ms
    json params
    datetime created_at
  }

  JOB {
    string id
    string status
    string template_id
    json render_params
    datetime created_at
    datetime started_at
    datetime finished_at
  }

  ASSET {
    string id
    string kind
    string provider
    string object_key
    string mime
    int size_bytes
    string checksum
    datetime created_at
  }
```
