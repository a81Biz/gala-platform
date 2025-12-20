"""
Transcripción de audio con WhisperX
Genera timestamps precisos a nivel de palabra para captions
"""
import os
from typing import List, Dict, Optional
from dataclasses import dataclass

# Lazy import para no fallar si whisperx no está instalado
_whisperx = None
_model = None
_align_model = None
_align_metadata = None


def _get_whisperx():
    """Lazy import de whisperx"""
    global _whisperx
    if _whisperx is None:
        import whisperx
        _whisperx = whisperx
    return _whisperx


def _get_model():
    """Carga el modelo de Whisper (singleton)"""
    global _model
    if _model is None:
        whisperx = _get_whisperx()
        device = os.environ.get("WHISPER_DEVICE", "cpu")
        model_name = os.environ.get("WHISPER_MODEL", "base")
        compute_type = os.environ.get("WHISPER_COMPUTE_TYPE", "int8")
        
        print(f"[transcriber] Loading WhisperX model: {model_name} on {device}")
        _model = whisperx.load_model(model_name, device=device, compute_type=compute_type)
    return _model


def _get_align_model(language_code: str = "en"):
    """Carga el modelo de alineación (singleton por idioma)"""
    global _align_model, _align_metadata
    
    # Por ahora solo cacheamos un idioma
    if _align_model is None:
        whisperx = _get_whisperx()
        device = os.environ.get("WHISPER_DEVICE", "cpu")
        
        print(f"[transcriber] Loading alignment model for: {language_code}")
        _align_model, _align_metadata = whisperx.load_align_model(
            language_code=language_code,
            device=device
        )
    return _align_model, _align_metadata


@dataclass
class WordSegment:
    """Segmento de palabra con timestamps"""
    word: str
    start: float
    end: float


@dataclass
class TranscriptionResult:
    """Resultado de transcripción completa"""
    text: str
    language: str
    words: List[WordSegment]
    segments: List[Dict]  # Segmentos originales de Whisper
    

def transcribe_audio(audio_path: str, language: Optional[str] = None) -> TranscriptionResult:
    """
    Transcribe un archivo de audio con timestamps precisos a nivel de palabra
    
    Args:
        audio_path: Ruta al archivo de audio
        language: Código de idioma (ej: "es", "en"). Si es None, se detecta automáticamente.
        
    Returns:
        TranscriptionResult con texto, idioma y palabras con timestamps
        
    Raises:
        FileNotFoundError: Si el archivo de audio no existe
        RuntimeError: Si la transcripción falla
    """
    if not os.path.exists(audio_path):
        raise FileNotFoundError(f"Audio file not found: {audio_path}")
    
    whisperx = _get_whisperx()
    model = _get_model()
    device = os.environ.get("WHISPER_DEVICE", "cpu")
    
    print(f"[transcriber] Transcribing: {audio_path}")
    
    # 1. Cargar audio
    audio = whisperx.load_audio(audio_path)
    
    # 2. Transcribir
    if language:
        result = model.transcribe(audio, language=language)
    else:
        result = model.transcribe(audio)
    
    detected_language = result.get("language", "en")
    print(f"[transcriber] Detected language: {detected_language}")
    
    # 3. Alinear para obtener timestamps a nivel de palabra
    align_model, align_metadata = _get_align_model(detected_language)
    
    result = whisperx.align(
        result["segments"],
        align_model,
        align_metadata,
        audio,
        device,
        return_char_alignments=False
    )
    
    # 4. Extraer palabras con timestamps
    words = []
    for segment in result.get("segments", []):
        for word_data in segment.get("words", []):
            # WhisperX puede no tener start/end para algunas palabras
            if "start" in word_data and "end" in word_data:
                words.append(WordSegment(
                    word=word_data.get("word", "").strip(),
                    start=word_data["start"],
                    end=word_data["end"]
                ))
    
    # 5. Construir texto completo
    full_text = " ".join(seg.get("text", "") for seg in result.get("segments", []))
    
    print(f"[transcriber] Transcription complete: {len(words)} words")
    
    return TranscriptionResult(
        text=full_text.strip(),
        language=detected_language,
        words=words,
        segments=result.get("segments", [])
    )


def group_words_into_captions(
    words: List[WordSegment],
    max_words_per_caption: int = 8,
    max_duration: float = 4.0,
    min_gap: float = 0.5
) -> List[Dict]:
    """
    Agrupa palabras en captions legibles
    
    Args:
        words: Lista de WordSegment con timestamps
        max_words_per_caption: Máximo de palabras por caption
        max_duration: Duración máxima de un caption en segundos
        min_gap: Gap mínimo entre palabras para forzar nuevo caption
        
    Returns:
        Lista de dicts con 'start', 'end', 'text' para cada caption
    """
    if not words:
        return []
    
    captions = []
    current_words = []
    current_start = None
    
    for word in words:
        # Iniciar nuevo caption si:
        # 1. Es la primera palabra
        # 2. Alcanzamos el máximo de palabras
        # 3. La duración excede el máximo
        # 4. Hay un gap grande entre palabras
        
        should_start_new = False
        
        if not current_words:
            should_start_new = True
        elif len(current_words) >= max_words_per_caption:
            should_start_new = True
        elif current_start and (word.end - current_start) > max_duration:
            should_start_new = True
        elif current_words and (word.start - current_words[-1].end) > min_gap:
            should_start_new = True
        
        if should_start_new and current_words:
            # Guardar caption actual
            captions.append({
                "start": current_start,
                "end": current_words[-1].end,
                "text": " ".join(w.word for w in current_words)
            })
            current_words = []
            current_start = None
        
        # Agregar palabra al caption actual
        if current_start is None:
            current_start = word.start
        current_words.append(word)
    
    # Guardar último caption
    if current_words:
        captions.append({
            "start": current_start,
            "end": current_words[-1].end,
            "text": " ".join(w.word for w in current_words)
        })
    
    return captions


def transcribe_to_captions(
    audio_path: str,
    language: Optional[str] = None,
    max_words_per_caption: int = 8,
    max_caption_duration: float = 4.0
) -> List[Dict]:
    """
    Función de conveniencia: transcribe audio y devuelve captions listos para VTT
    
    Args:
        audio_path: Ruta al archivo de audio
        language: Código de idioma (None para auto-detectar)
        max_words_per_caption: Máximo palabras por caption
        max_caption_duration: Duración máxima por caption
        
    Returns:
        Lista de dicts con 'start', 'end', 'text'
    """
    result = transcribe_audio(audio_path, language)
    
    return group_words_into_captions(
        result.words,
        max_words_per_caption=max_words_per_caption,
        max_duration=max_caption_duration
    )
