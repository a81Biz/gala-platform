"""
Parsing y validación de specs de render
"""
import os
from typing import Dict, Optional, Tuple
from config import DATA_ROOT


class ValidationError(Exception):
    """Error de validación de spec"""
    pass


def extract_job_id(spec: dict) -> str:
    """Extrae job_id del spec (soporta múltiples formatos)"""
    job_id = spec.get("job_id") or spec.get("JobID") or spec.get("jobId") or ""
    if not isinstance(job_id, str):
        raise ValidationError("job_id must be a string")
    
    job_id = job_id.strip()
    if not job_id:
        raise ValidationError("job_id is required")
    
    return job_id


def is_truthy(value) -> bool:
    """Evalúa si un valor debe considerarse verdadero"""
    if value is True:
        return True
    if value is False or value is None:
        return False
    if isinstance(value, (int, float)):
        return value == 1
    if isinstance(value, str):
        s = value.strip().lower()
        return s in ("1", "true", "yes", "y", "on")
    return False


def validate_path_under_data(path: str, name: str) -> None:
    """Valida que un path esté bajo DATA_ROOT (seguridad)"""
    try:
        abs_path = os.path.abspath(path)
        data_root = os.path.abspath(DATA_ROOT)
        
        if not (abs_path.startswith(data_root + os.sep) or abs_path == data_root):
            raise ValidationError(f"{name} must be under {DATA_ROOT}")
    except Exception as e:
        raise ValidationError(f"invalid {name} path: {e}")


def validate_file_exists(path: str, name: str) -> None:
    """Valida que un archivo exista"""
    if not os.path.exists(path):
        raise ValidationError(f"{name} file not found: {path}")


class V1Spec:
    """Spec parseado de v1 con validación"""
    
    def __init__(self, spec: dict):
        self.raw = spec
        self.job_id = extract_job_id(spec)
        
        # Inputs
        inputs = spec.get("inputs", {}) or {}
        self.avatar_path = self._extract_avatar(inputs)
        self.audio_path = self._extract_audio(inputs)
        
        # Output
        output = spec.get("output", {}) or {}
        self.video_dest = self._extract_video_output(output)
        self.thumb_dest = self._extract_thumb_output(output)
        self.captions_dest = self._extract_captions_output(output)
        
        # Params
        params = spec.get("params", {}) or {}
        self.text = params.get("text", "")
        self.captions_enabled = is_truthy(params.get("captions"))
    
    def _extract_avatar(self, inputs: dict) -> str:
        """Extrae y valida avatar_image_asset_id"""
        avatar = inputs.get("avatar_image_asset_id")
        
        if not avatar or not isinstance(avatar, str):
            raise ValidationError("inputs.avatar_image_asset_id is required")
        
        avatar = avatar.strip()
        validate_path_under_data(avatar, "avatar_image")
        validate_file_exists(avatar, "avatar_image")
        
        return avatar
    
    def _extract_audio(self, inputs: dict) -> Optional[str]:
        """Extrae y valida voice_audio_asset_id (opcional)"""
        audio = inputs.get("voice_audio_asset_id")
        
        if audio is None or (isinstance(audio, str) and audio.strip() == ""):
            return None
        
        if not isinstance(audio, str):
            raise ValidationError("voice_audio_asset_id must be a string")
        
        audio = audio.strip()
        validate_path_under_data(audio, "voice_audio")
        validate_file_exists(audio, "voice_audio")
        
        return audio
    
    def _extract_video_output(self, output: dict) -> str:
        """Extrae y valida video_object_key"""
        key = output.get("video_object_key", "")
        
        if not key or not isinstance(key, str):
            raise ValidationError("output.video_object_key is required")
        
        dest = os.path.join(DATA_ROOT, key)
        validate_path_under_data(dest, "video_object_key")
        
        return dest
    
    def _extract_thumb_output(self, output: dict) -> str:
        """Extrae y valida thumb_object_key"""
        key = output.get("thumb_object_key", "")
        
        if not key or not isinstance(key, str):
            raise ValidationError("output.thumb_object_key is required")
        
        dest = os.path.join(DATA_ROOT, key)
        validate_path_under_data(dest, "thumb_object_key")
        
        return dest
    
    def _extract_captions_output(self, output: dict) -> Optional[str]:
        """Extrae y valida captions_object_key (opcional)"""
        key = output.get("captions_object_key", "")
        
        if not key or not isinstance(key, str) or key.strip() == "":
            return None
        
        dest = os.path.join(DATA_ROOT, key.strip())
        validate_path_under_data(dest, "captions_object_key")
        
        return dest


class V0Spec:
    """Spec parseado de v0 (legacy)"""
    
    def __init__(self, spec: dict):
        self.raw = spec
        self.job_id = extract_job_id(spec)
        
        # Params
        params = spec.get("params", {}) or {}
        self.text = params.get("text", "GALA")
        
        # Output
        output = spec.get("output", {}) or {}
        video_key = output.get("video_object_key", "")
        thumb_key = output.get("thumb_object_key", "")
        
        if not video_key:
            raise ValidationError("output.video_object_key is required")
        if not thumb_key:
            raise ValidationError("output.thumb_object_key is required")
        
        self.video_dest = os.path.join(DATA_ROOT, video_key)
        self.thumb_dest = os.path.join(DATA_ROOT, thumb_key)
        
        validate_path_under_data(self.video_dest, "video_object_key")
        validate_path_under_data(self.thumb_dest, "thumb_object_key")
