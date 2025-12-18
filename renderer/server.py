#!/usr/bin/env python3
import json
import os
import subprocess
from http.server import BaseHTTPRequestHandler, HTTPServer


DATA_ROOT = os.environ.get("STORAGE_LOCAL_ROOT", "/data")
RENDER_SCRIPT = "/renderer/scripts/render.sh"


def _read_json(handler: BaseHTTPRequestHandler):
    length = int(handler.headers.get("Content-Length", "0"))
    raw = handler.rfile.read(length) if length > 0 else b"{}"
    try:
        return json.loads(raw.decode("utf-8"))
    except Exception:
        return None


def _write_json(handler: BaseHTTPRequestHandler, code: int, obj):
    body = json.dumps(obj).encode("utf-8")
    handler.send_response(code)
    handler.send_header("Content-Type", "application/json")
    handler.send_header("Content-Length", str(len(body)))
    handler.end_headers()
    handler.wfile.write(body)


def _ensure_dir(path: str):
    os.makedirs(path, exist_ok=True)


def _spec_path(job_id: str, suffix: str):
    base = os.path.join(DATA_ROOT, "specs")
    _ensure_dir(base)
    safe_job = job_id.replace("/", "_").replace("\\", "_")
    return os.path.join(base, f"{safe_job}.{suffix}.json")


def _safe_job_id(spec: dict):
    job_id = spec.get("job_id") or spec.get("JobID") or spec.get("jobId") or ""
    if not isinstance(job_id, str):
        return ""
    return job_id.strip()


def _to_v0_from_v1(spec_v1: dict) -> dict:
    job_id = _safe_job_id(spec_v1)
    params = spec_v1.get("params", {}) or {}
    output = spec_v1.get("output", {}) or {}
    return {
        "job_id": job_id,
        "params": params,
        "output": {
            "video_object_key": output.get("video_object_key", ""),
            "thumb_object_key": output.get("thumb_object_key", ""),
            "captions_object_key": output.get("captions_object_key", ""),
        },
    }


def _abs_under_data(path: str) -> bool:
    try:
        ap = os.path.abspath(path)
        dr = os.path.abspath(DATA_ROOT)
        return ap.startswith(dr + os.sep) or ap == dr
    except Exception:
        return False


def _is_truthy(v) -> bool:
    if v is True:
        return True
    if v is False or v is None:
        return False
    if isinstance(v, (int, float)):
        return v == 1
    if isinstance(v, str):
        s = v.strip().lower()
        return s in ("1", "true", "yes", "y", "on")
    return False


def _extract_v1_paths(spec_v1: dict):
    inputs = spec_v1.get("inputs", {}) or {}
    output = spec_v1.get("output", {}) or {}

    avatar_path = inputs.get("avatar_image_asset_id")
    if not avatar_path or not isinstance(avatar_path, str):
        return None, None, None, None, None, "inputs.avatar_image_asset_id is required"
    if not _abs_under_data(avatar_path):
        return None, None, None, None, None, f"avatar_image path must be under {DATA_ROOT}"
    if not os.path.exists(avatar_path):
        return None, None, None, None, None, f"avatar_image file not found: {avatar_path}"

    audio_path = inputs.get("voice_audio_asset_id")  # optional
    if audio_path is not None:
        if not isinstance(audio_path, str) or audio_path.strip() == "":
            audio_path = None
        else:
            audio_path = audio_path.strip()
            if not _abs_under_data(audio_path):
                return None, None, None, None, None, f"voice_audio path must be under {DATA_ROOT}"
            if not os.path.exists(audio_path):
                return None, None, None, None, None, f"voice_audio file not found: {audio_path}"

    thumb_key = output.get("thumb_object_key", "")
    if not thumb_key or not isinstance(thumb_key, str):
        return None, None, None, None, None, "output.thumb_object_key is required"
    thumb_dest = os.path.join(DATA_ROOT, thumb_key)

    video_key = output.get("video_object_key", "")
    if not video_key or not isinstance(video_key, str):
        return None, None, None, None, None, "output.video_object_key is required"
    video_dest = os.path.join(DATA_ROOT, video_key)

    captions_key = output.get("captions_object_key", "")
    captions_dest = None
    if captions_key and isinstance(captions_key, str) and captions_key.strip() != "":
        captions_dest = os.path.join(DATA_ROOT, captions_key.strip())

    if not _abs_under_data(thumb_dest) or not _abs_under_data(video_dest):
        return None, None, None, None, None, f"output paths must resolve under {DATA_ROOT}"
    if captions_dest and not _abs_under_data(captions_dest):
        return None, None, None, None, None, f"captions_object_key must resolve under {DATA_ROOT}"

    return avatar_path, audio_path, thumb_dest, video_dest, captions_dest, None


