package search

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
	"unicode"

	"gptgo/internal/platform/config"
)

type Hit struct {
	Title   string `json:"title"`
	URL     string `json:"url"`
	Snippet string `json:"snippet"`
	Excerpt string `json:"excerpt,omitempty"`
}

// UnavailableNote 是检索没有可用结果时交给模型的说明。
const UnavailableNote = "本次联网检索没有返回结果。"

const (
	browserUA = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/128.0.0.0 Safari/537.36"
	apiUA     = "ChatBI/1.0 (local research assistant)"
)

type fetchKind int

const (
	fetchAPI fetchKind = iota + 1
	fetchHTML
	fetchPage
)

type fetchKindKey struct{}

type Result struct {
	Notes string
	Hits  []Hit
}

type Client struct {
	provider       string
	apiKey         string
	timeout        time.Duration
	http           *http.Client
	permitLoopback bool
	duckURL        string
	liteURL        string
	duckAPI        string
	wikiZH         string
	fetchPages     bool
	wikiEN         string
	bingURL        string
	tavilyURL      string
}

func New(cfg *config.Config) *Client {
	client := &Client{
		provider:   "duckduckgo",
		timeout:    12 * time.Second,
		fetchPages: true,
		duckURL:    "https://html.duckduckgo.com/html/",
		liteURL:    "https://lite.duckduckgo.com/lite/",
		duckAPI:    "https://api.duckduckgo.com/",
		wikiZH:     "https://zh.wikipedia.org/w/api.php",
		wikiEN:     "https://en.wikipedia.org/w/api.php",
		bingURL:    "https://api.bing.microsoft.com/v7.0/search",
		tavilyURL:  "https://api.tavily.com/search",
	}
	if cfg != nil {
		if provider := strings.ToLower(strings.TrimSpace(cfg.Search.Provider)); provider != "" {
			client.provider = provider
		}
		client.apiKey = strings.TrimSpace(cfg.Search.APIKey)
		if cfg.Search.Timeout.Std() > 0 {
			client.timeout = cfg.Search.Timeout.Std()
		}
	}
	client.http = client.newHTTP()
	return client
}

func (c *Client) Search(ctx context.Context, query string) (Result, error) {
	query = normalizeQuery(query)
	if query == "" {
		return Result{}, fmt.Errorf("检索词为空")
	}
	if c.provider == "off" {
		return Result{}, fmt.Errorf("联网检索已关闭")
	}
	budget := c.timeout
	if budget <= 0 {
		budget = 12 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, budget)
	defer cancel()
	discoverBudget := budget * 2 / 3
	if discoverBudget < 3*time.Second && budget >= 3*time.Second {
		discoverBudget = 3 * time.Second
	}
	if discoverBudget > budget {
		discoverBudget = budget
	}
	discoverCtx, cancelDiscover := context.WithTimeout(ctx, discoverBudget)
	result, err := c.discover(discoverCtx, query)
	cancelDiscover()
	if err != nil || len(result.Hits) == 0 {
		return result, err
	}
	if ctx.Err() == nil {
		c.enrich(ctx, query, result.Hits)
	}
	result.Notes = formatNotes(result.Hits)
	return result, nil
}

func (c *Client) discover(ctx context.Context, query string) (Result, error) {
	switch c.provider {
	case "bing":
		hits, err := c.bing(ctx, query)
		return pack(hits, err)
	case "tavily":
		hits, err := c.tavily(ctx, query)
		return pack(hits, err)
	default:
		return c.duck(ctx, query)
	}
}

func pack(hits []Hit, err error) (Result, error) {
	hits = mergeHits(hits)
	if len(hits) == 0 {
		if err != nil {
			return Result{}, err
		}
		return Result{}, fmt.Errorf("没有检索结果")
	}
	return Result{Notes: formatNotes(hits), Hits: hits}, nil
}

