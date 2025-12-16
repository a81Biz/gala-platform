# Entregable 3 — Assets y Templates Reales

**Plataforma GALA (Generación Audiovisual Local con Avatares)**

---

## Contexto

Con el **Entregable 2 (Hello Render)**, la Plataforma GALA ya demostró que:

* puede recibir jobs,
* procesarlos de forma asíncrona,
* ejecutar un pipeline real de render,
* y producir archivos audiovisuales persistentes.

El siguiente paso natural no es “hacer más renders”, sino **formalizar los insumos y las reglas del render**.

En otras palabras:

> Pasar de “un script que genera un video”
> a una **plataforma que entiende qué está renderizando y por qué**.

Eso es exactamente lo que introduce este entregable.

---

## Objetivo del Entregable 3

Introducir **Assets** y **Templates** como **entidades de primer nivel** dentro de GALA, permitiendo:

* reutilización de recursos,
* parametrización real del render,
* desacoplamiento total entre API y Renderer,
* y una base sólida para escalar a avatares y storage cloud.

---

## Alcance funcional

### 1. Assets (recursos audiovisuales)

Un **Asset** representa **cualquier archivo pesado** que participa en la generación de contenido.

Ejemplos:

* imágenes de fondo
* videos base
* audios
* overlays
* outputs renderizados

Los assets:

* se **suben una sola vez**,
* se **referencian por ID**,
* y **nunca se duplican innecesariamente**.

---

### 2. Templates (reglas de render)

Un **Template** define:

* qué tipo de render se ejecuta,
* qué parámetros acepta,
* cómo se traduce a un `RendererSpec`.

Ejemplo:

* `hello_render`
* `presentation_basic`
* `product_showcase`
* `walk_cycle`

Los templates:

* viven en la base de datos,
* son reutilizables,
* y no contienen lógica de render (solo reglas).

---

## Nuevas entidades del dominio

### Asset

```text
Asset
- id
- kind
- provider
- object_key
- mime
- size
- checksum
- created_at
```

### Template

```text
Template
- id
- type
- name
- duration_ms
- format (width, height, fps)
- params_schema
- defaults
- created_at
```

### Job (extensión)

```text
Job
- id
- template_id
- input_asset_ids
- params
- output_asset_ids
- status
```

---

## Flujo operativo actualizado

### Subida y uso de assets

1. Usuario sube un archivo (`POST /assets`).
2. El backend:

   * guarda metadata,
   * delega el archivo al storage.
3. El asset queda disponible por ID.

### Creación de templates

1. Usuario define un template (`POST /templates`).
2. El template declara:

   * formato,
   * duración,
   * parámetros aceptados.
3. El template queda listo para usarse en jobs.

### Generación de jobs

1. Usuario crea un job (`POST /jobs`):

   * referencia `template_id`,
   * referencia `asset_ids`,
   * envía `params`.
2. El worker:

   * valida el schema del template,
   * construye el `RendererSpec`.
3. El renderer ejecuta sin conocer “qué es un template”.

---

## Contrato de API involucrado

### Assets

* `POST /assets`
* `GET /assets/{id}`
* `GET /assets/{id}/content`

### Templates

* `POST /templates`
* `GET /templates`
* `GET /templates/{id}`

### Jobs (extendido)

* `POST /jobs`
* `GET /jobs/{id}`

El **OpenAPI v0** ya contempla estos endpoints; este entregable los **activa funcionalmente**.

---

## RendererSpec (formalizado)

A partir de este entregable, **el renderer solo entiende esto**:

```json
{
  "job_id": "job_xxx",
  "template_type": "hello_render",
  "inputs": {
    "background": "assets/bg_01.png",
    "music": "assets/music_01.mp3"
  },
  "params": {
    "text": "Hola GALA"
  },
  "output": {
    "video_object_key": "renders/job_xxx/main.mp4",
    "thumb_object_key": "renders/job_xxx/thumb.jpg",
    "format": {
      "width": 1080,
      "height": 1920,
      "fps": 30
    }
  }
}
```

El renderer **no conoce IDs**, solo rutas y parámetros.

---

## Qué valida este entregable

✔️ Assets reutilizables
✔️ Templates desacoplados
✔️ Jobs reproducibles
✔️ Renderer independiente
✔️ Base sólida para storage cloud

---

## Qué **no** incluye todavía

* Google Drive / S3.
* Avatares.
* Motion / poses.
* Frontend completo.

Este entregable sigue enfocado en **estructura**, no en volumen de features.

---

## Impacto arquitectónico

Después de este punto:

* Los renders dejan de ser “scripts”.
* GALA se convierte en una **plataforma de composición audiovisual**.
* Cambiar el renderer **no rompe el API**.
* Cambiar el storage **no rompe los jobs**.

Este es el **punto de no retorno** hacia una plataforma seria.

---

## Próximo entregable

### Entregable 4 — Storage desacoplado (Google Drive / Buckets)

* Provider de almacenamiento intercambiable.
* Upload resumible.
* Assets grandes sin pasar por el backend.
* Preparación para cloud híbrido.

---
