import json
import os
import subprocess
from http.server import BaseHTTPRequestHandler, HTTPServer

PORT = int(os.getenv("RENDERER_PORT", "9000"))

def _bad_request(handler, message, details=None):
    payload = {"error": {"code": "BAD_REQUEST", "message": message}}
    if details is not None:
        payload["error"]["details"] = details
    handler._json(400, payload)

def _server_error(handler, message, details=None):
    payload = {"error": {"code": "RENDER_FAILED", "message": message}}
    if details is not None:
        payload["error"]["details"] = details
    handler._json(500, payload)

def validate_spec(spec: dict):
    # job_id
    job_id = spec.get("job_id")
    if not isinstance(job_id, str) or not job_id.strip():
        return False, "job_id is required and must be a non-empty string", {"field": "job_id"}

    # params (v0: libre, pero debe ser objeto si existe)
    params = spec.get("params", {})
    if params is None:
        params = {}
    if not isinstance(params, dict):
        return False, "params must be an object", {"field": "params"}

    # output
    output = spec.get("output")
    if not isinstance(output, dict):
        return False, "output is required and must be an object", {"field": "output"}

    vok = output.get("video_object_key")
    tok = output.get("thumb_object_key")

    if not isinstance(vok, str) or not vok.strip():
        return False, "output.video_object_key is required and must be a non-empty string", {"field": "output.video_object_key"}
    if not isinstance(tok, str) or not tok.strip():
        return False, "output.thumb_object_key is required and must be a non-empty string", {"field": "output.thumb_object_key"}

    # Hello Render v0: si quieres ser estricto con text:
    # (esto hace que el error sea del renderer si llega un job mal formado)
    text = params.get("text")
    if text is None or (isinstance(text, str) and not text.strip()):
        return False, "params.text is required for hello render v0", {"field": "params.text"}

    return True, "ok", None


class RendererHandler(BaseHTTPRequestHandler):
    def _json(self, status: int, payload: dict):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(payload).encode("utf-8"))

    def do_POST(self):
        if self.path != "/render":
            self._json(404, {"error": {"code": "NOT_FOUND", "message": "not found"}})
            return

        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length)

        try:
            spec = json.loads(raw.decode("utf-8"))
        except Exception:
            _bad_request(self, "invalid json")
            return

        if not isinstance(spec, dict):
            _bad_request(self, "spec must be a json object")
            return

        ok, msg, details = validate_spec(spec)
        if not ok:
            _bad_request(self, msg, details)
            return

        os.makedirs("/tmp/gala", exist_ok=True)
        spec_path = f"/tmp/gala/{spec['job_id']}.json"

        try:
            with open(spec_path, "w", encoding="utf-8") as f:
                json.dump(spec, f)
        except Exception as e:
            _server_error(self, "failed to write spec file", {"reason": str(e)})
            return

        # Log mínimo
        print(f"[renderer] job_id={spec['job_id']} video={spec['output']['video_object_key']} thumb={spec['output']['thumb_object_key']}")

        try:
            subprocess.check_call(
                ["bash", "/renderer/scripts/render.sh", spec_path],
                env=os.environ,
            )
        except subprocess.CalledProcessError as e:
            _server_error(self, "render failed", {"reason": str(e), "job_id": spec["job_id"]})
            return

        self._json(200, {"status": "ok", "job_id": spec["job_id"]})


def main():
    server = HTTPServer(("0.0.0.0", PORT), RendererHandler)
    print(f"GALA Renderer listening on :{PORT}")
    server.serve_forever()


if __name__ == "__main__":
    main()
