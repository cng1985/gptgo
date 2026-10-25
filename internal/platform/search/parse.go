package search

import (
	"encoding/json"
	"html"
	"net"
	"net/url"
	"path"
	"regexp"
	"strconv"
	"strings"
	"unicode/utf8"
)

var (
	resultAnchor = regexp.MustCompile(`(?is)<a\b[^>]*class\s*=\s*['"][^'"]*(?:result__a|result-link)[^'"]*['"][^>]*>.*?</a>`)
	resultBlurb  = regexp.MustCompile(`(?is)<(?:a|td|span)\b[^>]*class\s*=\s*['"][^'"]*(?:result__snippet|result-snippet)[^'"]*['"][^>]*>(.*?)</(?:a|td|span)>`)
	hrefAttr     = regexp.MustCompile(`(?i)\bhref\s*=\s*['"]([^'"]*)['"]`)
	htmlComment  = regexp.MustCompile(`(?s)<!--.*?-->`)
	bingTitle    = regexp.MustCompile(`(?is)<h2\b[^>]*>\s*<a\b[^>]*\bhref\s*=\s*['"]([^'"]+)['"][^>]*>(.*?)</a>`)
	bingSnippet  = regexp.MustCompile(`(?is)<p\b[^>]*\bb_lineclamp\d\b[^>]*>(.*?)</p>`)
	baiduMu      = regexp.MustCompile(`"mu"\s*:\s*"((?:\\.|[^"\\])*)"`)
	baiduSText   = regexp.MustCompile(`(?s)<!--s-text-->(.*?)<!--/s-text-->`)
	baiduSummary = regexp.MustCompile(`"text"\s*:\s*"((?:\\.|[^"\\])*)"`)
	sogouAnchor  = regexp.MustCompile(`(?is)<h3\b[^>]*\bvr-title\b[^>]*>\s*<a\b([^>]*)>(.*?)</a>`)
	sogouSnippet = regexp.MustCompile(`(?is)<(?:p|span)\b[^>]*class\s*=\s*['"][^'"]*(?:space-txt|star-wiki)[^'"]*['"][^>]*>(.*?)</(?:p|span)>`)
	soTitle      = regexp.MustCompile(`(?is)<h3\b[^>]*\bres-title\b[^>]*>(.*?)</h3>`)
	soSnippet    = regexp.MustCompile(`(?is)<p\b[^>]*\bsummary\b[^>]*>(.*?)</p>`)
	toutiaoTitle = regexp.MustCompile(`"title":"((?:\\.|[^"\\])*)"`)
	toutiaoText  = regexp.MustCompile(`"summary_text":"((?:\\.|[^"\\])*)"`)
	toutiaoURL   = regexp.MustCompile(`"url":"((?:\\.|[^"\\])*)"`)
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
			href = unescapeURL(match[1])
		}
		title := ""
		lower := strings.ToLower(anchor)
		gt := strings.Index(anchor, ">")
		end := strings.LastIndex(lower, "</a>")
		if gt >= 0 && end > gt {
			title = anchor[gt+1 : end]
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
	raw = strings.TrimSpace(unescapeURL(raw))
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

// unescapeURL 只还原 &amp;。完整的 HTML 反转义会把查询串里的 &timestamp 吃成乘号。
func unescapeURL(raw string) string {
	for {
		lower := strings.ToLower(raw)
		i := strings.Index(lower, "&amp;")
		if i < 0 {
			return raw
		}
		raw = raw[:i] + "&" + raw[i+5:]
	}
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
	value = htmlComment.ReplaceAllString(value, "")
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
		body := strings.TrimSpace(hit.Excerpt)
		if utf8.RuneCountInString(body) < utf8.RuneCountInString(hit.Snippet) {
			body = hit.Snippet
		}
		if body != "" {
			b.WriteByte('\n')
			b.WriteString(body)
		}
	}
	return clip(b.String(), 6000)
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

func tagEngine(hits []Hit, engine string) []Hit {
	if engine == "" {
		return hits
	}
	for i := range hits {
		if hits[i].Engine == "" {
			hits[i].Engine = engine
		}
	}
	return hits
}

func parseBingHTML(body string) []Hit {
	locs := bingTitle.FindAllStringSubmatchIndex(body, 8)
	hits := make([]Hit, 0, len(locs))
	for _, loc := range locs {
		rawURL := unescapeURL(body[loc[2]:loc[3]])
		title := body[loc[4]:loc[5]]
		window := after(body, loc[1], 700)
		snippet := ""
		if match := bingSnippet.FindStringSubmatch(window); len(match) > 1 {
			snippet = match[1]
		}
		hit, ok := acceptEngineHit(title, rawURL, snippet)
		if ok {
			hits = append(hits, hit)
		}
		if len(hits) == 5 {
			break
		}
	}
	return tagEngine(hits, "必应")
}

func parseBaiduHTML(body string) []Hit {
	locs := baiduMu.FindAllStringSubmatchIndex(body, 12)
	hits := make([]Hit, 0, len(locs))
	for _, loc := range locs {
		rawURL := decodeJSString(body[loc[2]:loc[3]])
		window := after(body, loc[1], 8000)
		if next := strings.Index(window, `"mu":`); next > 0 {
			window = window[:next]
		}
		title := ""
		if match := baiduSText.FindStringSubmatch(window); len(match) > 1 {
			title = match[1]
		}
		snippet := ""
		if match := baiduSummary.FindStringSubmatch(window); len(match) > 1 {
			snippet = decodeJSString(match[1])
		}
		hit, ok := acceptEngineHit(title, rawURL, snippet)
		if ok {
			hits = append(hits, hit)
		}
		if len(hits) == 5 {
			break
		}
	}
	return tagEngine(hits, "百度")
}

func parseSogouHTML(body string) []Hit {
	locs := sogouAnchor.FindAllStringSubmatchIndex(body, 10)
	hits := make([]Hit, 0, len(locs))
	for _, loc := range locs {
		rawURL := sogouURL(body[loc[2]:loc[3]])
		title := body[loc[4]:loc[5]]
		window := after(body, loc[1], 1600)
		if next := strings.Index(window, "vr-title"); next > 0 {
			window = window[:next]
		}
		snippet := ""
		if match := sogouSnippet.FindStringSubmatch(window); len(match) > 1 {
			snippet = match[1]
		}
		hit, ok := acceptEngineHit(title, rawURL, snippet)
		if ok {
			hits = append(hits, hit)
		}
		if len(hits) == 5 {
			break
		}
	}
	return tagEngine(hits, "搜狗")
}

func parseSoHTML(body string) []Hit {
	const marker = `data-pcurl="`
	hits := make([]Hit, 0, 5)
	from := 0
	for len(hits) < 5 {
		rel := strings.Index(body[from:], marker)
		if rel < 0 {
			break
		}
		start := from + rel + len(marker)
		rest := body[start:]
		end := strings.IndexByte(rest, '"')
		if end < 0 {
			break
		}
		rawURL := unescapeURL(rest[:end])
		window := rest[end:]
		if len(window) > 2000 {
			window = window[:2000]
		}
		if next := strings.Index(window, marker); next > 0 {
			window = window[:next]
		}
		title := ""
		if match := soTitle.FindStringSubmatch(window); len(match) > 1 {
			title = match[1]
		}
		snippet := ""
		if match := soSnippet.FindStringSubmatch(window); len(match) > 1 {
			snippet = match[1]
		}
		if hit, ok := acceptEngineHit(title, rawURL, snippet); ok {
			hits = append(hits, hit)
		}
		from = start
	}
	return tagEngine(hits, "360搜索")
}

func parseToutiaoHTML(body string) []Hit {
	locs := toutiaoTitle.FindAllStringSubmatchIndex(body, 24)
	hits := make([]Hit, 0, len(locs))
	seen := map[string]struct{}{}
	for _, loc := range locs {
		title := decodeJSString(body[loc[2]:loc[3]])
		if toutiaoNoise(title) {
			continue
		}
		window := after(body, loc[1], 2000)
		match := toutiaoURL.FindStringSubmatch(window)
		if len(match) < 2 {
			continue
		}
		rawURL := decodeJSString(match[1])
		snippet := ""
		if text := toutiaoText.FindStringSubmatch(window); len(text) > 1 {
			snippet = decodeJSString(text[1])
		}
		hit, ok := acceptEngineHit(title, rawURL, snippet)
		if !ok {
			continue
		}
		if _, ok := seen[hit.URL]; ok {
			continue
		}
		seen[hit.URL] = struct{}{}
		hits = append(hits, hit)
		if len(hits) == 5 {
			break
		}
	}
	return tagEngine(hits, "头条搜索")
}

func after(body string, start, width int) string {
	if start < 0 || start >= len(body) {
		return ""
	}
	end := start + width
	if end > len(body) {
		end = len(body)
	}
	return body[start:end]
}

func sogouURL(attrs string) string {
	if link := attrLoose(attrs, "linkurl"); absoluteHTTP(link) {
		return link
	}
	href := attrLoose(attrs, "href")
	if strings.HasPrefix(href, "//") {
		href = "https:" + href
	}
	if absoluteHTTP(href) {
		return href
	}
	return ""
}

func absoluteHTTP(raw string) bool {
	return strings.HasPrefix(raw, "http://") || strings.HasPrefix(raw, "https://")
}

func attrLoose(tag, name string) string {
	pattern := `(?i)(?:^|[\s])` + regexp.QuoteMeta(name) + `\s*=\s*(?:'([^']*)'|"([^"]*)"|([^\s"'=<>` + "`" + `]+))`
	match := regexp.MustCompile(pattern).FindStringSubmatch(tag)
	if len(match) == 0 {
		return ""
	}
	for _, part := range match[1:] {
		if part != "" {
			return unescapeURL(part)
		}
	}
	return ""
}

func acceptEngineHit(title, rawURL, snippet string) (Hit, bool) {
	hit, ok := cleanHit(title, rawURL, snippet)
	if !ok || skipEngineResult(hit.URL) {
		return Hit{}, false
	}
	return hit, true
}

func skipEngineResult(raw string) bool {
	parsed, err := url.Parse(raw)
	if err != nil {
		return true
	}
	host := strings.ToLower(parsed.Hostname())
	switch host {
	case "bing.com", "www.bing.com", "cn.bing.com",
		"baidu.com", "www.baidu.com", "m.baidu.com",
		"sogou.com", "www.sogou.com", "m.sogou.com",
		"so.com", "www.so.com", "m.so.com",
		"duckduckgo.com", "www.duckduckgo.com", "html.duckduckgo.com", "lite.duckduckgo.com",
		"so.toutiao.com", "toutiaoimg.com":
		return true
	}
	if strings.Contains(host, "toutiaoimg.com") || strings.Contains(host, "byteimg.com") || strings.Contains(host, "recommend_list.baidu.com") {
		return true
	}
	switch strings.ToLower(path.Ext(parsed.Path)) {
	case ".jpg", ".jpeg", ".png", ".gif", ".webp", ".svg", ".ico":
		return true
	}
	return false
}

func toutiaoNoise(title string) bool {
	title = strings.TrimSpace(stripTags(title))
	switch title {
	case "", "资讯", "图片", "视频", "搜索", "网页", "更多", "站内", "热搜":
		return true
	}
	return utf8.RuneCountInString(title) < 4
}

func decodeJSString(value string) string {
	value = strings.ReplaceAll(value, `\/`, `/`)
	if unquoted, err := strconv.Unquote(`"` + value + `"`); err == nil {
		return unquoted
	}
	return value
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
