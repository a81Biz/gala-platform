"""
Configuración centralizada del renderer
"""
import os

# Server
RENDERER_PORT = int(os.environ.get("RENDERER_PORT", "9000"))

# Storage
DATA_ROOT = os.environ.get("STORAGE_LOCAL_ROOT", "/data")
SPECS_DIR = os.path.join(DATA_ROOT, "specs")

# Video defaults
DEFAULT_DURATION = 3.0
VIDEO_WIDTH = 1280
VIDEO_HEIGHT = 720
VIDEO_FPS = 30
FONT_SIZE = 48
TEXT_Y_OFFSET = 120

# Timeouts (segundos)
FFMPEG_TIMEOUT = 300
FFPROBE_TIMEOUT = 30
