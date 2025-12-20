# =====================================================
# GALA Renderer - WhisperX + llamadas HTTP a SadTalker
# =====================================================
# Este contenedor solo tiene WhisperX para transcripción
# SadTalker corre en contenedor separado
# =====================================================

FROM nvidia/cuda:12.1.0-cudnn8-runtime-ubuntu22.04

ENV DEBIAN_FRONTEND=noninteractive
ENV PYTHONUNBUFFERED=1

# Instalar dependencias del sistema
RUN apt-get update && apt-get install -y --no-install-recommends \
    python3.11 \
    python3.11-dev \
    python3-pip \
    ffmpeg \
    git \
    fonts-noto \
    ca-certificates \
    && rm -rf /var/lib/apt/lists/*

# Hacer python3.11 el default
RUN update-alternatives --install /usr/bin/python3 python3 /usr/bin/python3.11 1 \
    && update-alternatives --install /usr/bin/python python /usr/bin/python3.11 1

RUN pip3 install --upgrade pip setuptools wheel

WORKDIR /renderer

# ============================================
# Instalar PyTorch 2.1 con CUDA 12.1
# ============================================
RUN pip3 install --no-cache-dir \
    torch==2.1.2 \
    torchvision==0.16.2 \
    torchaudio==2.1.2 \
    --index-url https://download.pytorch.org/whl/cu121

# ============================================
# Instalar WhisperX
# ============================================
RUN pip3 install --no-cache-dir \
    whisperx==3.1.1 \
    openai-whisper

# ============================================
# Fix: WhisperX/Torch no van bien con NumPy 2.x
# ============================================
RUN pip3 install --no-cache-dir "numpy==1.26.4" --force-reinstall

# ============================================
# Otras dependencias
# ============================================
RUN pip3 install --no-cache-dir \
    requests \
    opencv-python-headless \
    Pillow

# Copiar aplicación
COPY server.py ./
COPY config.py ./
COPY core/ ./core/
COPY handlers/ ./handlers/

# Verificar instalación
RUN python3 -c "import torch; print(f'PyTorch: {torch.__version__}'); print(f'CUDA: {torch.cuda.is_available()}')"

EXPOSE 9000

ENV RENDERER_PORT=9000
ENV STORAGE_LOCAL_ROOT=/data
ENV WHISPER_MODEL=base
ENV WHISPER_DEVICE=cuda
ENV WHISPER_COMPUTE_TYPE=float16
ENV SADTALKER_API_URL=http://sadtalker:10364

CMD ["python3", "/renderer/server.py"]