def _overwrite_thumbnail_with_avatar(avatar_path: str, thumb_dest: str):
    _ensure_dir(os.path.dirname(thumb_dest))
    proc = subprocess.run(
        ["ffmpeg", "-y", "-i", avatar_path, "-frames:v", "1", thumb_dest],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        return False, f"ffmpeg thumbnail failed: {proc.stderr[-2000:]}"
    return True, None


def _probe_audio_duration_seconds(audio_path: str):
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
    )
    if proc.returncode != 0:
        return None, f"ffprobe failed: {proc.stderr[-2000:]}"
    try:
        val = float(proc.stdout.strip())
        if val <= 0:
            return None, "invalid audio duration"
        return val, None
    except Exception:
        return None, "could not parse audio duration"


def _render_video_from_avatar(avatar_path: str, video_dest: str, text: str, duration: float):
    _ensure_dir(os.path.dirname(video_dest))

    vf = "scale=1280:720:force_original_aspect_ratio=decrease,pad=1280:720:(ow-iw)/2:(oh-ih)/2"
    if isinstance(text, str) and text.strip() != "":
        safe = text.replace(":", "\\:").replace("'", "\\'")
        vf += f",drawtext=text='{safe}':x=(w-text_w)/2:y=h-120:fontsize=48:fontcolor=white:box=1:boxcolor=black@0.5:boxborderw=16"

    proc = subprocess.run(
        [
            "ffmpeg", "-y",
            "-loop", "1",
            "-i", avatar_path,
            "-t", f"{duration:.3f}",
            "-r", "30",
            "-vf", vf,
            "-pix_fmt", "yuv420p",
            video_dest,
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        return False, f"ffmpeg image->video failed: {proc.stderr[-2000:]}"
    return True, None


def _mux_audio(video_path: str, audio_path: str, out_path: str):
    _ensure_dir(os.path.dirname(out_path))
    proc = subprocess.run(
        [
            "ffmpeg", "-y",
            "-i", video_path,
            "-i", audio_path,
            "-c:v", "copy",
            "-c:a", "aac",
            "-shortest",
            out_path,
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        return False, f"ffmpeg mux failed: {proc.stderr[-2000:]}"
    return True, None


def _format_vtt_time(seconds: float) -> str:
    if seconds < 0:
        seconds = 0
    ms_total = int(round(seconds * 1000.0))
    hh = ms_total // 3600000
    ms_total -= hh * 3600000
    mm = ms_total // 60000
    ms_total -= mm * 60000
    ss = ms_total // 1000
    ms = ms_total - ss * 1000
    return f"{hh:02d}:{mm:02d}:{ss:02d}.{ms:03d}"


def _write_captions_vtt(dest_path: str, text: str, duration: float):
    _ensure_dir(os.path.dirname(dest_path))
    if not isinstance(text, str) or text.strip() == "":
        text = "(sin texto)"
    start = "00:00:00.000"
    end = _format_vtt_time(duration)
    content = "WEBVTT\n\n1\n{} --> {}\n{}\n".format(start, end, text.strip())
    with open(dest_path, "w", encoding="utf-8") as f:
        f.write(content)


def _burn_in_captions(video_in: str, vtt_path: str, video_out: str):
    """
    Burn subtitles into video. Requires ffmpeg with libass.
    """
    _ensure_dir(os.path.dirname(video_out))
    # IMPORTANT: subtitles filter expects a local file path.
    proc = subprocess.run(
        [
            "ffmpeg", "-y",
            "-i", video_in,
            "-vf", f"subtitles={vtt_path}",
            "-c:a", "copy",
            video_out,
        ],
        stdout=subprocess.PIPE,
        stderr=subprocess.PIPE,
        text=True,
        check=False,
    )
    if proc.returncode != 0:
        return False, f"ffmpeg burn-in failed: {proc.stderr[-2000:]}"
    return True, None


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        if self.path not in ("/render", "/render/v1"):
            self.send_response(404)
            self.end_headers()
            return

        spec = _read_json(self)
        if spec is None:
            _write_json(self, 400, {"error": "invalid json"})
            return

        job_id = _safe_job_id(spec)
        if job_id == "":
            _write_json(self, 400, {"error": "job_id is required"})
            return

        if self.path == "/render/v1":
            avatar_path, audio_path, thumb_dest, video_dest, captions_dest, err = _extract_v1_paths(spec)
            if err:
                _write_json(self, 400, {"error": err})
                return

            spec_to_run = _to_v0_from_v1(spec)
            path = _spec_path(job_id, "v1")
        else:
            spec_to_run = spec
            path = _spec_path(job_id, "v0")

        with open(path, "w", encoding="utf-8") as f:
            json.dump(spec_to_run, f)

        proc = subprocess.run(
            ["bash", RENDER_SCRIPT, path],
            stdout=subprocess.PIPE,
            stderr=subprocess.PIPE,
            text=True,
            check=False,
        )
        if proc.returncode != 0:
            _write_json(self, 500, {"error": "render failed", "stderr": proc.stderr[-2000:], "stdout": proc.stdout[-2000:]})
            return

        if self.path == "/render/v1":
            ok, err = _overwrite_thumbnail_with_avatar(avatar_path, thumb_dest)
            if not ok:
                _write_json(self, 500, {"error": err})
                return

            params = spec.get("params", {}) or {}
            text = params.get("text", "")

            duration = 3.0
            if audio_path:
                d, derr = _probe_audio_duration_seconds(audio_path)
                if derr:
                    _write_json(self, 500, {"error": derr})
                    return
                duration = d

            temp_video = video_dest + ".silent.mp4"
            ok, err = _render_video_from_avatar(avatar_path, temp_video, text, duration)
            if not ok:
                _write_json(self, 500, {"error": err})
                return

            if audio_path:
                ok, err = _mux_audio(temp_video, audio_path, video_dest)
                if not ok:
                    _write_json(self, 500, {"error": err})
                    return
                try:
                    os.remove(temp_video)
                except Exception:
                    pass
            else:
                os.replace(temp_video, video_dest)

            # ✅ captions generation + burn-in ONLY when params.captions=1 AND captions_object_key present
            if captions_dest and _is_truthy(params.get("captions")):
                _write_captions_vtt(captions_dest, text, duration)

                burnt = video_dest + ".burnt.mp4"
                ok, err = _burn_in_captions(video_dest, captions_dest, burnt)
                if not ok:
                    _write_json(self, 500, {"error": err})
                    return
                # replace final video with burnt version
                os.replace(burnt, video_dest)

        _write_json(self, 200, {"ok": True, "spec": os.path.basename(path)})

    def log_message(self, format, *args):
        return


def main():
    port = int(os.environ.get("RENDERER_PORT", "9000"))
    server = HTTPServer(("0.0.0.0", port), Handler)
    print(f"renderer listening on :{port}")
    server.serve_forever()


if __name__ == "__main__":
    main()
