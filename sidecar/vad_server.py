#!/usr/bin/env python3
"""Sidecar: VAD (Silero) + verificação de locutor (Resemblyzer).
Responde se o áudio é (a) fala humana E (b) a voz do dono."""
import http.server, json, wave, io, os
import numpy as np, torch
from silero_vad import load_silero_vad, get_speech_timestamps
from resemblyzer import VoiceEncoder, preprocess_wav

vad_model = load_silero_vad(onnx=True)
encoder = VoiceEncoder()

# embedding de referência do dono (se existir)
REF_PATH = "voz_carlos_embedding.npy"
ref_embedding = np.load(REF_PATH) if os.path.exists(REF_PATH) else None
THRESHOLD = 0.75  # similaridade mínima pra considerar "é o dono"

def has_speech(audio, sr):
    if sr != 16000:
        return True
    wav = torch.from_numpy(audio)
    ts = get_speech_timestamps(wav, vad_model, sampling_rate=16000)
    return len(ts) > 0

def is_owner(wav_path):
    if ref_embedding is None:
        return True, 1.0  # sem referência, aceita todos
    try:
        emb = encoder.embed_utterance(preprocess_wav(wav_path))
        sim = float(np.dot(ref_embedding, emb))
        return sim >= THRESHOLD, sim
    except Exception:
        return True, 0.0  # erro = deixa passar

class H(http.server.BaseHTTPRequestHandler):
    def do_POST(self):
        n = int(self.headers.get("Content-Length", 0))
        data = self.rfile.read(n)
        speech, owner, sim = True, True, 1.0
        try:
            # salva temp pra o resemblyzer ler
            tmp = "/tmp/vad_seg.wav"
            with open(tmp, "wb") as f:
                f.write(data)
            w = wave.open(io.BytesIO(data), "rb")
            sr = w.getframerate()
            audio = np.frombuffer(w.readframes(w.getnframes()), dtype=np.int16).astype(np.float32) / 32768.0
            speech = has_speech(audio, sr)
            if speech:
                owner, sim = is_owner(tmp)
        except Exception:
            pass
        self.send_response(200)
        self.send_header("Content-Type", "application/json")
        self.end_headers()
        # passa só se for fala E do dono
        self.wfile.write(json.dumps({
            "speech": bool(speech),
            "owner": bool(owner),
            "similarity": round(float(sim), 3),
            "pass": bool(speech and owner)
        }).encode())
    def log_message(self, *a): pass

if __name__ == "__main__":
    print("Sidecar VAD+Speaker em http://127.0.0.1:17494")
    if ref_embedding is not None:
        print(f"  verificação de locutor ATIVA (threshold {THRESHOLD})")
    http.server.HTTPServer(("127.0.0.1", 17494), H).serve_forever()

