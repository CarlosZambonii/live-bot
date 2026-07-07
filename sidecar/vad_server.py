#!/usr/bin/env python3
"""Sidecar VAD: recebe WAV, responde se tem fala humana (Silero oficial)."""
import http.server, json, wave, io
import numpy as np, torch
from silero_vad import load_silero_vad, get_speech_timestamps

model = load_silero_vad(onnx=True)

def has_speech(data):
    w = wave.open(io.BytesIO(data), "rb")
    sr = w.getframerate()
    audio = np.frombuffer(w.readframes(w.getnframes()), dtype=np.int16).astype(np.float32) / 32768.0
    if sr != 16000:
        return True
    wav = torch.from_numpy(audio)
    ts = get_speech_timestamps(wav, model, sampling_rate=16000)
    return len(ts) > 0

class H(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        data = self.rfile.read(n)
        try:
            speech = has_speech(data)
        except Exception:
            speech = True  # erro = deixa passar, não bloqueia
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        self.wfile.write(json.dumps({"speech": bool(speech)}).encode())
    def log_message(self, *a): pass

if __name__ == "__main__":
    print("VAD sidecar em http://127.0.0.1:17494")
    http.server.HTTPServer(("127.0.0.1", 17494), H).serve_forever()
