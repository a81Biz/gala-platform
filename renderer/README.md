# GALA Renderer

Servicio de renderizado desacoplado para la Plataforma GALA.

## Responsabilidad
- Recibir un `RendererSpec`
- Ejecutar un pipeline de render
- Generar archivos audiovisuales
- No conocer jobs, assets, DB ni API

## Endpoint
POST /render

## Output
Escribe archivos en el storage compartido (`/data`).

## Filosofía
Renderer = función pura:
input → procesamiento → output
