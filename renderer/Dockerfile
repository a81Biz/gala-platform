FROM alpine:3.20

# FFmpeg + Python
RUN apk add --no-cache \
    ffmpeg \
    python3 \
    py3-pip \
    bash \
    font-noto \
    ca-certificates

WORKDIR /renderer

# Copiar toda la aplicación
COPY server.py ./
COPY config.py ./
COPY core/ ./core/
COPY handlers/ ./handlers/

EXPOSE 9000

ENV RENDERER_PORT=9000
ENV STORAGE_LOCAL_ROOT=/data

CMD ["python3", "/renderer/server.py"]
