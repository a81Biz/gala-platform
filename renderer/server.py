import json
import os
import subprocess
from http.server import BaseHTTPRequestHandler, HTTPServer

PORT = int(os.getenv("RENDERER_PORT", "9000"))

class RendererHandler(BaseHTTPRequestHandler):

    def _json(self, status: int, payload: dict):
        self.send_response(status)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps(payload).encode("utf-8"))

    def do_POST(self):
        if self.path != "/render":
            self._json(404, {"error": "not found"})
            return

        length = int(self.headers.get("Content-Length", "0"))
        raw = self.rfile.read(length)

        try:
            spec = json.loads(raw.decode("utf-8"))
        except Exception:
            self._json(400, {"error": "invalid json"})
            return

        # Validación mínima
        if "job_id" not in spec or "output" not in spec:
            self._json(400, {"error": "invalid renderer spec"})
            return

        os.makedirs("/tmp/gala", exist_ok=True)
        spec_path = f"/tmp/gala/{spec['job_id']}.json"

        with open(spec_path, "w", encoding="utf-8") as f:
            json.dump(spec, f)

        try:
            subprocess.check_call(
                ["bash", "/renderer/scripts/render.sh", spec_path],
                env=os.environ,
            )
        except subprocess.CalledProcessError as e:
            self._json(500, {"error": "render failed", "details": str(e)})
            return

        self._json(200, {"status": "ok"})


def main():
    server = HTTPServer(("0.0.0.0", PORT), RendererHandler)
    print(f"GALA Renderer listening on :{PORT}")
    server.serve_forever()


if __name__ == "__main__":
    main()
