"""
Cliente HTTP para el servicio SadTalker
Llama al contenedor sadtalker via API REST
"""
import os
import requests
from typing import Optional

# URL del servicio SadTalker (contenedor separado)
SADTALKER_API_URL = os.environ.get("SADTALKER_API_URL", "http://sadtalker:10364")
SADTALKER_TIMEOUT = int(os.environ.get("SADTALKER_TIMEOUT", "600"))  # 10 minutos


class SadTalkerError(Exception):
    """Error en la animación con SadTalker"""
    pass


def is_sadtalker_available() -> bool:
    """Verifica si el servicio SadTalker está disponible"""
    try:
        response = requests.get(
            f"{SADTALKER_API_URL}/health",
            timeout=5
        )
        return response.status_code == 200
    except Exception as e:
        print(f"[animator] SadTalker health check failed: {e}")
        return False


def animate_avatar(
    image_path: str,
    audio_path: str,
    output_path: str,
    size: int = 256,
    preprocess: str = "crop",
    enhancer: str = "gfpgan",
    still_mode: bool = False,
    expression_scale: float = 1.0,
) -> str:
    """
    Genera video animado llamando al servicio SadTalker
    
    Args:
        image_path: Ruta local a la imagen del avatar
        audio_path: Ruta local al audio
        output_path: Ruta donde guardar el video generado
        size: 256 o 512
        preprocess: crop, resize, full
        enhancer: gfpgan o vacío
        still_mode: Reducir movimiento de cabeza
        expression_scale: Intensidad de expresiones
        
    Returns:
        Ruta al video generado
        
    Raises:
        SadTalkerError: Si la animación falla
    """
    if not os.path.exists(image_path):
        raise FileNotFoundError(f"Image not found: {image_path}")
    if not os.path.exists(audio_path):
        raise FileNotFoundError(f"Audio not found: {audio_path}")
    
    print(f"[animator] Calling SadTalker API...")
    print(f"[animator] Image: {image_path}")
    print(f"[animator] Audio: {audio_path}")
    print(f"[animator] Output: {output_path}")
    
    # Usar endpoint /animate/path que trabaja con rutas locales
    # (ambos contenedores comparten el volumen /data)
    try:
        response = requests.post(
            f"{SADTALKER_API_URL}/animate/path",
            data={
                "image_path": image_path,
                "audio_path": audio_path,
                "output_path": output_path,
                "size": size,
                "preprocess": preprocess,
                "enhancer": enhancer or "",
                "still_mode": still_mode,
                "expression_scale": expression_scale,
            },
            timeout=SADTALKER_TIMEOUT
        )
        
        if response.status_code == 200:
            result = response.json()
            if result.get("ok"):
                print(f"[animator] Animation complete: {output_path}")
                return output_path
            else:
                raise SadTalkerError(f"SadTalker returned error: {result}")
        else:
            error_detail = response.text[:500]
            raise SadTalkerError(f"SadTalker API error {response.status_code}: {error_detail}")
    
    except requests.exceptions.Timeout:
        raise SadTalkerError(f"SadTalker timeout after {SADTALKER_TIMEOUT}s")
    
    except requests.exceptions.ConnectionError as e:
        raise SadTalkerError(f"Cannot connect to SadTalker service: {e}")
    
    except Exception as e:
        if isinstance(e, SadTalkerError):
            raise
        raise SadTalkerError(f"SadTalker request failed: {e}")


def animate_avatar_upload(
    image_path: str,
    audio_path: str,
    output_path: str,
    size: int = 256,
    preprocess: str = "crop",
    enhancer: str = "gfpgan",
    still_mode: bool = False,
    expression_scale: float = 1.0,
) -> str:
    """
    Alternativa: sube archivos via multipart form
    Útil si los contenedores NO comparten volumen
    """
    if not os.path.exists(image_path):
        raise FileNotFoundError(f"Image not found: {image_path}")
    if not os.path.exists(audio_path):
        raise FileNotFoundError(f"Audio not found: {audio_path}")
    
    print(f"[animator] Uploading to SadTalker API...")
    
    try:
        with open(image_path, "rb") as img_file, open(audio_path, "rb") as aud_file:
            response = requests.post(
                f"{SADTALKER_API_URL}/animate",
                files={
                    "image": (os.path.basename(image_path), img_file),
                    "audio": (os.path.basename(audio_path), aud_file),
                },
                data={
                    "size": size,
                    "preprocess": preprocess,
                    "enhancer": enhancer or "",
                    "still_mode": still_mode,
                    "expression_scale": expression_scale,
                },
                timeout=SADTALKER_TIMEOUT
            )
        
        if response.status_code == 200:
            # Guardar video recibido
            os.makedirs(os.path.dirname(output_path), exist_ok=True)
            with open(output_path, "wb") as f:
                f.write(response.content)
            print(f"[animator] Animation complete: {output_path}")
            return output_path
        else:
            error_detail = response.text[:500]
            raise SadTalkerError(f"SadTalker API error {response.status_code}: {error_detail}")
    
    except requests.exceptions.Timeout:
        raise SadTalkerError(f"SadTalker timeout after {SADTALKER_TIMEOUT}s")
    
    except requests.exceptions.ConnectionError as e:
        raise SadTalkerError(f"Cannot connect to SadTalker service: {e}")
    
    except Exception as e:
        if isinstance(e, SadTalkerError):
            raise
        raise SadTalkerError(f"SadTalker request failed: {e}")