func (c *Client) duck(ctx context.Context, query string) (Result, error) {
	var (
		htmlHits []Hit
		wikiHits []Hit
		htmlErr  error
		wikiErr  error
		group    sync.WaitGroup
	)
	group.Add(2)
	go func() {
		defer group.Done()
		htmlHits, htmlErr = c.webIndex(ctx, query)
	}()
	go func() {
		defer group.Done()
		wikiHits, wikiErr = c.wiki(ctx, c.wikiZH, query)
	}()
	group.Wait()
	var first error
	if htmlErr != nil {
		first = htmlErr
	} else if wikiErr != nil {
		first = wikiErr
	}
	if len(wikiHits) > 2 {
		wikiHits = wikiHits[:2]
	}
	hits := mergeHits(htmlHits, wikiHits)
	if len(hits) < 2 && ctx.Err() == nil {
		more, err := c.duckJSON(ctx, query)
		if err != nil && first == nil {
			first = err
		}
		hits = mergeHits(hits, more)
	}
	if len(hits) == 0 && !hasHan(query) && ctx.Err() == nil {
		more, err := c.wiki(ctx, c.wikiEN, query)
		if err != nil && first == nil {
			first = err
		}
		hits = mergeHits(hits, more)
	}
	return pack(hits, first)
}

func (c *Client) webIndex(ctx context.Context, query string) ([]Hit, error) {
	hits, err := c.duckHTML(ctx, query)
	if len(hits) > 0 {
		return hits, nil
	}
	if ctx.Err() != nil {
		if err == nil {
			err = ctx.Err()
		}
		return nil, err
	}
	if strings.TrimSpace(c.liteURL) == "" {
		return nil, err
	}
	liteHits, liteErr := c.duckPage(ctx, c.liteURL, query)
	if len(liteHits) > 0 {
		return liteHits, nil
	}
	if err == nil {
		err = liteErr
	}
	return nil, err
}

func (c *Client) duckHTML(ctx context.Context, query string) ([]Hit, error) {
	return c.duckPage(ctx, c.duckURL, query)
}

func (c *Client) duckPage(ctx context.Context, endpoint, query string) ([]Hit, error) {
	// POST 会落到验证页并返回 202。GET 配合浏览器标识才能拿到结果；
	// 标识仍被拦截时，调用方会再读 Lite 页面。
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return nil, err
	}
	values := parsed.Query()
	values.Set("q", query)
	values.Set("kl", "cn-zh")
	parsed.RawQuery = values.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return nil, err
	}
	raw, _, err := c.roundTrip(req, fetchHTML, 1<<20)
	if err != nil {
		return nil, err
	}
	body := string(raw)
	if challengePage(body) {
		return nil, fmt.Errorf("检索服务要求验证")
	}
	return parseDuckHTML(body), nil
}

func challengePage(body string) bool {
	return strings.Contains(body, "anomaly-modal") && !strings.Contains(body, "result__a")
}

