#!/bin/bash
# =====================================================
# Download SadTalker models if not present
# =====================================================

CHECKPOINT_DIR="/app/checkpoints"
GFPGAN_DIR="/app/gfpgan/weights"

BASE_URL="https://github.com/OpenTalker/SadTalker/releases/download/v0.0.2-rc"
GFPGAN_URL="https://github.com/TencentARC/GFPGAN/releases/download/v1.3.0"

download_with_retry() {
    local url=$1
    local output=$2
    local max_retries=3
    local retry=0
    
    while [ $retry -lt $max_retries ]; do
        echo "[download] Downloading $(basename $output)... (attempt $((retry+1)))"
        if wget --timeout=120 --tries=2 -O "$output" "$url"; then
            echo "[download] ✓ $(basename $output)"
            return 0
        fi
        retry=$((retry+1))
        sleep 5
    done
    
    echo "[download] ✗ Failed: $(basename $output)"
    return 1
}

check_models_exist() {
    [ -f "$CHECKPOINT_DIR/SadTalker_V0.0.2_256.safetensors" ] && \
    [ -f "$CHECKPOINT_DIR/mapping_00109-model.pth.tar" ] && \
    [ -f "$CHECKPOINT_DIR/auido2exp_00300-model.pth" ] && \
    [ -d "$CHECKPOINT_DIR/BFM_Fitting" ]
}

download_models() {
    echo "=========================================="
    echo "  SadTalker Model Download"
    echo "=========================================="
    
    mkdir -p "$CHECKPOINT_DIR" "$GFPGAN_DIR"
    
    # Core models
    [ -f "$CHECKPOINT_DIR/SadTalker_V0.0.2_256.safetensors" ] || \
        download_with_retry "$BASE_URL/SadTalker_V0.0.2_256.safetensors" \
            "$CHECKPOINT_DIR/SadTalker_V0.0.2_256.safetensors"
    
    [ -f "$CHECKPOINT_DIR/mapping_00109-model.pth.tar" ] || \
        download_with_retry "$BASE_URL/mapping_00109-model.pth.tar" \
            "$CHECKPOINT_DIR/mapping_00109-model.pth.tar"
    
    [ -f "$CHECKPOINT_DIR/mapping_00229-model.pth.tar" ] || \
        download_with_retry "$BASE_URL/mapping_00229-model.pth.tar" \
            "$CHECKPOINT_DIR/mapping_00229-model.pth.tar"
    
    [ -f "$CHECKPOINT_DIR/auido2exp_00300-model.pth" ] || \
        download_with_retry "$BASE_URL/auido2exp_00300-model.pth" \
            "$CHECKPOINT_DIR/auido2exp_00300-model.pth"
    
    [ -f "$CHECKPOINT_DIR/auido2pose_00140-model.pth" ] || \
        download_with_retry "$BASE_URL/auido2pose_00140-model.pth" \
            "$CHECKPOINT_DIR/auido2pose_00140-model.pth"
    
    # BFM Fitting
    if [ ! -d "$CHECKPOINT_DIR/BFM_Fitting" ]; then
        download_with_retry "$BASE_URL/BFM_Fitting.zip" "/tmp/BFM_Fitting.zip"
        if [ -f "/tmp/BFM_Fitting.zip" ]; then
            unzip -q /tmp/BFM_Fitting.zip -d "$CHECKPOINT_DIR/"
            rm /tmp/BFM_Fitting.zip
        fi
    fi
    
    # Hub models
    if [ ! -d "$CHECKPOINT_DIR/hub" ]; then
        download_with_retry "$BASE_URL/hub.zip" "/tmp/hub.zip"
        if [ -f "/tmp/hub.zip" ]; then
            unzip -q /tmp/hub.zip -d "$CHECKPOINT_DIR/"
            rm /tmp/hub.zip
        fi
    fi
    
    # GFPGAN
    [ -f "$GFPGAN_DIR/GFPGANv1.4.pth" ] || \
        download_with_retry "$GFPGAN_URL/GFPGANv1.4.pth" \
            "$GFPGAN_DIR/GFPGANv1.4.pth"
    
    echo "=========================================="
    echo "  Download complete"
    echo "=========================================="
}

# Main
if check_models_exist; then
    echo "[startup] Models already present"
else
    echo "[startup] Downloading models..."
    download_models
fi

# Start server
echo "[startup] Starting SadTalker API..."
exec python3 /app/server.py
