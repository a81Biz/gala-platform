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

# WhisperX Configuration
WHISPER_MODEL = os.environ.get("WHISPER_MODEL", "base")
# Modelos disponibles: tiny, base, small, medium, large-v2, large-v3
# tiny: más rápido, menos preciso
# base: buen balance (recomendado para CPU)
# small: mejor calidad, más lento
# medium/large: mejor calidad, requiere GPU

WHISPER_DEVICE = os.environ.get("WHISPER_DEVICE", "cpu")
# cpu: funciona en cualquier máquina
# cuda: requiere GPU NVIDIA con CUDA

WHISPER_COMPUTE_TYPE = os.environ.get("WHISPER_COMPUTE_TYPE", "int8")
# int8: más rápido, menos memoria (recomendado para CPU)
# float16: requiere GPU
# float32: máxima precisión, más lento

# Caption settings
CAPTION_MAX_WORDS = int(os.environ.get("CAPTION_MAX_WORDS", "8"))
CAPTION_MAX_DURATION = float(os.environ.get("CAPTION_MAX_DURATION", "4.0"))
