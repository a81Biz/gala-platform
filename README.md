# Plataforma GALA

**Generación Audiovisual Local con Avatares**

Plataforma **local-first**, modular y extensible para la **generación automatizada de contenido audiovisual** usando avatares digitales, renderizado por jobs y almacenamiento desacoplado.

GALA está diseñada para:

* correr **100% en local** (Docker),
* separar estrictamente **frontend, backend, renderer y storage**,
* escalar por **entregables claros**, no por código improvisado,
* y mantener un **contrato de API vivo** como fuente única de verdad.

---

## Estado del proyecto

> 🟢 **Activo — Entregable 1 completado**
> Plataforma base levantada, contrato definido y ejecutable.

---

# Entregable 1 — Fundación técnica y contrato de la plataforma

### Objetivo

Establecer una **base técnica sólida y verificable** antes de agregar complejidad funcional (avatares, templates, assets avanzados, storage cloud).

Este entregable **no busca “features finales”**, sino:

* garantizar arquitectura correcta,
* evitar deuda técnica temprana,
* y permitir crecimiento controlado.

---

## Qué se logró en este entregable

### 1. Arquitectura definida y documentada

* Separación clara de responsabilidades:

  * **Frontend** (React)
  * **Backend API** (Go)
  * **Worker** (Go)
  * **Renderer** (FFmpeg en contenedor)
  * **Storage** (local, intercambiable)
* Enfoque **modular y atomizado** desde el día 1.
* Todo corre bajo **Docker Compose**.

---

### 2. Contrato de API v0 (fuente única de verdad)

Se definió el **API Contract v0** usando **OpenAPI 3.1**, cubriendo:

* Health
* Assets
* Models
* Templates
* Jobs

Este contrato:

* no es documentación “decorativa”,
* **define cómo debe comportarse la plataforma**,
* sirve como base para backend, frontend y testing.

---

### 3. Swagger UI levantado en local

El contrato se expone mediante **Swagger UI**, accesible en:

```
http://localhost:8081
```

Esto permite:

* explorar endpoints,
* validar payloads,
* probar la API sin frontend,
* detectar inconsistencias temprano.

✔️ Evidencia: Swagger muestra correctamente todos los endpoints del API v0.

---

### 4. Plataforma ejecutable (no solo documentos)

La plataforma ya puede levantarse con:

```bash
docker compose up --build
```

Servicios activos:

* API: `http://localhost:8080`
* Swagger UI: `http://localhost:8081`
* Renderer: `http://localhost:9000`

Esto valida:

* networking entre contenedores,
* configuración de volúmenes,
* arranque reproducible del entorno.

---

## Qué **no** se intentó todavía (a propósito)

Este entregable **no incluye aún**:

* generación real de avatares,
* templates complejos,
* faceswap o motion transfer,
* storage cloud (Drive / S3),
* frontend funcional completo.

👉 **Esto es intencional**.
Primero se construyó la **infraestructura correcta**.

---

# Cómo levantar GALA hasta este punto

### Requisitos

* Docker
* Docker Compose
* Go (solo si se quiere trabajar fuera de contenedores)

### Pasos

```bash
cd infra
docker compose up --build
```

Luego:

* Swagger UI → `http://localhost:8081`
* Health check → `http://localhost:8080/health`

---

# Estructura del repositorio (resumen)

```text
gala-platform/
├─ docs/                # Documentación y contrato OpenAPI
├─ infra/               # Docker Compose, envs
├─ backend/             # API + Worker (Go)
├─ frontend/            # UI (React)
├─ renderer/            # Pipeline de render (FFmpeg)
└─ storage/             # Storage local (dev)
```

---

# Roadmap de entregables (alto nivel)

### ✔️ Entregable 1 — Fundación y contrato (actual)

* Arquitectura
* Swagger
* Plataforma levantada

### 🔜 Entregable 2 — Hello Render funcional

* Job real que genera MP4
* Pipeline FFmpeg probado
* Outputs persistentes

### 🔜 Entregable 3 — Assets y Templates

* Gestión real de assets
* Templates parametrizables
* Outputs referenciados por la API

### 🔜 Entregable 4 — Storage desacoplado

* Google Drive como provider
* Diseño intercambiable (S3 / GCS / MinIO)

### 🔜 Entregable 5 — Avatares y generación avanzada

* Avatares por modelo
* Presentación / movimiento
* Batch rendering

---

## Filosofía del proyecto

GALA **no crece por parches**, crece por **entregables cerrados**.
Cada fase deja el sistema **estable, documentado y ejecutable**.

---