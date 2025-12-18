# GALA Renderer

Servicio de renderizado desacoplado para la Plataforma GALA.

## 📁 Estructura del Proyecto

```
renderer/
├── server.py              # HTTP server (solo routing)
├── config.py              # Configuración centralizada
├── Dockerfile             # Imagen Docker
├── README.md
├── core/                  # Lógica de negocio
│   ├── __init__.py
│   ├── spec_parser.py     # Parsing y validación de specs
│   ├── video_ops.py       # Operaciones FFmpeg
│   ├── captions.py        # Generación de VTT
│   └── file_utils.py      # Utilidades de archivos
└── handlers/              # Handlers de endpoints
    ├── __init__.py
    ├── render_v0.py       # Handler legacy (video vertical)
    └── render_v1.py       # Handler moderno (avatar + audio)
```

## 🎯 Responsabilidad

- Recibir un `RendererSpec` (v0 o v1)
- Ejecutar un pipeline de render
- Generar archivos audiovisuales
- **NO conoce** jobs, assets, DB ni API

## 🚀 Endpoints

### POST /render (v0 - Legacy)
Genera video vertical negro con texto centrado.

**Request:**
```json
{
  "job_id": "job_123",
  "params": {
    "text": "Mi texto"
  },
  "output": {
    "video_object_key": "renders/job_123/video.mp4",
    "thumb_object_key": "renders/job_123/thumb.jpg"
  }
}
```

### POST /render/v1 (Moderno)
Genera video desde avatar con audio y captions opcionales.

**Request:**
```json
{
  "job_id": "job_123",
  "template_id": "tmpl_456",
  "inputs": {
    "avatar_image_asset_id": "/data/jobs/job_123/inputs/avatar.jpg",
    "voice_audio_asset_id": "/data/jobs/job_123/inputs/audio.mp3"
  },
  "params": {
    "text": "Mi texto",
    "captions": true
  },
  "output": {
    "video_object_key": "renders/job_123/video.mp4",
    "thumb_object_key": "renders/job_123/thumb.jpg",
    "captions_object_key": "renders/job_123/captions.vtt"
  }
}
```

## 🔧 Configuración

Variables de entorno:
- `RENDERER_PORT`: Puerto HTTP (default: 9000)
- `STORAGE_LOCAL_ROOT`: Raíz del storage compartido (default: /data)

## 📦 Output

Escribe archivos en el storage compartido (`/data`):
- Videos en formato MP4
- Thumbnails en formato JPG
- Captions en formato VTT (v1 opcional)

## 🏗️ Arquitectura

### Filosofía
Renderer = función pura: `input → procesamiento → output`

### Separación de responsabilidades
- **server.py**: Solo HTTP routing
- **handlers/**: Lógica de cada endpoint
- **core/**: Módulos reutilizables
- **config.py**: Configuración centralizada

### Agregar nuevas funcionalidades

1. **Nuevo pipeline de render**: Crear handler en `handlers/`
2. **Nueva operación de video**: Agregar función en `core/video_ops.py`
3. **Nuevo formato de spec**: Extender `core/spec_parser.py`

## 🧪 Testing

```bash
# Test v0
curl -X POST http://localhost:9000/render \
  -H "Content-Type: application/json" \
  -d '{"job_id":"test_001","params":{"text":"Hello"},"output":{"video_object_key":"test.mp4","thumb_object_key":"test.jpg"}}'

# Test v1
curl -X POST http://localhost:9000/render/v1 \
  -H "Content-Type: application/json" \
  -d @test_spec_v1.json
```

## 🐳 Docker

```bash
# Build
docker build -t gala-renderer .

# Run
docker run -p 9000:9000 -v /tmp/data:/data gala-renderer
```
