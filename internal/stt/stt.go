package stt

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"os/exec"
	"time"
)

type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

func New(baseURL, model string) *Client {
	return &Client{baseURL: baseURL, model: model, http: &http.Client{Timeout: 120 * time.Second}}
}

// Listen grava `seconds` de áudio do mic (arecord) e devolve o caminho do wav.
func Listen(seconds int) (string, error) {
	tmp, err := os.CreateTemp("", "backseat-mic-*.wav")
	if err != nil {
		return "", err
	}
	tmp.Close()
	// 16kHz mono 16-bit: formato ideal pro Whisper
	cmd := exec.Command("arecord", "-q",
		"-f", "S16_LE", "-r", "16000", "-c", "1",
		"-d", fmt.Sprint(seconds), tmp.Name())
	if err := cmd.Run(); err != nil {
		return "", fmt.Errorf("arecord: %w (mic ok? teste: arecord -l)", err)
	}
	return tmp.Name(), nil
}

// Transcribe manda o wav pro Voicebox /transcribe e devolve o texto.
func (c *Client) Transcribe(wavPath string) (string, error) {
	f, err := os.Open(wavPath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	var buf bytes.Buffer
	w := multipart.NewWriter(&buf)
	part, _ := w.CreateFormFile("file", "audio.wav")
	if _, err := io.Copy(part, f); err != nil {
		return "", err
	}
	_ = w.WriteField("model", c.model)
	_ = w.WriteField("language", "pt")
	w.Close()

	req, _ := http.NewRequest("POST", c.baseURL+"/transcribe", &buf)
	req.Header.Set("Content-Type", w.FormDataContentType())

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("voicebox transcribe: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("transcribe status %d: %s", resp.StatusCode, string(b))
	}

	var out struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	return out.Text, nil
}
