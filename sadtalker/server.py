"""
GALA SadTalker API Service
Servidor HTTP aislado para animación de avatares
"""
import os
import sys
import uuid
import shutil
import tempfile
from typing import Optional
from pathlib import Path

from fastapi import FastAPI, HTTPException, UploadFile, File, Form
from fastapi.responses import FileResponse
import uvicorn

# Agregar SadTalker al path
SADTALKER_DIR = "/app/SadTalker"
os.chdir(SADTALKER_DIR)
sys.path.insert(0, SADTALKER_DIR)
sys.path.insert(0, os.path.join(SADTALKER_DIR, "src"))

app = FastAPI(
    title="GALA SadTalker API",
    description="Servicio de animación de avatares con lip-sync",
    version="1.0.0"
)

# Configuración
CHECKPOINT_DIR = os.environ.get("SADTALKER_CHECKPOINT_DIR", "/app/checkpoints")
RESULT_DIR = "/tmp/sadtalker_results"
os.makedirs(RESULT_DIR, exist_ok=True)


def get_sadtalker_args(
    image_path: str,
    audio_path: str,
    result_dir: str,
    size: int = 256,
    preprocess: str = "crop",
    enhancer: str = "gfpgan",
    still_mode: bool = False,
    expression_scale: float = 1.0,
    pose_style: int = 0,
):
    """Construye los argumentos para SadTalker"""
    from argparse import Namespace
    
    return Namespace(
        driven_audio=audio_path,
        source_image=image_path,
        result_dir=result_dir,
        size=size,
        preprocess=preprocess,
        enhancer=enhancer if enhancer else None,
        background_enhancer=None,
        still=still_mode,
        expression_scale=expression_scale,
        pose_style=pose_style,
        checkpoint_dir=CHECKPOINT_DIR,
        config_dir=os.path.join(SADTALKER_DIR, "src", "config"),
        device="cuda",
        cpu=False,
        old_version=False,
        batch_size=2,
        input_yaw=None,
        input_pitch=None,
        input_roll=None,
        ref_eyeblink=None,
        ref_pose=None,
        face3dvis=False,
        verbose=False,
        net_recon="resnet50",
        use_last_fc=False,
        bfm_folder=os.path.join(CHECKPOINT_DIR, "BFM_Fitting"),
        bfm_model="BFM_model_front.mat",
        focal=1015.0,
        center=112.0,
        camera_d=10.0,
        z_near=5.0,
        z_far=15.0,
    )


@app.get("/health")
async def health_check():
    """Health check endpoint"""
    return {"status": "ok", "service": "sadtalker"}


@app.post("/animate")
async def animate_avatar(
    image: UploadFile = File(..., description="Imagen del avatar (JPG/PNG)"),
    audio: UploadFile = File(..., description="Audio para sincronizar (WAV/MP3)"),
    size: int = Form(256, description="Tamaño del modelo (256 o 512)"),
    preprocess: str = Form("crop", description="Preprocesamiento: crop, resize, full"),
    enhancer: str = Form("gfpgan", description="Mejorador facial: gfpgan o vacío"),
    still_mode: bool = Form(False, description="Reducir movimiento de cabeza"),
    expression_scale: float = Form(1.0, description="Escala de expresiones"),
):
    """
    Genera video animado con lip-sync
    
    - **image**: Imagen del avatar (rostro frontal)
    - **audio**: Audio a sincronizar
    - **size**: 256 (rápido) o 512 (más calidad)
    - **preprocess**: crop (solo rostro), resize, full (imagen completa)
    - **enhancer**: gfpgan para mejorar calidad facial
    - **still_mode**: True para menos movimiento de cabeza
    - **expression_scale**: Intensidad de expresiones (0.5-1.5)
    
    Returns: Video MP4 animado
    """
    job_id = str(uuid.uuid4())[:8]
    work_dir = os.path.join(RESULT_DIR, job_id)
    os.makedirs(work_dir, exist_ok=True)
    
    try:
        # Guardar archivos subidos
        image_path = os.path.join(work_dir, f"image{Path(image.filename).suffix}")
        audio_path = os.path.join(work_dir, f"audio{Path(audio.filename).suffix}")
        
        with open(image_path, "wb") as f:
            shutil.copyfileobj(image.file, f)
        
        with open(audio_path, "wb") as f:
            shutil.copyfileobj(audio.file, f)
        
        print(f"[sadtalker] Job {job_id}: Processing...")
        print(f"[sadtalker] Image: {image_path}")
        print(f"[sadtalker] Audio: {audio_path}")
        
        # Importar y ejecutar SadTalker
        from inference import main as sadtalker_main
        
        args = get_sadtalker_args(
            image_path=image_path,
            audio_path=audio_path,
            result_dir=work_dir,
            size=size,
            preprocess=preprocess,
            enhancer=enhancer if enhancer else None,
            still_mode=still_mode,
            expression_scale=expression_scale,
        )
        
        # Ejecutar animación
        result_path = sadtalker_main(args)
        
        if not result_path or not os.path.exists(result_path):
            # Buscar video generado
            import glob
            videos = glob.glob(os.path.join(work_dir, "**", "*.mp4"), recursive=True)
            if videos:
                result_path = videos[0]
            else:
                raise HTTPException(status_code=500, detail="SadTalker no generó video")
        
        print(f"[sadtalker] Job {job_id}: Complete -> {result_path}")
        
        # Devolver video
        return FileResponse(
            result_path,
            media_type="video/mp4",
            filename=f"animated_{job_id}.mp4"
        )
    
    except ImportError as e:
        print(f"[sadtalker] Import error: {e}")
        raise HTTPException(status_code=500, detail=f"SadTalker import error: {str(e)}")
    
    except Exception as e:
        print(f"[sadtalker] Error: {e}")
        import traceback
        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))
    
    finally:
        # Limpiar después de un delay (para que FileResponse pueda enviar)
        # En producción, usar un job de limpieza
        pass


