package brain

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"sync"
	"time"
)

type Client struct {
	apiKey  string
	model   string
	http    *http.Client
	history []message
	persona string

	mu        sync.Mutex
	extraCtx  string // contexto dinâmico (chat da live), injetado a cada chamada
}

// SetContext atualiza o contexto extra (ex: chat recente) usado nas próximas chamadas.
func (c *Client) SetContext(ctx string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extraCtx = ctx
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
		{Type: "text", Text: userText + "\n\n(A imagem é a tela atual do streamer, use como contexto APENAS se a pergunta for sobre o jogo/tela. Se a pergunta for sobre o chat ou outra coisa, responda a pergunta e ignore a imagem.)"},
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

	c.mu.Lock()
	system := c.persona
	if c.extraCtx != "" {
		system += "\n\nContexto (mensagens recentes do chat da live, use APENAS quando a pergunta do streamer for sobre o chat; caso contrário responda a pergunta normalmente e ignore este bloco; nunca invente mensagens):\n" + c.extraCtx
	}
	ctxLen := len(c.extraCtx)
	c.mu.Unlock()
	log.Printf("[debug] extraCtx no prompt: %d chars", ctxLen)
	msgs := []message{{Role: "system", Content: system}}
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


// ThinkStateless: chamada isolada, sem histórico e sem persona de conversa.
// Para classificadores e vereditos de sistema — não contamina a conversa.
func (c *Client) ThinkStateless(prompt string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      c.model,
		"messages":   []message{{Role: "user", Content: prompt}},
		"max_tokens": 100,
	})
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct{ Content string `json:"content"` } `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("resposta vazia")
	}
	return out.Choices[0].Message.Content, nil
}

// VisionStateless: veredito com imagem, sem histórico.
func (c *Client) VisionStateless(prompt, imagePath string) (string, error) {
	img, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(img)
	body, _ := json.Marshal(map[string]any{
		"model": c.model,
		"messages": []message{{Role: "user", Content: []part{
			{Type: "text", Text: prompt},
			{Type: "image_url", ImageURL: &imageURL{URL: "data:image/png;base64," + b64, Detail: "low"}},
		}}},
		"max_tokens": 100,
	})
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.http.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	var out struct {
		Choices []struct {
			Message struct{ Content string `json:"content"` } `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}
	if len(out.Choices) == 0 {
		return "", fmt.Errorf("resposta vazia")
	}
	return out.Choices[0].Message.Content, nil
}