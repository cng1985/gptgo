package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"
)

type Searcher struct {
	client *http.Client
}

func NewSearcher() *Searcher {
	return &Searcher{client: &http.Client{Timeout: 8 * time.Second}}
}

type topic struct {
	Text   string  `json:"Text"`
	Topics []topic `json:"Topics"`
}

func (s *Searcher) Search(ctx context.Context, query string) (string, error) {
	query = strings.TrimSpace(query)
	if query == "" {
		return "", nil
	}
	endpoint := "https://api.duckduckgo.com/?format=json&no_html=1&skip_disambig=1&q=" + url.QueryEscape(query)
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "ChatBI-AI/1.0")
	resp, err := s.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", fmt.Errorf("检索服务返回 %d", resp.StatusCode)
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", err
	}
	var parsed struct {
		Heading       string  `json:"Heading"`
		AbstractText  string  `json:"AbstractText"`
		RelatedTopics []topic `json:"RelatedTopics"`
	}
	if err := json.Unmarshal(raw, &parsed); err != nil {
		return "", err
	}
	lines := make([]string, 0, 6)
	if text := strings.TrimSpace(parsed.AbstractText); text != "" {
		heading := strings.TrimSpace(parsed.Heading)
		if heading != "" {
			lines = append(lines, heading+"："+clip(text, 400))
		} else {
			lines = append(lines, clip(text, 400))
		}
	}
	for _, item := range flattenTopics(parsed.RelatedTopics) {
		if len(lines) >= 5 {
			break
		}
		lines = append(lines, clip(item, 240))
	}
	return strings.Join(lines, "\n"), nil
}

func flattenTopics(items []topic) []string {
	out := make([]string, 0, len(items))
	for _, item := range items {
		if text := strings.TrimSpace(item.Text); text != "" {
			out = append(out, text)
		}
		out = append(out, flattenTopics(item.Topics)...)
	}
	return out
}

func clip(text string, max int) string {
	rs := []rune(strings.TrimSpace(text))
	if len(rs) <= max {
		return string(rs)
	}
	return string(rs[:max]) + "…"
}
