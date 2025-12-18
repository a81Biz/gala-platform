#!/usr/bin/env python3
"""
GALA Renderer - HTTP Server
Responsabilidad: Solo routing HTTP, delega la lógica a handlers
"""
import json
from http.server import BaseHTTPRequestHandler, HTTPServer

from config import RENDERER_PORT
from handlers.render_v0 import handle_render_v0
from handlers.render_v1 import handle_render_v1


def read_json(handler: BaseHTTPRequestHandler):
    """Lee y parsea JSON del request body"""
    length = int(handler.headers.get("Content-Length", "0"))
    raw = handler.rfile.read(length) if length > 0 else b"{}"
    try:
        return json.loads(raw.decode("utf-8"))
    except Exception:
        return None


def write_json(handler: BaseHTTPRequestHandler, code: int, obj: dict):
    """Escribe respuesta JSON"""
    body = json.dumps(obj).encode("utf-8")
    handler.send_response(code)
    handler.send_header("Content-Type", "application/json")
    handler.send_header("Content-Length", str(len(body)))
    handler.end_headers()
    handler.wfile.write(body)


class Handler(BaseHTTPRequestHandler):
    def do_POST(self):
        # Routing
        if self.path == "/render":
            self._handle_v0()
        elif self.path == "/render/v1":
            self._handle_v1()
        else:
            self.send_response(404)
            self.end_headers()

    def _handle_v0(self):
        spec = read_json(self)
        if spec is None:
            write_json(self, 400, {"error": "invalid json"})
            return

        result = handle_render_v0(spec)
        write_json(self, result["status_code"], result["body"])

    def _handle_v1(self):
        spec = read_json(self)
        if spec is None:
            write_json(self, 400, {"error": "invalid json"})
            return

        result = handle_render_v1(spec)
        write_json(self, result["status_code"], result["body"])

    def log_message(self, format, *args):
        # Silenciar logs HTTP
        return


def main():
    server = HTTPServer(("0.0.0.0", RENDERER_PORT), Handler)
    print(f"🎬 Renderer listening on :{RENDERER_PORT}")
    server.serve_forever()


if __name__ == "__main__":
    main()
