# Endless Image Processor

Lambda AWS en Go para redimensionar imágenes automáticamente cuando se suben a S3.

## Características

- **Formatos soportados**: JPEG, PNG, WebP, GIF
- **Tamaños generados**: thumbnail (150px), small (400px), medium (800px), large (1200px)
- **Trigger automático**: Se activa cuando se sube una imagen al bucket S3
- **Mismo directorio**: Las imágenes redimensionadas se guardan en el mismo directorio que la original

## Estructura del proyecto

```
├── main.go           # Función Lambda principal
├── go.mod           # Dependencias Go
├── template.yaml    # Template SAM para deployment
├── Makefile         # Comandos de build y deploy
└── README.md        # Este archivo
```

## Requisitos

- Go 1.21+
- AWS CLI configurado
- SAM CLI para deployment

## Build y Deploy

```bash
# Instalar dependencias
make init-go

# Build
make build

# Deploy (primera vez - modo guiado)
make deploy

# Deploy con bucket específico
make deploy BUCKET_NAME=mi-bucket-imagenes
```

## Funcionamiento

1. Se sube una imagen al bucket S3
2. S3 trigger activa la Lambda
3. Lambda valida que sea un formato soportado
4. Genera 4 versiones redimensionadas
5. Guarda todas en el mismo directorio con sufijos `_thumbnail`, `_small`, `_medium`, `_large`

## Ejemplo

Archivo original: `photos/vacation/beach.jpg`

Archivos generados:
- `photos/vacation/beach_thumbnail.jpg` (150x150)
- `photos/vacation/beach_small.jpg` (400x400) 
- `photos/vacation/beach_medium.jpg` (800x800)
- `photos/vacation/beach_large.jpg` (1200x1200)