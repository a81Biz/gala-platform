# Entregable 2 — Hello Render funcional

**Plataforma GALA (Generación Audiovisual Local con Avatares)**

---

## Contexto

Tras completar el **Entregable 1**, la Plataforma GALA cuenta con:

* Arquitectura modular definida.
* Servicios levantados en Docker.
* API Contract v0 documentado y visible en Swagger.
* Backend, worker y renderer comunicándose correctamente.

El siguiente paso lógico no es “agregar features”, sino **probar el corazón de la plataforma**:

> 👉 Que GALA sea capaz de **recibir una solicitud**, **procesarla como job**, **renderizar un video real** y **entregar un resultado persistente**.

Eso es exactamente lo que cubre este entregable.

---

## Objetivo del Entregable 2

Implementar y validar el **primer pipeline completo de renderizado**, denominado **Hello Render**, que sirva como referencia técnica para todos los renders futuros.

Este pipeline debe demostrar:

* Orquestación correcta de jobs.
* Ejecución real de FFmpeg.
* Persistencia de resultados.
* Observabilidad vía API.

---

## Alcance funcional

### Qué hace Hello Render

Hello Render genera un video con las siguientes características:

* Formato vertical **9:16 (1080x1920)**.
* Duración: **7 segundos**.
* Fondo sólido (negro).
* Texto centrado (parametrizable).
* Thumbnail generado automáticamente.

No utiliza avatares ni assets externos todavía.
Su objetivo es **validar infraestructura, no creatividad**.

---

## Flujo completo (end-to-end)

1. El usuario crea un job vía API (`POST /jobs`).
2. El backend:

   * valida el payload,
   * guarda el job en base de datos,
   * lo encola en Redis.
3. El worker:

   * consume el job,
   * marca el job como `RUNNING`,
   * construye un `job_spec`.
4. El worker invoca al renderer vía HTTP.
5. El renderer:

   * ejecuta FFmpeg,
   * genera el video y el thumbnail,
   * los guarda en storage local.
6. El worker:

   * marca el job como `DONE`.
7. El usuario consulta el estado y los outputs (`GET /jobs/{id}`).

---

## Componentes involucrados

### Backend API (Go)

* Endpoint: `POST /jobs`
* Endpoint: `GET /jobs/{id}`
* Responsabilidad:

  * Validar requests.
  * Persistir estado del job.
  * Encolar jobs.

### Worker (Go)

* Consume la cola `gala:jobs`.
* Traduce el job a un `RendererSpec`.
* Maneja estados (`QUEUED → RUNNING → DONE/FAILED`).

### Renderer (contenedor dedicado)

* Expone `POST /render`.
* Ejecuta scripts Bash + FFmpeg.
* Produce archivos reales (`.mp4`, `.jpg`).

### Storage (LocalFS)

* Volumen Docker compartido.
* Persistencia real de outputs:

  ```
  /data/renders/<job_id>/
    ├─ hello.mp4
    └─ hello.jpg
  ```

---

## Contrato del Hello Render

### Request (crear job)

```http
POST /jobs
Content-Type: application/json
```

```json
{
  "name": "hello-01",
  "params": {
    "text": "GALA HELLO"
  }
}
```

### Respuesta (job creado)

```json
{
  "job": {
    "id": "job_1734567890123",
    "name": "hello-01",
    "status": "QUEUED",
    "params": {
      "text": "GALA HELLO"
    },
    "created_at": "2025-12-15T00:00:00Z"
  }
}
```

---

### Consulta del job

```http
GET /jobs/{jobId}
```

### Respuesta (job terminado)

```json
{
  "job": {
    "id": "job_1734567890123",
    "status": "DONE",
    "params": {
      "text": "GALA HELLO"
    },
    "outputs": [
      {
        "variant": 1,
        "video_object_key": "renders/job_1734567890123/hello.mp4",
        "thumb_object_key": "renders/job_1734567890123/hello.jpg"
      }
    ]
  }
}
```

---

## Evidencia técnica

* Swagger UI muestra los endpoints activos.
* El job cambia de estado correctamente.
* El archivo `hello.mp4` es reproducible.
* El thumbnail es generado desde el video.
* Todo ocurre **sin intervención manual**.

---

## Qué valida este entregable

✔️ Comunicación API ↔ Worker ↔ Renderer
✔️ Uso real de FFmpeg
✔️ Manejo de jobs asincrónicos
✔️ Persistencia de outputs
✔️ Diseño preparado para templates reales

---

## Qué **no** incluye todavía

* Avatares.
* Templates dinámicos.
* Assets externos.
* Storage cloud.
* Frontend funcional.

Esto es intencional: Hello Render es un **test estructural**, no un feature final.

---

## Impacto en la arquitectura

A partir de este punto:

* Todo render futuro **debe** seguir este patrón.
* Los templates solo cambiarán el `RendererSpec`.
* El renderer puede crecer sin tocar el API.
* El storage puede cambiar sin romper el flujo.

Hello Render se convierte en el **baseline técnico de GALA**.

---

## Próximo entregable

### Entregable 3 — Assets y Templates reales

* Gestión de assets vía API.
* Templates como entidad persistente.
* Renderer parametrizable.
* Outputs referenciados como assets.

---