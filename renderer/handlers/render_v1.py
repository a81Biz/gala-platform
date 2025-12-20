"""
Handler para renders v1 (moderno)
Pipeline: avatar + audio + captions
Animación via servicio SadTalker HTTP
"""
import os
import shutil

from core.spec_parser import V1Spec, ValidationError
from core.video_ops import (
    create_thumbnail_from_image,
    render_video_from_image,
    probe_audio_duration,
    mux_audio_to_video,
    burn_subtitles,
    FFmpegError
)
from core.captions import generate_vtt_file, generate_vtt_from_transcription
from core.file_utils import save_json, sanitize_filename, safe_remove, safe_replace, ensure_dir
from config import SPECS_DIR, DEFAULT_DURATION


def _copy_external_captions(src_path: str, dest_path: str) -> None:
    """Copia archivo VTT externo al destino"""
    ensure_dir(os.path.dirname(dest_path))
    shutil.copy2(src_path, dest_path)


def _try_animate_avatar(image_path: str, audio_path: str, output_path: str) -> bool:
    """
    Intenta animar el avatar llamando al servicio SadTalker.
    Returns True si tuvo éxito, False si no está disponible o falló.
    """
    try:
        from core.animator import animate_avatar, is_sadtalker_available
        
        if not is_sadtalker_available():
            print("[render_v1] SadTalker service not available, using static image")
            return False
        
        print("[render_v1] Animating avatar via SadTalker service...")
        animate_avatar(
            image_path=image_path,
            audio_path=audio_path,
            output_path=output_path,
            size=256,
            preprocess="crop",
            enhancer="gfpgan",
            still_mode=False,
            expression_scale=1.0,
        )
        
        if os.path.exists(output_path):
            print("[render_v1] Avatar animation successful")
            return True
        else:
            print("[render_v1] SadTalker did not produce output")
            return False
            
    except ImportError as e:
        print(f"[render_v1] Animator import error: {e}")
        return False
    except Exception as e:
        print(f"[render_v1] SadTalker animation failed: {e}")
        return False


def handle_render_v1(spec: dict) -> dict:
    """
    Procesa un render v1 (moderno)
    
    Pipeline:
    1. Validar spec
    2. Generar thumbnail desde avatar
    3. Determinar duración (audio o default)
    4. Si hay audio + SadTalker disponible: animar avatar via HTTP
       Si no: renderizar video estático
    5. Generar captions (transcripción o texto estático)
    6. Quemar captions en el video
    
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
        
        # 5. Renderizar video (animado o estático)
        used_animation = False
        
        if parsed.audio_path:
            # Intentar animar con SadTalker
            animated_video = parsed.video_dest + ".animated.mp4"
            
            used_animation = _try_animate_avatar(
                image_path=parsed.avatar_path,
                audio_path=parsed.audio_path,
                output_path=animated_video
            )
            
            if used_animation:
                # SadTalker ya incluye el audio
                final_video = animated_video
                temp_files.append(animated_video)
            else:
                # Fallback: video estático + audio
                temp_video = parsed.video_dest + ".silent.mp4"
                temp_files.append(temp_video)
                
                overlay_text = "" if (parsed.captions_enabled and parsed.captions_dest) else parsed.text
                
                render_video_from_image(
                    image_path=parsed.avatar_path,
                    output_path=temp_video,
                    text=overlay_text,
                    duration=duration
                )
                
                final_video = parsed.video_dest + ".with_audio.mp4"
                temp_files.append(final_video)
                mux_audio_to_video(temp_video, parsed.audio_path, final_video)
                safe_remove(temp_video)
                temp_files.remove(temp_video)
        else:
            # Sin audio: video estático
            temp_video = parsed.video_dest + ".static.mp4"
            temp_files.append(temp_video)
            
            overlay_text = "" if (parsed.captions_enabled and parsed.captions_dest) else parsed.text
            
            render_video_from_image(
                image_path=parsed.avatar_path,
                output_path=temp_video,
                text=overlay_text,
                duration=duration
            )
            final_video = temp_video
        
        # 6. Captions
        used_transcription = False
        used_external_captions = False
        
        if parsed.captions_enabled and parsed.captions_dest:
            if parsed.has_external_captions:
                print(f"[render_v1] Using external captions: {parsed.captions_input_path}")
                _copy_external_captions(parsed.captions_input_path, parsed.captions_dest)
                used_external_captions = True
            elif parsed.audio_path:
                used_transcription = generate_vtt_from_transcription(
                    output_path=parsed.captions_dest,
                    audio_path=parsed.audio_path,
                    language=None,
                    fallback_text=parsed.text,
                    fallback_duration=duration
                )
            else:
                generate_vtt_file(parsed.captions_dest, parsed.text, duration)
            
            # Quemar captions
            burnt_video = parsed.video_dest + ".burnt.mp4"
            temp_files.append(burnt_video)
            
            burn_subtitles(final_video, parsed.captions_dest, burnt_video)
            
            safe_remove(final_video)
            if final_video in temp_files:
                temp_files.remove(final_video)
            safe_replace(burnt_video, parsed.video_dest)
            temp_files.remove(burnt_video)
        else:
            safe_replace(final_video, parsed.video_dest)
            if final_video in temp_files:
                temp_files.remove(final_video)
        
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
                "used_transcription": used_transcription,
                "used_external_captions": used_external_captions,
                "used_animation": used_animation,
            }
        }
    
    except ValidationError as e:
        _cleanup_temp_files(temp_files)
        return {"status_code": 400, "body": {"error": str(e)}}
    
    except FFmpegError as e:
        _cleanup_temp_files(temp_files)
        return {"status_code": 500, "body": {"error": f"render failed: {str(e)}"}}
    
    except Exception as e:
        _cleanup_temp_files(temp_files)
        import traceback
        traceback.print_exc()
        return {"status_code": 500, "body": {"error": f"unexpected error: {str(e)}"}}


def _save_spec(job_id: str, spec: dict, version: str) -> str:
    safe_job = sanitize_filename(job_id)
    filename = f"{safe_job}.{version}.json"
    path = os.path.join(SPECS_DIR, filename)
    save_json(path, spec)
    return path


def _cleanup_temp_files(files: list) -> None:
    for f in files:
        safe_remove(f)
