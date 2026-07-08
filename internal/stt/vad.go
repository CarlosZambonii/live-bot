package stt

import (
	"bytes"
	"encoding/json"
	"net/http"
	"os"
	"time"
)

// VADClient consulta o sidecar Silero pra saber se um WAV tem fala humana.
type VADClient struct {
	url  string
	http *http.Client
}

func NewVAD(url string) *VADClient {
	return &VADClient{url: url, http: &http.Client{Timeout: 5 * time.Second}}
}

// HasSpeech devolve true se o WAV contém fala. Em erro, devolve true (não bloqueia).
func (v *VADClient) HasSpeech(wavPath string) bool {
	data, err := os.ReadFile(wavPath)
	if err != nil {
		return true
	}
	resp, err := v.http.Post(v.url, "application/octet-stream", bytes.NewReader(data))
	if err != nil {
		return true // sidecar fora do ar = deixa passar
	}
	defer resp.Body.Close()
	var out struct {
		Speech     bool    `json:"speech"`
		Owner      bool    `json:"owner"`
		Similarity float64 `json:"similarity"`
		Pass       bool    `json:"pass"`
	}
	if json.NewDecoder(resp.Body).Decode(&out) != nil {
		return true
	}
	if out.Speech && !out.Owner {
		// é fala, mas não é o dono (pessoa do lado, TV, eco)
		return false
	}
	return out.Pass
}
