"""
Generación de archivos de captions VTT
"""
import os
from typing import List, Dict, Optional
from core.file_utils import ensure_dir


def format_vtt_timestamp(seconds: float) -> str:
    """
    Formatea segundos a timestamp VTT (HH:MM:SS.mmm)
    
    Args:
        seconds: Tiempo en segundos
        
    Returns:
        String formateado como "00:00:05.500"
    """
    if seconds < 0:
        seconds = 0
    
    ms_total = int(round(seconds * 1000.0))
    
    hours = ms_total // 3600000
    ms_total -= hours * 3600000
    
    minutes = ms_total // 60000
    ms_total -= minutes * 60000
    
    secs = ms_total // 1000
    ms = ms_total - secs * 1000
    
    return f"{hours:02d}:{minutes:02d}:{secs:02d}.{ms:03d}"


def generate_vtt_file(output_path: str, text: str, duration: float) -> None:
    """
    Genera un archivo VTT con un caption que cubre toda la duración
    (Método legacy - un solo caption para todo el video)
    
    Args:
        output_path: Ruta donde guardar el archivo VTT
        text: Texto del caption
        duration: Duración del video en segundos
    """
    ensure_dir(os.path.dirname(output_path))
    
    # Validar texto
    if not isinstance(text, str) or text.strip() == "":
        text = "(sin texto)"
    
    # Timestamps
    start = "00:00:00.000"
    end = format_vtt_timestamp(duration)
    
    # Construir contenido VTT
    content = f"WEBVTT\n\n1\n{start} --> {end}\n{text.strip()}\n"
    
    # Escribir archivo
    with open(output_path, "w", encoding="utf-8") as f:
        f.write(content)


def generate_multi_caption_vtt(output_path: str, captions: List[Dict], duration: float = None) -> None:
    """
    Genera un archivo VTT con múltiples captions
    
    Args:
        output_path: Ruta donde guardar el archivo VTT
        captions: Lista de dicts con 'start', 'end', 'text'
        duration: Duración total del video (opcional, para validación)
        
    Example:
        captions = [
            {"start": 0.0, "end": 2.5, "text": "First caption"},
            {"start": 2.5, "end": 5.0, "text": "Second caption"},
        ]
    """
    ensure_dir(os.path.dirname(output_path))
    
    lines = ["WEBVTT", ""]
    
    for idx, cap in enumerate(captions, start=1):
        start = format_vtt_timestamp(cap.get("start", 0))
        end_time = cap.get("end", duration if duration else 0)
        end = format_vtt_timestamp(end_time)
        text = cap.get("text", "").strip()
        
        if not text:
            continue
        
        lines.append(str(idx))
        lines.append(f"{start} --> {end}")
        lines.append(text)
        lines.append("")
    
    content = "\n".join(lines)
    
    with open(output_path, "w", encoding="utf-8") as f:
        f.write(content)


def generate_vtt_from_transcription(
    output_path: str,
    audio_path: str,
    language: Optional[str] = None,
    fallback_text: Optional[str] = None,
    fallback_duration: Optional[float] = None
) -> bool:
    """
    Genera VTT desde transcripción de audio con WhisperX
    
    Args:
        output_path: Ruta donde guardar el archivo VTT
        audio_path: Ruta del archivo de audio a transcribir
        language: Código de idioma (None para auto-detectar)
        fallback_text: Texto a usar si la transcripción falla
        fallback_duration: Duración para el fallback
        
    Returns:
        True si se usó transcripción, False si se usó fallback
    """
    try:
        # Importar transcriber (puede fallar si whisperx no está instalado)
        from core.transcriber import transcribe_to_captions
        
        print(f"[captions] Transcribing audio for captions: {audio_path}")
        
        captions = transcribe_to_captions(
            audio_path=audio_path,
            language=language,
            max_words_per_caption=8,
            max_caption_duration=4.0
        )
        
        if captions:
            generate_multi_caption_vtt(output_path, captions)
            print(f"[captions] Generated VTT with {len(captions)} captions from transcription")
            return True
        else:
            print("[captions] Transcription returned no captions, using fallback")
    
    except ImportError as e:
        print(f"[captions] WhisperX not available: {e}, using fallback")
    
    except Exception as e:
        print(f"[captions] Transcription failed: {e}, using fallback")
    
    # Fallback: usar texto proporcionado
    if fallback_text and fallback_duration:
        print("[captions] Using fallback text for captions")
        generate_vtt_file(output_path, fallback_text, fallback_duration)
        return False
    
    # Si no hay fallback, crear VTT vacío
    print("[captions] No fallback available, creating minimal VTT")
    ensure_dir(os.path.dirname(output_path))
    with open(output_path, "w", encoding="utf-8") as f:
        f.write("WEBVTT\n\n")
    return False
