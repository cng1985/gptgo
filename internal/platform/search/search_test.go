package search

import (
	"context"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestParseDuckHTMLKeepsPublicLinks(t *testing.T) {
	body := `
<a class="result__a" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fgo&amp;rut=abc">Go <b>语言</b></a>
<a class="result__snippet" href="https://example.com/go">Go 是一门编程语言。</a>
<a class="result__a" href="javascript:alert(1)">坏链接</a>
<a class="result__snippet">忽略</a>
<a class="result__a" href="https://duckduckgo.com/l/?uddg=http%3A%2F%2F127.0.0.1%2Fsecret">内网</a>
<a class="result__snippet">不该出现</a>
<a class="result__a" href="https://zh.wikipedia.org/wiki/Go">维基</a>
<a class="result__snippet">维基摘要</a>`
	hits := parseDuckHTML(body)
	if len(hits) != 2 {
		t.Fatalf("结果数量: %#v", hits)
	}
	if hits[0].Title != "Go 语言" || hits[0].URL != "https://example.com/go" || hits[0].Snippet != "Go 是一门编程语言。" {
		t.Fatalf("第一条: %#v", hits[0])
	}
	if hits[1].URL != "https://zh.wikipedia.org/wiki/Go" {
		t.Fatalf("第二条: %#v", hits[1])
	}
}

func TestParseStructuredFeeds(t *testing.T) {
	wiki := parseWiki([]byte(`{"query":{"pages":[{"title":"后","index":2,"extract":"后文","fullurl":"https://zh.wikipedia.org/wiki/B"},{"title":"先","index":1,"extract":"前文","fullurl":"https://zh.wikipedia.org/wiki/A"}]}}`))
	if len(wiki) != 2 || wiki[0].Title != "先" || wiki[1].Title != "后" {
		t.Fatalf("维基排序: %#v", wiki)
	}
	bing := parseBing([]byte(`{"webPages":{"value":[{"name":"必应","url":"https://example.com/bing","snippet":"摘要"}]}}`))
	if len(bing) != 1 || bing[0].URL != "https://example.com/bing" {
		t.Fatalf("必应: %#v", bing)
	}
	tavily := parseTavily([]byte(`{"results":[{"title":"Tavily","url":"https://example.com/tavily","content":"内容"}]}`))
	if len(tavily) != 1 || tavily[0].Snippet != "内容" {
		t.Fatalf("tavily: %#v", tavily)
	}
	instant := parseDuckJSON([]byte(`{"Heading":"标题","AbstractText":"简介","AbstractURL":"https://example.com/abs","RelatedTopics":[{"Topics":[{"Text":"相关 - 说明","FirstURL":"https://example.com/rel"}]}]}`))
	if len(instant) != 2 || instant[1].Title != "相关" || instant[1].Snippet != "说明" {
		t.Fatalf("即时答案: %#v", instant)
	}
}

func TestBlockedAddresses(t *testing.T) {
	if !blockedIP(net.ParseIP("127.0.0.1"), false) || !blockedIP(net.ParseIP("10.1.2.3"), false) || !blockedIP(net.ParseIP("169.254.169.254"), false) {
		t.Fatal("私网和回环地址应该被拒绝")
	}
	if blockedIP(net.ParseIP("127.0.0.1"), true) {
		t.Fatal("测试里可以访问回环地址")
	}
	if blockedIP(net.ParseIP("8.8.8.8"), false) {
		t.Fatal("公网地址不应被拒绝")
	}
	if _, ok := cleanURL("http://10.0.0.8/hidden"); ok {
		t.Fatal("结果链接不应指向私网")
	}
	if _, ok := cleanURL("javascript:alert(1)"); ok {
		t.Fatal("结果链接必须是 http 或 https")
	}
}

func TestClientMergesSourcesAndStaysOnAllowlist(t *testing.T) {
	var jsonCalls int
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html/":
			if r.Method != http.MethodGet || r.URL.Query().Get("q") == "" {
				http.Error(w, "bad html search", http.StatusMethodNotAllowed)
				return
			}
			io.WriteString(w, `<a class="result__a" href="https://example.com/go">Go</a><a class="result__snippet">编程语言</a>`)
		case "/wiki":
			io.WriteString(w, `{"query":{"pages":[{"title":"维基","index":1,"extract":"百科","fullurl":"https://zh.wikipedia.org/wiki/Go"}]}}`)
		case "/json":
			jsonCalls++
			io.WriteString(w, `{"Heading":"不该调用","AbstractURL":"https://example.com/extra","AbstractText":"多余"}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := testClient(server.URL)
	result, err := client.Search(context.Background(), "Go 语言")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 2 || result.Hits[0].URL != "https://example.com/go" || !strings.Contains(result.Notes, "维基") {
		t.Fatalf("网页结果应该排在维基前面: %#v", result)
	}
	if jsonCalls != 0 {
		t.Fatalf("已经有两条结果时不应再请求即时答案: %d", jsonCalls)
	}

	client.duckURL = "https://evil.example/html/"
	client.duckAPI = "https://evil.example/json"
	client.wikiZH = "https://evil.example/wiki"
	client.wikiEN = "https://evil.example/wiki"
	if _, err := client.Search(context.Background(), "Go 语言"); err == nil {
		t.Fatal("非允许的检索地址应该失败")
	}
}

func TestClientFallsBackAndUsesConfiguredProvider(t *testing.T) {
	var bingKey string
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html/":
			if r.Method != http.MethodGet || r.URL.Query().Get("q") == "" {
				http.Error(w, "bad html search", http.StatusMethodNotAllowed)
				return
			}
			io.WriteString(w, `<html></html>`)
		case "/wiki":
			io.WriteString(w, `{"query":{"pages":[]}}`)
		case "/json":
			io.WriteString(w, `{"RelatedTopics":[{"Text":"回退 - 摘要","FirstURL":"https://example.com/fallback"}]}`)
		case "/bing":
			bingKey = r.Header.Get("Ocp-Apim-Subscription-Key")
			io.WriteString(w, `{"webPages":{"value":[{"name":"必应结果","url":"https://example.com/bing","snippet":"来自必应"}]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := testClient(server.URL)
	result, err := client.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 || result.Hits[0].URL != "https://example.com/fallback" {
		t.Fatalf("回退结果: %#v", result)
	}

	client.provider = "bing"
	client.apiKey = "bing-secret"
	client.bingURL = server.URL + "/bing"
	binged, err := client.Search(context.Background(), "天气")
	if err != nil {
		t.Fatal(err)
	}
	if bingKey != "bing-secret" || binged.Hits[0].Title != "必应结果" {
		t.Fatalf("必应请求: key=%s %#v", bingKey, binged)
	}

	client.provider = "off"
	if _, err := client.Search(context.Background(), "天气"); err == nil {
		t.Fatal("关闭检索时应该失败")
	}
}

func TestDuckHTMLRejectsChallengePage(t *testing.T) {
	accepted := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || r.URL.Query().Get("q") == "" {
			http.Error(w, "bad html search", http.StatusMethodNotAllowed)
			return
		}
		w.WriteHeader(http.StatusAccepted)
		io.WriteString(w, `<div class="anomaly-modal__title"></div><a class="result__a" href="https://example.com/trap">陷阱</a>`)
	}))
	defer accepted.Close()

	client := testClient(accepted.URL)
	hits, err := client.duckHTML(context.Background(), "天气")
	if err == nil || len(hits) != 0 {
		t.Fatalf("验证页不应被当成结果: %v %#v", err, hits)
	}

	challenge := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		io.WriteString(w, `<div class="anomaly-modal__title">验证</div>`)
	}))
	defer challenge.Close()
	client.duckURL = challenge.URL + "/html/"
	hits, err = client.duckHTML(context.Background(), "天气")
	if err == nil || len(hits) != 0 {
		t.Fatalf("200 验证页也不应被当成结果: %v %#v", err, hits)
	}
}

func testClient(base string) *Client {
	client := &Client{
		provider:       "duckduckgo",
		timeout:        3 * time.Second,
		permitLoopback: true,
		duckURL:        base + "/html/",
		duckAPI:        base + "/json",
		wikiZH:         base + "/wiki",
		wikiEN:         base + "/wiki",
		bingURL:        base + "/bing",
		tavilyURL:      base + "/tavily",
	}
	client.http = client.newHTTP()
	return client
}
