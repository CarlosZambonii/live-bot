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
	"strings"
	"sync"
	"time"

	"github.com/CarlosZambonii/backseat/internal/tools"
)

type Client struct {
	apiKey  string
	model   string
	http    *http.Client
	history []message
	persona string

	mu        sync.Mutex
	extraCtx   string
	memoryCtx  string
	narrative  string

	search *tools.Searcher
}

type message struct {
	Role       string     `json:"role"`
	Content    any        `json:"content"`
	ToolCalls  []toolCall `json:"tool_calls,omitempty"`
	ToolCallID string     `json:"tool_call_id,omitempty"`
}

type toolCall struct {
	ID       string `json:"id"`
	Type     string `json:"type"`
	Function struct {
		Name      string `json:"name"`
		Arguments string `json:"arguments"`
	} `json:"function"`
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

func (c *Client) SetSearcher(s *tools.Searcher) {
	c.search = s
	log.Printf("[brain] searcher plugado: enabled=%v", s != nil && s.Enabled())
}

func (c *Client) SetPersona(p string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.persona = p
}

func (c *Client) SetMemory(m string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.memoryCtx = m
}

func (c *Client) SetNarrative(n string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.narrative = n
}

func (c *Client) SetContext(ctx string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.extraCtx = ctx
}

func (c *Client) systemPrompt() string {
	c.mu.Lock()
	defer c.mu.Unlock()
	s := c.persona
	if c.search != nil && c.search.Enabled() {
		s += "\n\nIMPORTANTE: seu conhecimento sobre jogos, patches, versões e fatos atuais está desatualizado. Quando perguntarem sobre esses temas, use a ferramenta buscar_web ANTES de responder. Não confie na sua memória para fatos que mudam com o tempo."
	}
	if c.memoryCtx != "" {
		s += "\n\n" + c.memoryCtx
	}
	if c.narrative != "" {
		s += "\n\nO que está acontecendo AGORA na live (use pra acompanhar o momento, referencie naturalmente):\n" + c.narrative
	}
	if c.extraCtx != "" {
		s += "\n\nContexto (mensagens recentes do chat da live, use APENAS quando a pergunta do streamer for sobre o chat; caso contrário responda normalmente e ignore este bloco; nunca invente mensagens):\n" + c.extraCtx
	}
	s += "\n\nPREFIXO DE HUMOR: comece TODA resposta com seu humor entre colchetes, um destes: [neutra] [animada] [zoeira] [entediada] [surpresa] [provocada]. Exemplo: \"[zoeira] morreu de novo, hein campeão\". O colchete não é falado, é só marcação."
	return s
}

// volumeToolDef: ferramenta de ajuste de volume.
func volumeToolDef() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "ajustar_volume",
			"description": "Ajusta o volume do sistema quando o streamer pede (ex: abaixa a música, aumenta o som). delta negativo abaixa, positivo aumenta.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"delta": map[string]any{"type": "integer", "description": "Variação em porcentagem, ex: -20 pra abaixar, 20 pra aumentar"},
				},
				"required": []string{"delta"},
			},
		},
	}
}

// musicToolDef: ferramenta de controle de música.
func musicToolDef() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "controlar_musica",
			"description": "Controla a música tocando quando o streamer pede (pausar, tocar, próxima, anterior).",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"action": map[string]any{"type": "string", "enum": []string{"play", "pause", "play-pause", "next", "previous"}},
				},
				"required": []string{"action"},
			},
		},
	}
}

// searchTool descreve a ferramenta de busca pro modelo.
func searchToolDef() map[string]any {
	return map[string]any{
		"type": "function",
		"function": map[string]any{
			"name":        "buscar_web",
			"description": "Busca informação atual na web. SEMPRE use esta ferramenta quando perguntarem sobre: patch notes, versão atual de jogos, último campeão/item/personagem adicionado, notícias, datas de hoje, resultados, ou qualquer fato que mude com o tempo. Seu conhecimento interno está DESATUALIZADO para esses temas — nunca responda de memória sobre eles, sempre busque. Só dispense a busca em conversa casual ou opinião pura.",
			"parameters": map[string]any{
				"type": "object",
				"properties": map[string]any{
					"query": map[string]any{"type": "string", "description": "termo de busca claro e específico"},
				},
				"required": []string{"query"},
			},
		},
	}
}

