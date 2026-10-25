package search

import (
	"context"
	"errors"
	"html"
	"mime"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"sync"
	"time"
	"unicode"
	"unicode/utf8"
)

const (
	pageReadLimit = 3
	pageBytes     = 512 << 10
	pageExcerpt   = 480
)

var (
	hiddenBlocks = []*regexp.Regexp{
		regexp.MustCompile(`(?is)<script\b[^>]*>.*?</script>`),
		regexp.MustCompile(`(?is)<style\b[^>]*>.*?</style>`),
		regexp.MustCompile(`(?is)<noscript\b[^>]*>.*?</noscript>`),
		regexp.MustCompile(`(?is)<svg\b[^>]*>.*?</svg>`),
		regexp.MustCompile(`(?is)<iframe\b[^>]*>.*?</iframe>`),
		regexp.MustCompile(`(?is)<header\b[^>]*>.*?</header>`),
		regexp.MustCompile(`(?is)<footer\b[^>]*>.*?</footer>`),
		regexp.MustCompile(`(?is)<nav\b[^>]*>.*?</nav>`),
		regexp.MustCompile(`(?is)<aside\b[^>]*>.*?</aside>`),
		regexp.MustCompile(`(?is)<form\b[^>]*>.*?</form>`),
	}
	blockEnd = regexp.MustCompile(`(?i)</(?:p|div|li|tr|h[1-6]|section|article)\s*>|<br\s*/?>`)
)

func (c *Client) enrich(ctx context.Context, query string, hits []Hit) {
	if c == nil || !c.fetchPages || len(hits) == 0 || ctx.Err() != nil {
		return
	}
	indexes := pageIndexes(hits)
	var group sync.WaitGroup
	for _, index := range indexes {
		group.Add(1)
		go func(index int) {
			defer group.Done()
			pageCtx, cancel := context.WithTimeout(ctx, 4*time.Second)
			defer cancel()
			text, err := c.readPage(pageCtx, hits[index].URL)
			if err != nil {
				return
			}
			text = focusExcerpt(query, hits[index].Title, text)
			if utf8.RuneCountInString(text) < 40 {
				return
			}
			hits[index].Excerpt = clip(text, pageExcerpt)
		}(index)
	}
	group.Wait()
}

func pageIndexes(hits []Hit) []int {
	seen := map[string]struct{}{}
	indexes := make([]int, 0, pageReadLimit)
	for i, hit := range hits {
		host := strings.ToLower(hostOf(hit.URL))
		if host == "" {
			continue
		}
		if _, ok := seen[host]; ok {
			continue
		}
		seen[host] = struct{}{}
		indexes = append(indexes, i)
		if len(indexes) == pageReadLimit {
			break
		}
	}
	return indexes
}

func hostOf(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func (c *Client) readPage(ctx context.Context, rawURL string) (string, error) {
	parsed, err := url.Parse(strings.TrimSpace(rawURL))
	if err != nil {
		return "", err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.8")
	raw, _, err := c.roundTrip(req, fetchPage, pageBytes)
	if err != nil {
		return "", err
	}
	if !utf8.Valid(raw) {
		return "", errUnreadable
	}
	return readableText(string(raw)), nil
}

var errUnreadable = errors.New("不是可阅读的网页")

func pageTypeOK(ctype string) bool {
	ctype = strings.TrimSpace(ctype)
	if ctype == "" {
		return true
	}
	media, params, err := mime.ParseMediaType(ctype)
	if err != nil {
		return false
	}
	switch strings.ToLower(media) {
	case "text/html", "text/plain", "application/xhtml+xml":
		return charsetOK(params["charset"])
	default:
		return false
	}
}

func charsetOK(charset string) bool {
	charset = strings.ToLower(strings.TrimSpace(charset))
	switch charset {
	case "", "utf-8", "utf8", "us-ascii":
		return true
	default:
		return false
	}
}

func readableText(raw string) string {
	if !utf8.ValidString(raw) {
		return ""
	}
	for _, pattern := range hiddenBlocks {
		raw = pattern.ReplaceAllString(raw, "\n")
	}
	raw = blockEnd.ReplaceAllString(raw, "\n")
	text := stripTagsPreserveLines(raw)
	var kept []string
	total := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if utf8.RuneCountInString(line) < 18 {
			continue
		}
		kept = append(kept, line)
		total += utf8.RuneCountInString(line)
		if total >= pageExcerpt+40 {
			break
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func focusExcerpt(query, title, text string) string {
	terms := queryTerms(query, title)
	if len(terms) == 0 {
		return strings.TrimSpace(text)
	}
	var kept []string
	started := false
	total := 0
	for _, line := range strings.Split(text, "\n") {
		line = strings.Join(strings.Fields(line), " ")
		if line == "" || boilerplateLine(line) {
			continue
		}
		if !relevantLine(line, terms, started) {
			if started {
				break
			}
			continue
		}
		started = true
		kept = append(kept, line)
		total += utf8.RuneCountInString(line)
		if total >= pageExcerpt+40 {
			break
		}
	}
	return strings.TrimSpace(strings.Join(kept, "\n"))
}

func queryTerms(query, title string) []string {
	loosened := query
	for _, stop := range []string{"请问", "帮我", "一下", "今天", "今日", "现在", "最新", "如何", "怎么", "什么", "哪些"} {
		loosened = strings.ReplaceAll(loosened, stop, " ")
	}
	seen := map[string]struct{}{}
	var terms []string
	add := func(term string) {
		term = strings.TrimSpace(term)
		if utf8.RuneCountInString(term) < 2 {
			return
		}
		if _, ok := seen[term]; ok {
			return
		}
		seen[term] = struct{}{}
		terms = append(terms, term)
	}
	for _, field := range splitTerms(loosened + " " + title) {
		add(field)
		rs := []rune(field)
		if len(rs) >= 4 {
			add(string(rs[:2]))
			add(string(rs[len(rs)-2:]))
		}
	}
	return terms
}

func splitTerms(text string) []string {
	return strings.FieldsFunc(text, func(r rune) bool {
		return unicode.IsSpace(r) || unicode.IsPunct(r) || r == '【' || r == '】' || r == '_' || r == '|'
	})
}

func relevantLine(line string, terms []string, started bool) bool {
	if containsTerm(line, terms) {
		return true
	}
	return started && hasMeasure(line)
}

func containsTerm(line string, terms []string) bool {
	for _, term := range terms {
		if strings.Contains(line, term) {
			return true
		}
	}
	return false
}

func hasMeasure(line string) bool {
	return strings.Contains(line, "℃") || strings.Contains(line, "°") || strings.Contains(line, "%") || strings.Contains(line, "度") || strings.Contains(line, "级")
}

func boilerplateLine(line string) bool {
	for _, mark := range []string{"版权", "备案", "ICP", "主办", "邮政编码", "网站标识码", "Copyright", "copyright"} {
		if strings.Contains(line, mark) {
			return true
		}
	}
	return false
}

func stripTagsPreserveLines(value string) string {
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
	return html.UnescapeString(b.String())
}
