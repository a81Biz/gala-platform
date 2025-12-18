"""
Handler para renders v1 (moderno)
Pipeline: avatar + audio + texto + captions opcionales
"""
import os

from core.spec_parser import V1Spec, ValidationError
from core.video_ops import (
    create_thumbnail_from_image,
    render_video_from_image,
    probe_audio_duration,
    mux_audio_to_video,
    burn_subtitles,
    FFmpegError
)
from core.captions import generate_vtt_file
from core.file_utils import save_json, sanitize_filename, safe_remove, safe_replace
from config import SPECS_DIR, DEFAULT_DURATION


def handle_render_v1(spec: dict) -> dict:
    """
    Procesa un render v1 (moderno)
    
    Pipeline:
    1. Validar spec
    2. Generar thumbnail desde avatar
    3. Determinar duración (audio o default)
    4. Renderizar video desde avatar + texto
    5. Mezclar audio (si existe)
    6. Generar y quemar captions (si están habilitados)
    
    Args:
        spec: Diccionario con el spec de render
        
    Returns:
        Dict con status_code y body para la respuesta HTTP
    """
    temp_files = []
    
    try:
        # 1. Parsear y validar spec
        parsed = V1Spec(spec)
        
        # 2. Guardar spec para debugging
        spec_path = _save_spec(parsed.job_id, spec, "v1")
        
        # 3. Generar thumbnail desde avatar
        create_thumbnail_from_image(parsed.avatar_path, parsed.thumb_dest)
        
        # 4. Determinar duración
        duration = DEFAULT_DURATION
        if parsed.audio_path:
            duration = probe_audio_duration(parsed.audio_path)
        
        # 5. Renderizar video desde avatar
        temp_video = parsed.video_dest + ".silent.mp4"
        temp_files.append(temp_video)
        
        render_video_from_image(
            image_path=parsed.avatar_path,
            output_path=temp_video,
            text=parsed.text,
            duration=duration
        )
        
        # 6. Mezclar audio (si existe)
        if parsed.audio_path:
            final_video = parsed.video_dest + ".with_audio.mp4"
            temp_files.append(final_video)
            
            mux_audio_to_video(temp_video, parsed.audio_path, final_video)
            safe_remove(temp_video)
            temp_files.remove(temp_video)
        else:
            final_video = temp_video
        
        # 7. Captions (si están habilitados)
        if parsed.captions_enabled and parsed.captions_dest:
            # Generar archivo VTT
            generate_vtt_file(parsed.captions_dest, parsed.text, duration)
            
            # Quemar captions en el video
            burnt_video = parsed.video_dest + ".burnt.mp4"
            temp_files.append(burnt_video)
            
            burn_subtitles(final_video, parsed.captions_dest, burnt_video)
            
            # Reemplazar video final
            safe_remove(final_video)
            safe_replace(burnt_video, parsed.video_dest)
            temp_files.remove(burnt_video)
        else:
            # Sin captions, solo mover el video final
            safe_replace(final_video, parsed.video_dest)
        
        # Limpiar archivos temporales restantes
        _cleanup_temp_files(temp_files)
        
        return {
            "status_code": 200,
            "body": {
                "ok": True,
                "spec": os.path.basename(spec_path),
                "job_id": parsed.job_id,
                "duration": duration,
                "has_audio": parsed.audio_path is not None,
                "has_captions": parsed.captions_enabled and parsed.captions_dest is not None,
            }
        }
    
    except ValidationError as e:
        _cleanup_temp_files(temp_files)
        return {
            "status_code": 400,
            "body": {"error": str(e)}
        }
    
    except FFmpegError as e:
        _cleanup_temp_files(temp_files)
        return {
            "status_code": 500,
            "body": {"error": f"render failed: {str(e)}"}
        }
    
    except Exception as e:
        _cleanup_temp_files(temp_files)
        return {
            "status_code": 500,
            "body": {"error": f"unexpected error: {str(e)}"}
        }


def _save_spec(job_id: str, spec: dict, version: str) -> str:
    """Guarda el spec para debugging"""
    safe_job = sanitize_filename(job_id)
    filename = f"{safe_job}.{version}.json"
    path = os.path.join(SPECS_DIR, filename)
    save_json(path, spec)
    return path


def _cleanup_temp_files(files: list) -> None:
    """Limpia archivos temporales"""
    for f in files:
        safe_remove(f)
