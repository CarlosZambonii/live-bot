package brain

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"time"
)

type Client struct {
	apiKey  string
	model   string
	http    *http.Client
	history []message
	persona string
}

// content pode ser string (texto puro) ou []part (multimodal)
type message struct {
	Role    string `json:"role"`
	Content any    `json:"content"`
}

type part struct {
	Type     string    `json:"type"`
	Text     string    `json:"text,omitempty"`
	ImageURL *imageURL `json:"image_url,omitempty"`
}

type imageURL struct {
	URL    string `json:"url"`
	Detail string `json:"detail,omitempty"`
}

func New(apiKey, model, persona string) *Client {
	return &Client{
		apiKey:  apiKey,
		model:   model,
		persona: persona,
		http:    &http.Client{Timeout: 60 * time.Second},
	}
}

// Think: só texto.
func (c *Client) Think(userText string) (string, error) {
	return c.chat(message{Role: "user", Content: userText})
}

// ThinkWithVision: texto + screenshot. A imagem NÃO entra no histórico
// (só o texto), senão o contexto explode de tamanho/custo.
func (c *Client) ThinkWithVision(userText, imagePath string) (string, error) {
	img, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("ler screenshot: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(img)

	visionMsg := message{Role: "user", Content: []part{
		{Type: "text", Text: userText + "\n\n(Você está vendo a tela do streamer agora. Comente com base no que vê, sem descrever a imagem inteira.)"},
		{Type: "image_url", ImageURL: &imageURL{
			URL:    "data:image/png;base64," + b64,
			Detail: "low", // low = ~85 tokens por imagem; barato e suficiente pra contexto de jogo
		}},
	}}
	return c.chat(visionMsg)
}

func (c *Client) chat(userMsg message) (string, error) {
	// histórico: só a versão texto (extrai o texto se for multimodal)
	histEntry := userMsg
	if parts, ok := userMsg.Content.([]part); ok {
		for _, p := range parts {
			if p.Type == "text" {
				histEntry = message{Role: "user", Content: p.Text}
				break
			}
		}
	}
	c.history = append(c.history, histEntry)

	msgs := []message{{Role: "system", Content: c.persona}}
	start := 0
	if len(c.history) > 10 {
		start = len(c.history) - 10
	}
	// histórico (sem a última, que vai na versão completa) + mensagem atual
	msgs = append(msgs, c.history[start:len(c.history)-1]...)
	msgs = append(msgs, userMsg)

	body, _ := json.Marshal(map[string]any{
		"model":      c.model,
		"messages":   msgs,
		"max_tokens": 60,
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
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
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
