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

# Scripts de render
COPY scripts/ ./scripts/
RUN chmod +x ./scripts/*.sh

# Server HTTP
COPY server.py ./server.py

EXPOSE 9000

ENV RENDERER_PORT=9000
ENV STORAGE_LOCAL_ROOT=/data

CMD ["python3", "/renderer/server.py"]