func (c *Client) duckJSON(ctx context.Context, query string) ([]Hit, error) {
	target := c.duckAPI + "?" + url.Values{
		"q":             {query},
		"format":        {"json"},
		"no_html":       {"1"},
		"no_redirect":   {"1"},
		"skip_disambig": {"1"},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	raw, err := c.do(req)
	if err != nil {
		return nil, err
	}
	return parseDuckJSON(raw), nil
}

func (c *Client) wiki(ctx context.Context, endpoint, query string) ([]Hit, error) {
	target := endpoint + "?" + url.Values{
		"action":        {"query"},
		"generator":     {"search"},
		"gsrsearch":     {query},
		"gsrlimit":      {"4"},
		"prop":          {"extracts|info"},
		"inprop":        {"url"},
		"exintro":       {"1"},
		"explaintext":   {"1"},
		"exchars":       {"220"},
		"format":        {"json"},
		"formatversion": {"2"},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	raw, err := c.do(req)
	if err != nil {
		return nil, err
	}
	return parseWiki(raw), nil
}

func (c *Client) bing(ctx context.Context, query string) ([]Hit, error) {
	target := c.bingURL + "?" + url.Values{
		"q":              {query},
		"count":          {"5"},
		"mkt":            {"zh-CN"},
		"responseFilter": {"Webpages"},
	}.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, target, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Ocp-Apim-Subscription-Key", c.apiKey)
	raw, err := c.do(req)
	if err != nil {
		return nil, err
	}
	return parseBing(raw), nil
}

func (c *Client) tavily(ctx context.Context, query string) ([]Hit, error) {
	body, err := json.Marshal(map[string]any{
		"query":          query,
		"max_results":    5,
		"search_depth":   "basic",
		"include_answer": false,
	})
	if err != nil {
		return nil, err
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.tavilyURL, bytes.NewReader(body))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	raw, err := c.do(req)
	if err != nil {
		return nil, err
	}
	return parseTavily(raw), nil
}

func (c *Client) do(req *http.Request) ([]byte, error) {
	body, _, err := c.roundTrip(req, fetchAPI, 1<<20)
	return body, err
}

func (c *Client) roundTrip(req *http.Request, kind fetchKind, limit int64) ([]byte, string, error) {
	req = req.WithContext(context.WithValue(req.Context(), fetchKindKey{}, kind))
	if err := c.allow(req.Context(), req.URL); err != nil {
		return nil, "", err
	}
	decorate(req, kind)
	resp, err := c.http.Do(req)
	if err != nil {
		if req.Context().Err() != nil {
			return nil, "", req.Context().Err()
		}
		if kind == fetchPage {
			return nil, "", fmt.Errorf("网页读取失败")
		}
		return nil, "", fmt.Errorf("检索请求失败")
	}
	defer resp.Body.Close()
	ctype := resp.Header.Get("Content-Type")
	if kind == fetchPage && !pageTypeOK(ctype) {
		return nil, ctype, fmt.Errorf("不是可阅读的网页")
	}
	raw, err := io.ReadAll(io.LimitReader(resp.Body, limit))
	if err != nil {
		if kind == fetchPage {
			return nil, ctype, fmt.Errorf("网页读取失败")
		}
		return nil, ctype, fmt.Errorf("检索请求失败")
	}
	if resp.StatusCode == http.StatusAccepted {
		return nil, ctype, fmt.Errorf("检索服务要求验证")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if kind == fetchPage {
			return nil, ctype, fmt.Errorf("网页读取失败")
		}
		return nil, ctype, fmt.Errorf("检索服务返回 %d", resp.StatusCode)
	}
	return raw, ctype, nil
}

func decorate(req *http.Request, kind fetchKind) {
	switch kind {
	case fetchHTML, fetchPage:
		req.Header.Set("User-Agent", browserUA)
		req.Header.Set("Accept-Language", "zh-CN,zh;q=0.9,en;q=0.6")
		if req.Header.Get("Accept") == "" {
			req.Header.Set("Accept", "text/html,application/xhtml+xml,text/plain;q=0.8,*/*;q=0.5")
		}
	default:
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", apiUA)
		}
		if req.Header.Get("Accept") == "" {
			req.Header.Set("Accept", "application/json,text/html;q=0.8,*/*;q=0.5")
		}
	}
}

func kindFrom(ctx context.Context) fetchKind {
	kind, _ := ctx.Value(fetchKindKey{}).(fetchKind)
	return kind
}

func (c *Client) newHTTP() *http.Client {
	dialer := &net.Dialer{Timeout: 5 * time.Second}
	transport := &http.Transport{
		Proxy: nil,
		DialContext: func(ctx context.Context, network, addr string) (net.Conn, error) {
			return c.dial(ctx, dialer, network, addr)
		},
		ForceAttemptHTTP2:     true,
		MaxIdleConns:          8,
		IdleConnTimeout:       30 * time.Second,
		TLSHandshakeTimeout:   5 * time.Second,
		ResponseHeaderTimeout: c.timeout,
		ExpectContinueTimeout: time.Second,
	}
	return &http.Client{
		Timeout:   c.timeout,
		Transport: transport,
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= 3 {
				return fmt.Errorf("重定向过多")
			}
			ctx := req.Context()
			if kindFrom(ctx) == 0 && len(via) > 0 {
				ctx = via[0].Context()
			}
			return c.allow(ctx, req.URL)
		},
	}
}

