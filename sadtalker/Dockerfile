# =====================================================
# GALA SadTalker Service - Contenedor aislado
# =====================================================
# Versiones CONGELADAS que funcionan juntas:
# - Python 3.8 (versión original de SadTalker)
# - PyTorch 1.12.1 + CUDA 11.3
# - numpy 1.23.5
# =====================================================

FROM nvidia/cuda:11.3.1-cudnn8-runtime-ubuntu20.04

ENV DEBIAN_FRONTEND=noninteractive
ENV PYTHONUNBUFFERED=1

# Instalar dependencias del sistema
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3.8 \
    python3.8-dev \
    python3-pip \
    ffmpeg \
    git \
    wget \
    unzip \
    libgl1-mesa-glx \
    libglib2.0-0 \
    libsm6 \
    libxext6 \
    libxrender-dev \
    && rm -rf /var/lib/apt/lists/*

# Hacer python3.8 el default
RUN update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.8 1 \
    && update-alternatives --install /usr/bin/python python /usr/bin/python3.8 1

# Actualizar pip
RUN pip3 install --upgrade pip setuptools wheel

WORKDIR /app

# ============================================
# Instalar dependencias con versiones EXACTAS
# ============================================

# PyTorch 1.12.1 con CUDA 11.3 (versión original de SadTalker)
RUN pip3 install --no-cache-dir \
    torch==1.12.1+cu113 \
    torchvision==0.13.1+cu113 \
    torchaudio==0.12.1 \
    --extra-index-url https://download.pytorch.org/whl/cu113

# Numpy versión compatible
RUN pip3 install --no-cache-dir numpy==1.23.5

# Scipy compatible
RUN pip3 install --no-cache-dir scipy==1.9.3

# Clonar SadTalker
RUN git clone --depth 1 https://github.com/OpenTalker/SadTalker.git /app/SadTalker

# Fix rutas relativas: SadTalker a veces asume /app/src/...
RUN ln -s /app/SadTalker/src /app/src

RUN apt-get update && apt-get install -y \
    cmake \
    build-essential \
    libopenblas-dev \
    liblapack-dev \
    libx11-dev \
    libgtk-3-dev \
    libboost-all-dev \
    && rm -rf /var/lib/apt/lists/*

# Instalar dependencias de SadTalker
WORKDIR /app/SadTalker
RUN pip3 install --no-cache-dir \
    face_alignment==1.3.5 \
    imageio==2.19.3 \
    imageio-ffmpeg==0.4.7 \
    librosa==0.9.2 \
    numba==0.56.4 \
    resampy==0.3.1 \
    pydub==0.25.1 \
    kornia==0.6.8 \
    yacs==0.1.8 \
    pyyaml \
    joblib \
    safetensors==0.4.5 \
    dlib

# BasicSR y GFPGAN para face enhancement
RUN pip3 install --no-cache-dir \
    basicsr==1.4.2 \
    facexlib==0.2.5 \
    gfpgan==1.3.8

# FastAPI para el servidor HTTP
RUN pip3 install --no-cache-dir \
    fastapi==0.100.0 \
    uvicorn==0.23.0 \
    python-multipart==0.0.6

# Crear directorios para modelos
RUN mkdir -p /app/checkpoints /app/gfpgan/weights

# Copiar servidor API
WORKDIR /app
COPY server.py ./
COPY download_models.sh ./
RUN chmod +x download_models.sh

EXPOSE 10364

ENV SADTALKER_PORT=10364
ENV SADTALKER_CHECKPOINT_DIR=/app/checkpoints
ENV PYTHONPATH="/app/SadTalker:/app/SadTalker/src:${PYTHONPATH}"

CMD ["./download_models.sh"]
