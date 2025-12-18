"""
Handler para renders v0 (legacy)
Pipeline simple: video vertical negro + texto
"""
import os

from core.spec_parser import V0Spec, ValidationError
from core.video_ops import render_legacy_video, extract_first_frame, FFmpegError
from core.file_utils import save_json, sanitize_filename
from config import SPECS_DIR


def handle_render_v0(spec: dict) -> dict:
    """
    Procesa un render v0 (legacy)
    
    Args:
        spec: Diccionario con el spec de render
        
    Returns:
        Dict con status_code y body para la respuesta HTTP
    """
    try:
        # 1. Parsear y validar spec
        parsed = V0Spec(spec)
        
        # 2. Guardar spec para debugging
        spec_path = _save_spec(parsed.job_id, spec, "v0")
        
        # 3. Renderizar video
        render_legacy_video(
            output_path=parsed.video_dest,
            text=parsed.text,
            duration=7.0
        )
        
        # 4. Generar thumbnail
        extract_first_frame(
            video_path=parsed.video_dest,
            output_path=parsed.thumb_dest
        )
        
        return {
            "status_code": 200,
            "body": {
                "ok": True,
                "spec": os.path.basename(spec_path),
                "job_id": parsed.job_id,
            }
        }
    
    except ValidationError as e:
        return {
            "status_code": 400,
            "body": {"error": str(e)}
        }
    
    except FFmpegError as e:
        return {
            "status_code": 500,
            "body": {"error": f"render failed: {str(e)}"}
        }
    
    except Exception as e:
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