func (c *Client) dial(ctx context.Context, dialer *net.Dialer, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if kindFrom(ctx) == fetchPage {
		if !c.pageHostOK(host) {
			return nil, fmt.Errorf("地址不可访问")
		}
	} else if !c.hostAllowed(host) {
		return nil, fmt.Errorf("地址不可访问")
	}
	ips, err := net.DefaultResolver.LookupIPAddr(ctx, host)
	if err != nil {
		return nil, fmt.Errorf("地址无法解析")
	}
	var last error
	for _, ip := range ips {
		if blockedIP(ip.IP, c.permitLoopback) {
			last = fmt.Errorf("地址不可访问")
			continue
		}
		conn, err := dialer.DialContext(ctx, network, net.JoinHostPort(ip.IP.String(), port))
		if err == nil {
			return conn, nil
		}
		last = err
	}
	if last == nil {
		last = fmt.Errorf("地址不可访问")
	}
	return nil, last
}

func (c *Client) allow(ctx context.Context, target *url.URL) error {
	if target == nil || target.Host == "" || target.User != nil {
		return fmt.Errorf("地址不可访问")
	}
	if kindFrom(ctx) == fetchPage {
		if target.Scheme != "http" && target.Scheme != "https" {
			return fmt.Errorf("地址不可访问")
		}
		if !c.pageHostOK(target.Hostname()) {
			return fmt.Errorf("地址不可访问")
		}
		return nil
	}
	if target.Scheme != "https" && !(c.permitLoopback && target.Scheme == "http") {
		return fmt.Errorf("地址不可访问")
	}
	if !c.hostAllowed(target.Hostname()) {
		return fmt.Errorf("地址不可访问")
	}
	return nil
}

func (c *Client) pageHostOK(host string) bool {
	host = strings.ToLower(strings.Trim(strings.TrimSpace(host), "[]"))
	if host == "" || strings.ContainsAny(host, " \t") {
		return false
	}
	if host == "localhost" || strings.HasSuffix(host, ".localhost") || strings.HasSuffix(host, ".local") || strings.HasSuffix(host, ".internal") {
		return c.permitLoopback && host == "localhost"
	}
	if ip := net.ParseIP(host); ip != nil {
		return !blockedIP(ip, c.permitLoopback)
	}
	return true
}

func (c *Client) hostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	switch host {
	case "html.duckduckgo.com", "lite.duckduckgo.com", "api.duckduckgo.com", "zh.wikipedia.org", "en.wikipedia.org", "api.bing.microsoft.com", "api.tavily.com":
		return true
	}
	if c.permitLoopback && (host == "127.0.0.1" || host == "localhost" || host == "::1") {
		return true
	}
	return false
}

func blockedIP(ip net.IP, permitLoopback bool) bool {
	if ip == nil {
		return true
	}
	if ip.IsLoopback() {
		return !permitLoopback
	}
	if ip.IsPrivate() || ip.IsUnspecified() || ip.IsMulticast() || ip.IsLinkLocalUnicast() || ip.IsLinkLocalMulticast() {
		return true
	}
	v4 := ip.To4()
	if v4 == nil {
		return false
	}
	if v4[0] == 0 {
		return true
	}
	if v4[0] == 100 && v4[1]&0xc0 == 64 {
		return true
	}
	if v4[0] == 198 && (v4[1] == 18 || v4[1] == 19) {
		return true
	}
	return false
}

func normalizeQuery(query string) string {
	query = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return ' '
		}
		return r
	}, query)
	return clip(strings.Join(strings.Fields(query), " "), 200)
}

func hasHan(text string) bool {
	for _, r := range text {
		if unicode.Is(unicode.Han, r) {
			return true
		}
	}
	return false
}