@app.post("/animate/path")
async def animate_avatar_by_path(
    image_path: str = Form(..., description="Ruta local a la imagen"),
    audio_path: str = Form(..., description="Ruta local al audio"),
    output_path: str = Form(..., description="Ruta donde guardar el video"),
    size: int = Form(256),
    preprocess: str = Form("crop"),
    enhancer: str = Form("gfpgan"),
    still_mode: bool = Form(False),
    expression_scale: float = Form(1.0),
):
    """
    Genera video animado usando rutas locales (para comunicación entre contenedores)
    
    Útil cuando renderer y sadtalker comparten volumen /data
    """
    if not os.path.exists(image_path):
        raise HTTPException(status_code=400, detail=f"Image not found: {image_path}")
    if not os.path.exists(audio_path):
        raise HTTPException(status_code=400, detail=f"Audio not found: {audio_path}")
    
    job_id = str(uuid.uuid4())[:8]
    work_dir = os.path.join(RESULT_DIR, job_id)
    os.makedirs(work_dir, exist_ok=True)
    
    try:
        print(f"[sadtalker] Job {job_id}: Processing paths...")
        print(f"[sadtalker] Image: {image_path}")
        print(f"[sadtalker] Audio: {audio_path}")
        print(f"[sadtalker] Output: {output_path}")
        
        from inference import main as sadtalker_main
        
        args = get_sadtalker_args(
            image_path=image_path,
            audio_path=audio_path,
            result_dir=work_dir,
            size=size,
            preprocess=preprocess,
            enhancer=enhancer if enhancer else None,
            still_mode=still_mode,
            expression_scale=expression_scale,
        )
        
        result_path = sadtalker_main(args)
        
        if not result_path or not os.path.exists(result_path):
            import glob
            videos = glob.glob(os.path.join(work_dir, "**", "*.mp4"), recursive=True)
            if videos:
                result_path = videos[0]
            else:
                raise HTTPException(status_code=500, detail="SadTalker no generó video")
        
        # Copiar a output_path
        os.makedirs(os.path.dirname(output_path), exist_ok=True)
        shutil.copy2(result_path, output_path)
        
        print(f"[sadtalker] Job {job_id}: Complete -> {output_path}")
        
        return {
            "ok": True,
            "job_id": job_id,
            "output_path": output_path
        }
    
    except Exception as e:
        print(f"[sadtalker] Error: {e}")
        import traceback
        traceback.print_exc()
        raise HTTPException(status_code=500, detail=str(e))
    
    finally:
        # Limpiar work_dir
        try:
            shutil.rmtree(work_dir, ignore_errors=True)
        except:
            pass


if __name__ == "__main__":
    port = int(os.environ.get("SADTALKER_PORT", "10364"))
    print(f"🎭 SadTalker API listening on :{port}")
    uvicorn.run(app, host="0.0.0.0", port=port)
