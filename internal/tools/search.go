package tools

import (
	"bytes"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type Searcher struct {
	apiKey string
	http   *http.Client
}

func NewSearcher(apiKey string) *Searcher {
	return &Searcher{apiKey: apiKey, http: &http.Client{Timeout: 15 * time.Second}}
}

func (s *Searcher) Enabled() bool { return s.apiKey != "" }

func (s *Searcher) Search(query string) (string, error) {
	body, _ := json.Marshal(map[string]any{
		"api_key":        s.apiKey,
		"query":          query,
		"search_depth":   "basic",
		"include_answer": true,
		"max_results":    3,
	})
	resp, err := s.http.Post("https://api.tavily.com/search", "application/json", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("tavily: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("tavily status %d", resp.StatusCode)
	}

	var out struct {
		Answer  string `json:"answer"`
		Results []struct {
			Title   string `json:"title"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return "", err
	}

	var sb strings.Builder
	if out.Answer != "" {
		sb.WriteString(out.Answer)
		sb.WriteString("\n")
	}
	for _, r := range out.Results {
		sb.WriteString("- ")
		sb.WriteString(r.Title)
		sb.WriteString(": ")
		content := r.Content
		if len(content) > 300 {
			content = content[:300]
		}
		sb.WriteString(content)
		sb.WriteString("\n")
	}
	if sb.Len() == 0 {
		return "Nenhum resultado encontrado.", nil
	}
	return sb.String(), nil
}
