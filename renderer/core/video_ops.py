"""
Operaciones de video con FFmpeg
"""
import os
import subprocess
from typing import Tuple

from config import (
    VIDEO_WIDTH, VIDEO_HEIGHT, VIDEO_FPS, FONT_SIZE, TEXT_Y_OFFSET,
    FFMPEG_TIMEOUT, FFPROBE_TIMEOUT
)
from core.file_utils import ensure_dir


class FFmpegError(Exception):
    """Error ejecutando FFmpeg"""
    pass


def probe_audio_duration(audio_path: str) -> float:
    """Obtiene la duración de un archivo de audio en segundos"""
    proc = subprocess.run(
        [
            "ffprobe", "-v", "error",
            "-show_entries", "format=duration",
            "-of", "default=noprint_wrappers=1:nokey=1",
            audio_path,
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        timeout=FFPROBE_TIMEOUT,
    )
    
    if proc.returncode != 0:
        raise FFmpegError(f"ffprobe failed: {proc.stderr[-2000:]}")
    
    try:
        duration = float(proc.stdout.strip())
        if duration <= 0:
            raise FFmpegError("invalid audio duration")
        return duration
    except ValueError:
        raise FFmpegError("could not parse audio duration")


def create_thumbnail_from_image(image_path: str, output_path: str) -> None:
    """Genera thumbnail desde una imagen usando FFmpeg"""
    ensure_dir(os.path.dirname(output_path))
    
    proc = subprocess.run(
        ["ffmpeg", "-y", "-i", image_path, "-frames:v", "1", output_path],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        timeout=FFMPEG_TIMEOUT,
    )
    
    if proc.returncode != 0:
        raise FFmpegError(f"thumbnail generation failed: {proc.stderr[-2000:]}")


def render_video_from_image(
    image_path: str,
    output_path: str,
    text: str,
    duration: float
) -> None:
    """
    Renderiza video desde imagen estática con texto superpuesto
    
    Args:
        image_path: Ruta de la imagen de entrada
        output_path: Ruta del video de salida
        text: Texto a superponer
        duration: Duración del video en segundos
    """
    ensure_dir(os.path.dirname(output_path))
    
    # Construir filtro de video
    vf = f"scale={VIDEO_WIDTH}:{VIDEO_HEIGHT}:force_original_aspect_ratio=decrease," \
         f"pad={VIDEO_WIDTH}:{VIDEO_HEIGHT}:(ow-iw)/2:(oh-ih)/2"
    
    # Agregar texto si existe
    if isinstance(text, str) and text.strip():
        # Escapar caracteres especiales para FFmpeg
        safe_text = text.replace(":", "\\:").replace("'", "\\'")
        vf += (
            f",drawtext=text='{safe_text}'"
            f":x=(w-text_w)/2"
            f":y=h-{TEXT_Y_OFFSET}"
            f":fontsize={FONT_SIZE}"
            f":fontcolor=white"
            f":box=1"
            f":boxcolor=black@0.5"
            f":boxborderw=16"
        )
    
    proc = subprocess.run(
        [
            "ffmpeg", "-y",
            "-loop", "1",
            "-i", image_path,
            "-t", f"{duration:.3f}",
            "-r", str(VIDEO_FPS),
            "-vf", vf,
            "-pix_fmt", "yuv420p",
            output_path,
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        timeout=FFMPEG_TIMEOUT,
    )
    
    if proc.returncode != 0:
        raise FFmpegError(f"video render failed: {proc.stderr[-2000:]}")


def mux_audio_to_video(video_path: str, audio_path: str, output_path: str) -> None:
    """
    Mezcla audio con video existente
    
    Args:
        video_path: Video de entrada (sin audio)
        audio_path: Audio a mezclar
        output_path: Video de salida con audio
    """
    ensure_dir(os.path.dirname(output_path))
    
    proc = subprocess.run(
        [
            "ffmpeg", "-y",
            "-i", video_path,
            "-i", audio_path,
            "-c:v", "copy",
            "-c:a", "aac",
            "-shortest",
            output_path,
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        timeout=FFMPEG_TIMEOUT,
    )
    
    if proc.returncode != 0:
        raise FFmpegError(f"audio muxing failed: {proc.stderr[-2000:]}")


def burn_subtitles(video_path: str, vtt_path: str, output_path: str) -> None:
    """
    Quema subtítulos VTT en el video
    
    Args:
        video_path: Video de entrada
        vtt_path: Archivo VTT con subtítulos
        output_path: Video de salida con subtítulos quemados
    """
    ensure_dir(os.path.dirname(output_path))
    
    proc = subprocess.run(
        [
            "ffmpeg", "-y",
            "-i", video_path,
            "-vf", f"subtitles={vtt_path}",
            "-c:a", "copy",
            output_path,
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        timeout=FFMPEG_TIMEOUT,
    )
    
    if proc.returncode != 0:
        raise FFmpegError(f"subtitle burn-in failed: {proc.stderr[-2000:]}")


def render_legacy_video(output_path: str, text: str, duration: float = 7.0) -> None:
    """
    Renderiza video legacy (v0) - video vertical negro con texto
    
    Args:
        output_path: Ruta del video de salida
        text: Texto a mostrar
        duration: Duración en segundos
    """
    ensure_dir(os.path.dirname(output_path))
    
    # Video vertical 1080x1920
    proc = subprocess.run(
        [
            "ffmpeg", "-y",
            "-f", "lavfi",
            "-i", f"color=c=black:s=1080x1920:d={duration}:r={VIDEO_FPS}",
            "-vf", f"drawtext=fontcolor=white:fontsize=72:text='{text}':x=(w-text_w)/2:y=(h-text_h)/2",
            "-c:v", "libx264",
            "-pix_fmt", "yuv420p",
            "-t", str(duration),
            output_path,
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        timeout=FFMPEG_TIMEOUT,
    )
    
    if proc.returncode != 0:
        raise FFmpegError(f"legacy video render failed: {proc.stderr[-2000:]}")


def extract_first_frame(video_path: str, output_path: str) -> None:
    """Extrae el primer frame de un video como thumbnail"""
    ensure_dir(os.path.dirname(output_path))
    
    proc = subprocess.run(
        [
            "ffmpeg", "-y",
            "-i", video_path,
            "-frames:v", "1",
            "-q:v", "2",
            output_path,
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
        timeout=FFMPEG_TIMEOUT,
    )
    
    if proc.returncode != 0:
        raise FFmpegError(f"frame extraction failed: {proc.stderr[-2000:]}")
