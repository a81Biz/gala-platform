"""
Utilidades para manejo de archivos y paths
"""
import os
import json


def ensure_dir(path: str) -> None:
    """Crea un directorio si no existe"""
    os.makedirs(path, exist_ok=True)


def safe_remove(path: str) -> None:
    """Elimina un archivo ignorando errores"""
    try:
        os.remove(path)
    except Exception:
        pass


def safe_replace(src: str, dst: str) -> None:
    """Reemplaza un archivo, creando directorio si es necesario"""
    ensure_dir(os.path.dirname(dst))
    os.replace(src, dst)


def save_json(path: str, data: dict) -> None:
    """Guarda un diccionario como JSON"""
    ensure_dir(os.path.dirname(path))
    with open(path, "w", encoding="utf-8") as f:
        json.dump(data, f, indent=2)


def load_json(path: str) -> dict:
    """Carga un archivo JSON"""
    with open(path, "r", encoding="utf-8") as f:
        return json.load(f)


def sanitize_filename(filename: str) -> str:
    """
    Sanitiza un nombre de archivo removiendo caracteres peligrosos
    
    Args:
        filename: Nombre de archivo original
        
    Returns:
        Nombre de archivo seguro
    """
    filename = filename.replace("/", "_")
    filename = filename.replace("\\", "_")
    filename = filename.replace("..", "_")
    return filename
