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
}

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
	duckAPI        string
	wikiZH         string
	wikiEN         string
	bingURL        string
	tavilyURL      string
}

func New(cfg *config.Config) *Client {
	client := &Client{
		provider:  "duckduckgo",
		timeout:   8 * time.Second,
		duckURL:   "https://html.duckduckgo.com/html/",
		duckAPI:   "https://api.duckduckgo.com/",
		wikiZH:    "https://zh.wikipedia.org/w/api.php",
		wikiEN:    "https://en.wikipedia.org/w/api.php",
		bingURL:   "https://api.bing.microsoft.com/v7.0/search",
		tavilyURL: "https://api.tavily.com/search",
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
	ctx, cancel := context.WithTimeout(ctx, c.timeout)
	defer cancel()
	switch c.provider {
	case "off":
		return Result{}, fmt.Errorf("联网检索已关闭")
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
		htmlHits, htmlErr = c.duckHTML(ctx, query)
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

func (c *Client) duckHTML(ctx context.Context, query string) ([]Hit, error) {
	form := url.Values{}
	form.Set("q", query)
	form.Set("kl", "cn-zh")
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.duckURL, strings.NewReader(form.Encode()))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	raw, err := c.do(req)
	if err != nil {
		return nil, err
	}
	return parseDuckHTML(string(raw)), nil
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
	if err := c.allow(req.URL); err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "ChatBI/1.0 (local research assistant)")
	req.Header.Set("Accept", "text/html,application/json;q=0.9,*/*;q=0.8")
	resp, err := c.http.Do(req)
	if err != nil {
		if req.Context().Err() != nil {
			return nil, req.Context().Err()
		}
		return nil, fmt.Errorf("检索请求失败")
	}
	defer resp.Body.Close()
	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("检索请求失败")
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, fmt.Errorf("检索服务返回 %d", resp.StatusCode)
	}
	return raw, nil
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
			return c.allow(req.URL)
		},
	}
}

func (c *Client) dial(ctx context.Context, dialer *net.Dialer, network, addr string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return nil, err
	}
	if !c.hostAllowed(host) {
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

func (c *Client) allow(target *url.URL) error {
	if target == nil || target.Host == "" || target.User != nil {
		return fmt.Errorf("地址不可访问")
	}
	if target.Scheme != "https" && !(c.permitLoopback && target.Scheme == "http") {
		return fmt.Errorf("地址不可访问")
	}
	if !c.hostAllowed(target.Hostname()) {
		return fmt.Errorf("地址不可访问")
	}
	return nil
}

func (c *Client) hostAllowed(host string) bool {
	host = strings.ToLower(strings.TrimSpace(host))
	switch host {
	case "html.duckduckgo.com", "api.duckduckgo.com", "zh.wikipedia.org", "en.wikipedia.org", "api.bing.microsoft.com", "api.tavily.com":
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
