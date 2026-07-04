package brain

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

type Client struct {
	apiKey  string
	model   string
	http    *http.Client
	history []message // memória curta da sessão
	persona string
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

func New(apiKey, model, persona string) *Client {
	return &Client{
		apiKey:  apiKey,
		model:   model,
		persona: persona,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Think recebe o que o streamer falou e devolve a resposta do co-host.
func (c *Client) Think(userText string) (string, error) {
	c.history = append(c.history, message{Role: "user", Content: userText})
	// janela curta: persona + últimas 10 mensagens (controla custo)
	msgs := []message{{Role: "system", Content: c.persona}}
	start := 0
	if len(c.history) > 10 {
		start = len(c.history) - 10
	}
	msgs = append(msgs, c.history[start:]...)

	body, _ := json.Marshal(map[string]any{
		"model":      c.model,
		"messages":   msgs,
		"max_tokens": 150, // resposta curta = fala curta = latência baixa
	})

	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("openai: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openai status %d: %s", resp.StatusCode, string(b))
	}

	var out struct {
		Choices []struct {
			Message message `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("openai: resposta vazia")
	}
	reply := out.Choices[0].Message.Content
	c.history = append(c.history, message{Role: "assistant", Content: reply})
	return reply, nil
}