// Think: conversa com o streamer, com busca web opcional via function calling.
func (c *Client) Think(userText string) (string, error) {
	c.history = append(c.history, message{Role: "user", Content: userText})

	msgs := []message{{Role: "system", Content: c.systemPrompt()}}
	start := 0
	if len(c.history) > 10 {
		start = len(c.history) - 10
	}
	msgs = append(msgs, c.history[start:]...)

	// até 2 rodadas: modelo pode pedir busca, a gente responde, ele conclui
	for round := 0; round < 3; round++ {
		reqBody := map[string]any{
			"model":      c.model,
			"messages":   msgs,
			"max_tokens": 150,
		}
		if c.search != nil && c.search.Enabled() {
			reqBody["tools"] = []any{searchToolDef(), volumeToolDef(), musicToolDef()}
		}
		body, _ := json.Marshal(reqBody)

		req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("Authorization", "Bearer "+c.apiKey)

		resp, err := c.http.Do(req)
		if err != nil {
			return "", fmt.Errorf("openai: %w", err)
		}
		var out struct {
			Choices []struct {
				Message struct {
					Content   string     `json:"content"`
					ToolCalls []toolCall `json:"tool_calls"`
				} `json:"message"`
			} `json:"choices"`
		}
		if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
			resp.Body.Close()
			return "", err
		}
		resp.Body.Close()
		if len(out.Choices) == 0 {
			return "", fmt.Errorf("openai: resposta vazia")
		}
		m := out.Choices[0].Message

		// sem tool call: resposta final
		if len(m.ToolCalls) == 0 {
			c.history = append(c.history, message{Role: "assistant", Content: m.Content})
			return m.Content, nil
		}

		// modelo pediu busca(s): registra a intenção e executa cada uma
		msgs = append(msgs, message{Role: "assistant", ToolCalls: m.ToolCalls})
		for _, tc := range m.ToolCalls {
			var result string
			switch tc.Function.Name {
			case "buscar_web":
				var args struct {
					Query string `json:"query"`
				}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)
				log.Printf("[busca] %s", args.Query)
				r, err := c.search.Search(args.Query)
				if err != nil {
					r = "busca falhou: " + err.Error()
				}
				result = r
			case "ajustar_volume":
				var args struct {
					Delta int `json:"delta"`
				}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)
				log.Printf("[volume] delta=%d", args.Delta)
				result = tools.SetVolume(args.Delta, false)
			case "controlar_musica":
				var args struct {
					Action string `json:"action"`
				}
				json.Unmarshal([]byte(tc.Function.Arguments), &args)
				log.Printf("[música] %s", args.Action)
				result = tools.MusicControl(args.Action)
			default:
				result = "ferramenta desconhecida"
			}
			msgs = append(msgs, message{Role: "tool", ToolCallID: tc.ID, Content: result})
		}
	}
	return "", fmt.Errorf("openai: muitas rodadas de tool")
}

// ThinkWithVision: texto + screenshot (sem busca; visão e busca não se misturam por ora).
func (c *Client) ThinkWithVision(userText, imagePath string) (string, error) {
	img, err := os.ReadFile(imagePath)
	if err != nil {
		return "", fmt.Errorf("ler screenshot: %w", err)
	}
	b64 := base64.StdEncoding.EncodeToString(img)

	visionMsg := message{Role: "user", Content: []part{
		{Type: "text", Text: userText + "\n\n(A imagem é a tela atual do streamer, use como contexto APENAS se a pergunta for sobre o jogo/tela. Se for sobre o chat ou outra coisa, responda a pergunta e ignore a imagem.)"},
		{Type: "image_url", ImageURL: &imageURL{URL: "data:image/png;base64," + b64, Detail: "low"}},
	}}

	histEntry := message{Role: "user", Content: userText}
	c.history = append(c.history, histEntry)

	msgs := []message{{Role: "system", Content: c.systemPrompt()}}
	start := 0
	if len(c.history) > 10 {
		start = len(c.history) - 10
	}
	msgs = append(msgs, c.history[start:len(c.history)-1]...)
	msgs = append(msgs, visionMsg)

	reply, err := c.rawChat(msgs)
	if err != nil {
		return "", err
	}
	c.history = append(c.history, message{Role: "assistant", Content: reply})
	return reply, nil
}

func (c *Client) ThinkStateless(prompt string) (string, error) {
	return c.rawChat([]message{{Role: "user", Content: prompt}})
}

func (c *Client) VisionStateless(prompt, imagePath string) (string, error) {
	img, err := os.ReadFile(imagePath)
	if err != nil {
		return "", err
	}
	b64 := base64.StdEncoding.EncodeToString(img)
	return c.rawChat([]message{{Role: "user", Content: []part{
		{Type: "text", Text: prompt},
		{Type: "image_url", ImageURL: &imageURL{URL: "data:image/png;base64," + b64, Detail: "low"}},
	}}})
}

// rawChat: chamada simples sem tools, sem histórico.
// doWithRetry tenta a request até 3x com backoff. Recria o body a cada tentativa
// (POST consome o body na 1ª, sem recriar as seguintes iriam vazias).
func (c *Client) doWithRetry(req *http.Request, body []byte) (*http.Response, error) {
	var lastErr error
	for i := 0; i < 3; i++ {
		if body != nil {
			req.Body = io.NopCloser(bytes.NewReader(body))
		}
		resp, err := c.http.Do(req)
		if err == nil && resp.StatusCode < 500 {
			return resp, nil
		}
		if resp != nil {
			resp.Body.Close()
		}
		lastErr = err
		time.Sleep(time.Duration(i+1) * 500 * time.Millisecond)
	}
	if lastErr == nil {
		lastErr = fmt.Errorf("falha após retries")
	}
	return nil, lastErr
}

func (c *Client) rawChat(msgs []message) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"model":      c.model,
		"messages":   msgs,
		"max_tokens": 150,
	})
	req, _ := http.NewRequest("POST", "https://api.openai.com/v1/chat/completions", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	resp, err := c.doWithRetry(req, body)
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
	return out.Choices[0].Message.Content, nil
}


// ExtractMood separa o prefixo [humor] do texto. Devolve (humor, textoLimpo).
// Se não houver prefixo válido, devolve ("neutra", textoOriginal).
func ExtractMood(text string) (string, string) {
	t := strings.TrimSpace(text)
	if !strings.HasPrefix(t, "[") {
		return "neutra", t
	}
	end := strings.Index(t, "]")
	if end < 0 {
		return "neutra", t
	}
	mood := strings.ToLower(strings.TrimSpace(t[1:end]))
	clean := strings.TrimSpace(t[end+1:])
	valid := map[string]bool{"neutra": true, "animada": true, "zoeira": true, "entediada": true, "surpresa": true, "provocada": true}
	if !valid[mood] {
		return "neutra", t
	}
	return mood, clean
}