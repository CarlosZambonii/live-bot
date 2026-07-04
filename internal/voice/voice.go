package voice

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strings"
	"time"
)

type Client struct {
	baseURL  string
	engine   string
	language string
	profile  string
	http     *http.Client
}

func New(baseURL, engine, language, profile string) *Client {
	return &Client{
		baseURL:  baseURL,
		engine:   engine,
		language: language,
		profile:  profile,
		http:     &http.Client{Timeout: 120 * time.Second},
	}
}

type generation struct {
	ID     string `json:"id"`
	Status string `json:"status"`
	Error  any    `json:"error"`
}

// Speak: submit assíncrono -> poll até completed -> baixa wav -> toca.
func (c *Client) Speak(text string) error {
	// 1. submit (engine incluído!)
	body, _ := json.Marshal(map[string]any{
		"profile_id": c.profile,
		"text":       text,
		"engine":     c.engine,
		"language":   c.language,
	})
	resp, err := c.http.Post(c.baseURL+"/generate", "application/json", bytes.NewReader(body))
	if err != nil {
		return fmt.Errorf("voicebox submit: %w", err)
	}
	var gen generation
	if err := json.NewDecoder(resp.Body).Decode(&gen); err != nil {
		resp.Body.Close()
		return fmt.Errorf("voicebox decode: %w", err)
	}
	resp.Body.Close()
	if gen.ID == "" {
		return fmt.Errorf("voicebox: sem generation id (perfil %s existe?)", c.profile)
	}

	// 2. poll até completed
	deadline := time.Now().Add(90 * time.Second)
	for {
		if time.Now().After(deadline) {
			return fmt.Errorf("voicebox: timeout na geração %s", gen.ID)
		}
		st, err := c.status(gen.ID)
		if err != nil {
			return err
		}
		if st.Status == "completed" || st.Status == "complete" || st.Status == "done" {
			break
		}
		if st.Status == "failed" || st.Status == "error" {
			return fmt.Errorf("voicebox: geração falhou: %v", st.Error)
		}
		time.Sleep(150 * time.Millisecond)
	}

	// 3. baixa o wav
	audio, err := c.http.Get(c.baseURL + "/audio/" + gen.ID)
	if err != nil {
		return fmt.Errorf("voicebox audio: %w", err)
	}
	defer audio.Body.Close()
	if audio.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(audio.Body)
		return fmt.Errorf("voicebox audio %d: %s", audio.StatusCode, string(b))
	}

	tmp, err := os.CreateTemp("", "backseat-*.wav")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := io.Copy(tmp, audio.Body); err != nil {
		return err
	}
	tmp.Close()

	// 4. toca
	return exec.Command("aplay", "-q", tmp.Name()).Run()
}

func (c *Client) status(id string) (*generation, error) {
	resp, err := c.http.Get(c.baseURL + "/generate/" + id + "/status")
	if err != nil {
		return nil, fmt.Errorf("voicebox status: %w", err)
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	// o endpoint responde em SSE: linhas "data: {json}"
	payload := strings.TrimSpace(string(raw))
	if i := strings.Index(payload, "data:"); i >= 0 {
		payload = strings.TrimSpace(payload[i+len("data:"):])
		// se vierem múltiplos eventos, fica só com a primeira linha JSON
		if j := strings.Index(payload, "\n"); j > 0 {
			payload = strings.TrimSpace(payload[:j])
		}
	}
	var g generation
	if err := json.Unmarshal([]byte(payload), &g); err != nil {
		return nil, fmt.Errorf("voicebox status parse: %w (raw: %.120s)", err, payload)
	}
	return &g, nil
}
