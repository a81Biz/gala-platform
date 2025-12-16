# Documento Técnico

## Plataforma Local de Generación de Contenido Audiovisual con Avatares Digitales (GALA platform)

**Modo de operación:** Local (Docker)
**Stack:** React (Frontend) + Go (Backend)
**Almacenamiento:** Provider intercambiable (Google Drive inicial)

---

## 1. Propósito de la Aplicación

La aplicación permite **generar contenido audiovisual de forma masiva y automatizada** utilizando **avatares digitales autorizados**, sin necesidad de grabar videos manualmente en cada ocasión.

El sistema está diseñado para:

* Crear **videos cortos (TikTok / Reels / Shorts)**.
* Reutilizar **plantillas de presentación o movimiento**.
* Generar **múltiples variaciones** de un mismo contenido (batch).
* Operar **100% en local**, delegando solo el almacenamiento pesado a un proveedor externo.

La aplicación **no realiza faceswap ni suplantación**, sino que trabaja con **avatares oficiales por modelo**, creados y gestionados dentro del sistema.

---

## 2. Qué Hace la Aplicación (Alcance Funcional)

### 2.1 Funciones principales

1. **Gestión de modelos (avatares)**

   * Alta y administración de modelos digitales.
   * Asociación de assets visuales y de estilo.
   * Presets por modelo (encuadre, look, overlays).

2. **Gestión de plantillas**

   * Plantillas reutilizables de video:

     * Presentación / hablar a cámara.
     * Modelaje / poses.
     * Movimiento genérico (fase posterior).
   * Parámetros configurables (duración, zonas de texto, formato).

3. **Generación de contenido**

   * Creación de “jobs” de renderizado.
   * Producción por lote (batch).
   * Estados de procesamiento (cola, ejecución, terminado).

4. **Biblioteca de resultados**

   * Visualización, descarga y organización de videos generados.
   * Metadatos por campaña, modelo y plantilla.

5. **Almacenamiento desacoplado**

   * Assets y renders se almacenan en un provider externo.
   * La app guarda solo referencias, no archivos físicos.

---

## 3. Principios de Diseño (Reglas del Sistema)

Estas reglas **no se rompen**:

1. **El backend no renderiza video**

   * Solo orquesta, valida y coordina.
2. **El frontend no procesa archivos pesados**

   * Sube directo al storage cuando es posible.
3. **Todo archivo pesado vive fuera del backend**

   * El backend guarda metadata y referencias.
4. **Todo es intercambiable**

   * Storage, renderer y futuros motores deben poder cambiarse.
5. **La aplicación es local-first**

   * No depende de servicios cloud para funcionar.

---

## 4. Arquitectura General (Alto Nivel)

### Componentes

1. **Frontend (React)**
2. **Backend API (Go)**
3. **Renderer (contenedor especializado)**
4. **Storage Provider (Google Drive inicial)**
5. **Base de datos**
6. **Sistema de colas**

Cada componente tiene una responsabilidad clara y **no se solapan funciones**.

---

## 5. Responsabilidades por Componente

### 5.1 Frontend (React – minimalista)

**Responsabilidad:**
Interfaz de usuario y definición de flujos.

**No hace:**

* Render de video.
* Procesamiento pesado.
* Lógica de negocio compleja.

**Pantallas mínimas:**

* Dashboard
* Modelos
* Plantillas
* Generar contenido
* Biblioteca

---

### 5.2 Backend (Go)

**Responsabilidad:**
Orquestación completa del sistema.

Funciones:

* API REST
* Validación de inputs
* Gestión de entidades
* Control de jobs
* Comunicación con storage
* Lanzamiento de renders

El backend **nunca guarda archivos grandes localmente**.

---

### 5.3 Renderer (contenedor independiente)

**Responsabilidad:**
Transformar una **especificación de render** en archivos de salida.

Funciones:

* Composición de video (FFmpeg).
* Aplicación de overlays, subtítulos, audio.
* Generación de thumbnails.
* Devolver resultados al storage.

Es **stateless**: procesa y termina.

---

### 5.4 Storage Provider (intercambiable)

**Responsabilidad:**
Guardar y servir archivos pesados.

Características:

* Implementado como **driver intercambiable**.
* Google Drive es el primer provider.
* Debe soportar:

  * Upload resumible
  * Descarga (URL firmada o stream)
  * Borrado

---

## 6. Modelo de Datos Conceptual

### Entidades principales

#### Model

* id
* nombre artístico
* tags
* presets visuales
* assets asociados

#### Template

* id
* tipo (presentación / movimiento)
* duración
* layout
* parámetros configurables

#### Asset

* id
* tipo (input, audio, render, thumbnail)
* provider
* object_key
* mime
* tamaño

#### Job

* id
* modelos involucrados
* plantilla
* estado
* assets de entrada
* assets de salida
* timestamps

---

## 7. Flujo Operativo (End-to-End)

1. Usuario crea modelo.
2. Usuario sube assets (directo a storage).
3. Usuario crea plantilla.
4. Usuario crea job de generación.
5. Backend registra job y lo pone en cola.
6. Worker toma job y llama al renderer.
7. Renderer genera video y guarda en storage.
8. Backend actualiza estado.
9. Frontend muestra resultado.

---

## 8. Entorno de Ejecución (Local)

Todo corre en Docker:

* Frontend (React)
* Backend (Go)
* Renderer
* Base de datos (Postgres)
* Redis (cola)

Google Drive **no corre en Docker**, solo se conecta vía API.

---

## 9. Escalabilidad Planeada (sin implementarla aún)

* Cambiar Drive por S3 / GCS / MinIO.
* Agregar más renderers en paralelo.
* Agregar nuevos tipos de plantillas.
* Integrar motores avanzados de avatar.

Nada de esto rompe la arquitectura actual.

---

## 10. Estado del Documento

Este documento es:

* ✅ Fuente única de trabajo
* ✅ Base para PRD, backlog y código
* ✅ Referencia obligatoria para decisiones técnicas

**Nada se implementa que contradiga este documento.**

