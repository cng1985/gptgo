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

	"golang.org/x/text/encoding/simplifiedchinese"
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
	client.liteURL = "https://evil.example/lite/"
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

func TestParseLiteDuckHTML(t *testing.T) {
	body := `<a rel="nofollow" href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fwww.weather.com.cn%2Fweather%2F101020100.shtml&amp;rut=abc" class='result-link'>上海天气预报</a>
<td class='result-snippet'>今日晴，最高 26 度。</td>`
	hits := parseDuckHTML(body)
	if len(hits) != 1 || hits[0].Title != "上海天气预报" || hits[0].URL != "https://www.weather.com.cn/weather/101020100.shtml" || hits[0].Snippet != "今日晴，最高 26 度。" {
		t.Fatalf("Lite 结果: %#v", hits)
	}
}

func TestWebIndexFallsBackToLite(t *testing.T) {
	var sawMozilla bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html/":
			sawMozilla = strings.Contains(r.Header.Get("User-Agent"), "Mozilla/")
			w.WriteHeader(http.StatusAccepted)
			io.WriteString(w, `<div class="anomaly-modal__title">验证</div>`)
		case "/lite/":
			if !strings.Contains(r.Header.Get("User-Agent"), "Mozilla/") {
				http.Error(w, "missing browser", http.StatusForbidden)
				return
			}
			io.WriteString(w, `<a href="//duckduckgo.com/l/?uddg=https%3A%2F%2Fexample.com%2Fweather" class='result-link'>上海天气</a><td class='result-snippet'>今日晴，最高 26 度</td>`)
		case "/wiki", "/json":
			io.WriteString(w, `{"query":{"pages":[]}}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := testClient(server.URL)
	result, err := client.Search(context.Background(), "上海今天天气")
	if err != nil {
		t.Fatal(err)
	}
	if !sawMozilla {
		t.Fatal("网页检索应该使用浏览器标识，否则会被验证页拦住")
	}
	if len(result.Hits) != 1 || result.Hits[0].URL != "https://example.com/weather" || !strings.Contains(result.Notes, "26 度") {
		t.Fatalf("Lite 回退: %#v", result)
	}
}

func TestReadPageKeepsExcerptAndBlocksPrivate(t *testing.T) {
	page := `<html><header><a>首页</a></header><style>.a{color:red}</style><script>var secret = "不要出现"</script><p>今日上海晴，最高 26 度，最低 18 度，东北风 3 级。空气质量为良，适合户外活动，傍晚会转多云。</p></html>`
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/page":
			w.Header().Set("Content-Type", "text/html; charset=utf-8")
			io.WriteString(w, page)
		case "/go":
			http.Redirect(w, r, "http://10.1.2.3/secret", http.StatusFound)
		case "/bin":
			w.Header().Set("Content-Type", "application/octet-stream")
			io.WriteString(w, "binary")
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := testClient(server.URL)
	text, err := client.readPage(context.Background(), server.URL+"/page")
	if err != nil || !strings.Contains(text, "26 度") || strings.Contains(text, "不要出现") || strings.Contains(text, "首页") {
		t.Fatalf("摘录: %v %q", err, text)
	}
	hits := []Hit{{Title: "上海天气", URL: server.URL + "/page", Snippet: "短"}}
	client.fetchPages = true
	client.enrich(context.Background(), "上海今天天气", hits)
	if !strings.Contains(hits[0].Excerpt, "26 度") || hits[0].Snippet != "短" {
		t.Fatalf("阅读后应保留原摘要并写入摘录: %#v", hits[0])
	}
	if _, err := client.readPage(context.Background(), server.URL+"/go"); err == nil {
		t.Fatal("跳转到内网应该失败")
	}
	if _, err := client.readPage(context.Background(), "http://10.1.2.3/secret"); err == nil {
		t.Fatal("内网地址应该失败")
	}
	if _, err := client.readPage(context.Background(), server.URL+"/bin"); err == nil {
		t.Fatal("非网页内容应该失败")
	}
	blocked := testClient(server.URL)
	blocked.permitLoopback = false
	blocked.http = blocked.newHTTP()
	if _, err := blocked.readPage(context.Background(), server.URL+"/page"); err == nil {
		t.Fatal("未允许回环时不应读取测试页")
	}
}

func TestReadDecodesGBKAndRejectsPrivate(t *testing.T) {
	page := `<html><meta charset="gbk"><title>上海天气</title><p>今日上海晴，最高 26 度，最低 18 度，东北风 3 级。空气质量为良，适合户外活动，傍晚会转多云。</p></html>`
	encoded, err := simplifiedchinese.GBK.NewEncoder().String(page)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=gbk")
		io.WriteString(w, encoded)
	}))
	defer server.Close()

	client := testClient(server.URL)
	text, title, err := client.readDocument(context.Background(), server.URL+"/weather", detailExcerpt)
	if err != nil {
		t.Fatal(err)
	}
	if title != "上海天气" || !strings.Contains(text, "26 度") {
		t.Fatalf("明细: title=%q text=%q", title, text)
	}
	if _, err := client.Read(context.Background(), "http://10.1.2.3/secret"); err == nil {
		t.Fatal("明细接口不应读取内网")
	}
	if _, err := client.Read(context.Background(), "javascript:alert(1)"); err == nil {
		t.Fatal("明细接口只接受网页地址")
	}
}

func TestReadableTextDropsFooterAndKeepsForecast(t *testing.T) {
	raw := "<p>当前位置：首页 > 上海市气象局 > 气象服务 > 今日天气</p><p>主办:上海市气象局办公室 版权所有：上海市气象局 ICP备案号：京ICP备05004897号</p><p>今日上海晴，最高 26 度，最低 18 度，东北风 3 级。空气质量为良，适合户外活动。</p>"
	got := readableText(raw, detailExcerpt)
	if strings.Contains(got, "版权") || strings.Contains(got, "ICP") || strings.Contains(got, "当前位置") || !strings.Contains(got, "26 度") {
		t.Fatalf("正文应去掉页脚并保留预报: %q", got)
	}
}

func TestFocusExcerptStopsAtUnrelatedLines(t *testing.T) {
	text := "上海今天晴，最高 26 度，最低 18 度，东北风 3 级。\n如何破解wifi密码 wifi密码怎么简单破解\n宫保鸡丁做法家常简单做法"
	got := focusExcerpt("上海今天天气", "上海天气预报", text)
	if !strings.Contains(got, "26 度") || strings.Contains(got, "wifi") || strings.Contains(got, "宫保鸡丁") {
		t.Fatalf("摘录混入了无关推荐: %q", got)
	}
}

func testClient(base string) *Client {
	client := &Client{
		provider:       "duckduckgo",
		timeout:        3 * time.Second,
		permitLoopback: true,
		fetchPages:     false,
		duckURL:        base + "/html/",
		liteURL:        base + "/lite/",
		duckAPI:        base + "/json",
		wikiZH:         base + "/wiki",
		wikiEN:         base + "/wiki",
		bingURL:        base + "/bing",
		tavilyURL:      base + "/tavily",
	}
	client.http = client.newHTTP()
	return client
}
