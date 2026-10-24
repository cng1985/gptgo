package search

import (
	"encoding/json"
	"html"
	"net"
	"net/url"
	"regexp"
	"strings"
	"unicode/utf8"
)

var (
	resultAnchor = regexp.MustCompile(`(?is)<a\b[^>]*class="[^"]*(?:result__a|result-link)[^"]*"[^>]*>.*?</a>`)
	resultBlurb  = regexp.MustCompile(`(?is)<(?:a|td|span)\b[^>]*class="[^"]*(?:result__snippet|result-snippet)[^"]*"[^>]*>(.*?)</(?:a|td|span)>`)
	hrefAttr     = regexp.MustCompile(`(?i)\bhref\s*=\s*"([^"]*)"`)
)

func parseDuckHTML(body string) []Hit {
	anchors := resultAnchor.FindAllString(body, 8)
	blurbs := resultBlurb.FindAllStringSubmatch(body, 8)
	hits := make([]Hit, 0, len(anchors))
	for i, anchor := range anchors {
		snippet := ""
		if i < len(blurbs) && len(blurbs[i]) > 1 {
			snippet = blurbs[i][1]
		}
		href := ""
		if match := hrefAttr.FindStringSubmatch(anchor); len(match) > 1 {
			href = html.UnescapeString(match[1])
		}
		title := ""
		if gt := strings.Index(anchor, ">"); gt >= 0 && gt+1 < len(anchor)-4 {
			title = anchor[gt+1 : len(anchor)-4]
		}
		hit, ok := cleanHit(title, unwrapDuck(href), snippet)
		if ok {
			hits = append(hits, hit)
		}
	}
	return hits
}

