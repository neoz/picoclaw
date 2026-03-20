#!/usr/bin/env python3
"""Proxy that translates Ollama /api/embed requests to OpenAI /v1/embeddings format."""

import json
import os
import sys
from http.server import HTTPServer, BaseHTTPRequestHandler
from urllib.request import Request, urlopen

UPSTREAM = os.environ.get("UPSTREAM_URL", "http://bge-m3:8081")
LISTEN_PORT = int(os.environ.get("LISTEN_PORT", "11434"))


class ProxyHandler(BaseHTTPRequestHandler):
    def do_GET(self):
        if self.path == "/api/tags":
            self._send_json(200, {"models": [{"name": "bge-m3"}]})
        else:
            self._send_json(404, {"error": "not found"})

    def do_POST(self):
        if self.path != "/api/embed":
            self._send_json(404, {"error": "not found"})
            return

        length = int(self.headers.get("Content-Length", 0))
        body = json.loads(self.rfile.read(length))

        req = Request(
            f"{UPSTREAM}/v1/embeddings",
            data=json.dumps({
                "model": body.get("model", "bge-m3"),
                "input": body.get("input", ""),
            }).encode(),
            headers={"Content-Type": "application/json"},
        )

        try:
            with urlopen(req, timeout=120) as resp:
                oai = json.loads(resp.read())
            embeddings = [item["embedding"] for item in oai.get("data", [])]
            self._send_json(200, {"embeddings": embeddings})
        except Exception as e:
            self._send_json(502, {"error": str(e)})

    def _send_json(self, code, obj):
        data = json.dumps(obj).encode()
        self.send_response(code)
        self.send_header("Content-Type", "application/json")
        self.send_header("Content-Length", str(len(data)))
        self.end_headers()
        self.wfile.write(data)

    def log_message(self, fmt, *args):
        print(f"[proxy] {fmt % args}", flush=True)


if __name__ == "__main__":
    server = HTTPServer(("0.0.0.0", LISTEN_PORT), ProxyHandler)
    print(f"Ollama-compat proxy listening on 0.0.0.0:{LISTEN_PORT} -> {UPSTREAM}", flush=True)
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        sys.exit(0)