func parseDuckJSON(raw []byte) []Hit {
	var payload struct {
		Heading       string `json:"Heading"`
		AbstractText  string `json:"AbstractText"`
		AbstractURL   string `json:"AbstractURL"`
		RelatedTopics []any  `json:"RelatedTopics"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	hits := make([]Hit, 0, 6)
	if hit, ok := cleanHit(payload.Heading, payload.AbstractURL, payload.AbstractText); ok {
		hits = append(hits, hit)
	}
	collectTopics(payload.RelatedTopics, &hits)
	if len(hits) > 5 {
		return hits[:5]
	}
	return hits
}

func collectTopics(items []any, hits *[]Hit) {
	for _, item := range items {
		obj, ok := item.(map[string]any)
		if !ok {
			continue
		}
		if nested, ok := obj["Topics"].([]any); ok {
			collectTopics(nested, hits)
			continue
		}
		text, _ := obj["Text"].(string)
		link, _ := obj["FirstURL"].(string)
		title, snippet := splitTopic(text)
		hit, ok := cleanHit(title, link, snippet)
		if ok {
			*hits = append(*hits, hit)
		}
		if len(*hits) >= 5 {
			return
		}
	}
}

func splitTopic(text string) (string, string) {
	text = strings.TrimSpace(text)
	if text == "" {
		return "", ""
	}
	if title, snippet, ok := strings.Cut(text, " - "); ok && strings.TrimSpace(title) != "" {
		return title, snippet
	}
	return text, ""
}

func parseWiki(raw []byte) []Hit {
	var payload struct {
		Query struct {
			Pages []struct {
				Title   string `json:"title"`
				Index   int    `json:"index"`
				Extract string `json:"extract"`
				FullURL string `json:"fullurl"`
			} `json:"pages"`
		} `json:"query"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	pages := payload.Query.Pages
	for i := 1; i < len(pages); i++ {
		item := pages[i]
		j := i
		for j > 0 && pages[j-1].Index > item.Index {
			pages[j] = pages[j-1]
			j--
		}
		pages[j] = item
	}
	hits := make([]Hit, 0, len(pages))
	for _, page := range pages {
		hit, ok := cleanHit(page.Title, page.FullURL, page.Extract)
		if ok {
			hits = append(hits, hit)
		}
	}
	return hits
}

func parseBing(raw []byte) []Hit {
	var payload struct {
		WebPages struct {
			Value []struct {
				Name    string `json:"name"`
				URL     string `json:"url"`
				Snippet string `json:"snippet"`
			} `json:"value"`
		} `json:"webPages"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	hits := make([]Hit, 0, len(payload.WebPages.Value))
	for _, item := range payload.WebPages.Value {
		hit, ok := cleanHit(item.Name, item.URL, item.Snippet)
		if ok {
			hits = append(hits, hit)
		}
	}
	return hits
}

func parseTavily(raw []byte) []Hit {
	var payload struct {
		Results []struct {
			Title   string `json:"title"`
			URL     string `json:"url"`
			Content string `json:"content"`
		} `json:"results"`
	}
	if err := json.Unmarshal(raw, &payload); err != nil {
		return nil
	}
	hits := make([]Hit, 0, len(payload.Results))
	for _, item := range payload.Results {
		hit, ok := cleanHit(item.Title, item.URL, item.Content)
		if ok {
			hits = append(hits, hit)
		}
	}
	return hits
}

func cleanHit(title, rawURL, snippet string) (Hit, bool) {
	title = clip(stripTags(title), 120)
	snippet = clip(stripTags(snippet), 240)
	link, ok := cleanURL(rawURL)
	if !ok || title == "" {
		return Hit{}, false
	}
	return Hit{Title: title, URL: link, Snippet: snippet}, true
}

func cleanURL(raw string) (string, bool) {
	raw = strings.TrimSpace(html.UnescapeString(raw))
	raw = unwrapDuck(raw)
	parsed, err := url.Parse(raw)
	if err != nil || parsed.Host == "" || parsed.User != nil {
		return "", false
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return "", false
	}
	host := strings.ToLower(parsed.Hostname())
	if host == "" || host == "localhost" || strings.HasSuffix(host, ".localhost") {
		return "", false
	}
	if strings.Contains(host, "duckduckgo.com") && strings.Contains(parsed.Path, "/l/") {
		return "", false
	}
	if ip := net.ParseIP(host); ip != nil && blockedIP(ip, false) {
		return "", false
	}
	parsed.Fragment = ""
	link := parsed.String()
	if len(link) > 500 || !utf8.ValidString(link) {
		return "", false
	}
	return link, true
}

func unwrapDuck(raw string) string {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "//") {
		raw = "https:" + raw
	}
	parsed, err := url.Parse(raw)
	if err != nil {
		return raw
	}
	target := parsed.Query().Get("uddg")
	if target == "" {
		return raw
	}
	if decoded, err := url.QueryUnescape(target); err == nil && decoded != "" {
		return decoded
	}
	return target
}

func stripTags(value string) string {
	var b strings.Builder
	in := false
	for _, r := range value {
		switch {
		case r == '<':
			in = true
		case r == '>':
			in = false
		case !in:
			b.WriteRune(r)
		}
	}
	return strings.Join(strings.Fields(html.UnescapeString(b.String())), " ")
}

func formatNotes(hits []Hit) string {
	var b strings.Builder
	for i, hit := range hits {
		if i > 0 {
			b.WriteString("\n\n")
		}
		b.WriteString(itoa(i + 1))
		b.WriteString(". ")
		b.WriteString(hit.Title)
		b.WriteByte('\n')
		b.WriteString(hit.URL)
		if hit.Snippet != "" {
			b.WriteByte('\n')
			b.WriteString(hit.Snippet)
		}
	}
	return clip(b.String(), 4000)
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	var digits [4]byte
	i := len(digits)
	for n > 0 && i > 0 {
		i--
		digits[i] = byte('0' + n%10)
		n /= 10
	}
	return string(digits[i:])
}

func clip(text string, max int) string {
	rs := []rune(strings.TrimSpace(text))
	if len(rs) <= max {
		return string(rs)
	}
	return string(rs[:max])
}

func mergeHits(groups ...[]Hit) []Hit {
	seen := map[string]struct{}{}
	out := make([]Hit, 0, 5)
	for _, group := range groups {
		for _, hit := range group {
			key := strings.ToLower(hit.URL)
			if _, ok := seen[key]; ok {
				continue
			}
			seen[key] = struct{}{}
			out = append(out, hit)
			if len(out) == 5 {
				return out
			}
		}
	}
	return out
}
