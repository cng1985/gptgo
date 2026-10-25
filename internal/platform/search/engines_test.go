package search

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

func TestParseSearchEngines(t *testing.T) {
	bing := parseBingHTML(`
<li class="b_algo"><h2><a href="https://www.tianqi.com/shanghai/today/">上海<strong>今天天气</strong></a></h2>
<div class="b_caption"><p class="b_lineclamp2">今日晴，最高 26 度。</p></div></li>
<li class="b_algo"><h2><a href="https://www.bing.com/search?q=weather">必应自己</a></h2></li>
<li class="b_algo"><h2><a href="http://10.0.0.8/secret">内网</a></h2></li>`)
	if len(bing) != 1 || bing[0].Engine != "必应" || bing[0].URL != "https://www.tianqi.com/shanghai/today/" || bing[0].Title != "上海今天天气" || bing[0].Snippet != "今日晴，最高 26 度。" {
		t.Fatalf("必应页面: %#v", bing)
	}

	baidu := parseBaiduHTML(`<div data-log='{"mu":"https://www.tianqi.com/shanghai/"}'><!--s-text-->【<em>上海天气</em>预报】<!--/s-text-->{"text":"天气网提供<em>上海</em>今日晴，最高 26 度。"}</div>
<div data-log='{"mu":"http://34689.recommend_list.baidu.com"}'><!--s-text-->推荐<!--/s-text--></div>
<div data-log='{"mu":"http://127.0.0.1/secret"}'><!--s-text-->内网<!--/s-text--></div>`)
	if len(baidu) != 1 || baidu[0].Engine != "百度" || baidu[0].URL != "https://www.tianqi.com/shanghai/" || !strings.Contains(baidu[0].Title, "上海天气预报") || !strings.Contains(baidu[0].Snippet, "26 度") {
		t.Fatalf("百度页面: %#v", baidu)
	}
	if !engineChallenge(`<title>百度安全验证</title>`) || engineChallenge(baiduPageWithMu()) {
		t.Fatal("验证页和正常结果页应该能分开")
	}

	sogou := parseSogouHTML(`<h3 class="vr-title "><a href="/link?url=abc" linkurl=http://sh.weather.com.cn/ ><em><!--red_beg-->上海天气<!--red_end--></em></a></h3><p class="star-wiki text-default">中国天气网，今日晴。</p>
<h3 class="vr-title"><a href="https://weatherol.cn/shanghai">上海一周天气</a></h3><p class="fz-mid space-txt">最高 26 度。</p>
<h3 class="vr-title"><a href="/link?url=only-redirect">没有真实地址</a></h3>`)
	if len(sogou) != 2 || sogou[0].Engine != "搜狗" || sogou[0].URL != "http://sh.weather.com.cn/" || sogou[0].Title != "上海天气" || sogou[1].URL != "https://weatherol.cn/shanghai" || sogou[1].Snippet != "最高 26 度。" {
		t.Fatalf("搜狗页面: %#v", sogou)
	}
	weixin, ok := acceptEngineHit("上海今日天气", "http://mp.weixin.qq.com/s?src=11&timestamp=179&ver=1", "公众号天气")
	if !ok || !strings.Contains(weixin.URL, "&timestamp=") || strings.Contains(weixin.URL, "×") {
		t.Fatalf("查询串里的 timestamp 不应被当成 HTML 实体: %#v", weixin)
	}
	amp, ok := acceptEngineHit("上海天气", "https://example.com/a?src=11&amp;timestamp=2", "编码过的链接")
	if !ok || amp.URL != "https://example.com/a?src=11&timestamp=2" {
		t.Fatalf("&amp; 应该还原成查询参数: %#v", amp)
	}

	so := parseSoHTML(`<div class="g-card res-list" data-pcurl="https://m.tianqi.com/shanghai/today/"><a href="https://m.so.com/jump?u=https%3A%2F%2Fm.tianqi.com%2Fshanghai%2Ftoday%2F"><h3 class="res-title"><em>上海今天天气</em>预报</h3><p class="g-main summary">今日晴，最高 26 度。</p></a></div>
<div data-pcurl="https://m.so.com/s?q=weather"><h3 class="res-title">还是搜索页</h3></div>`)
	if len(so) != 1 || so[0].Engine != "360搜索" || so[0].URL != "https://m.tianqi.com/shanghai/today/" || so[0].Title != "上海今天天气预报" || !strings.Contains(so[0].Snippet, "26 度") {
		t.Fatalf("360 页面: %#v", so)
	}

	toutiao := parseToutiaoHTML(`{"title":"\u003cem\u003e上海今天天气\u003c/em\u003e","summary_text":"今日阴转多云，北风 4 级。","url":"http:\/\/m.toutiao.com\/group\/7700\/"}
{"title":"上海天气配图","summary_text":"配图","url":"https://p3-sign.toutiaoimg.com/tos/a.jpg"}
{"title":"资讯","url":"https://so.toutiao.com/search?keyword=weather"}
{"title":"热","url":"https://example.com/short"}`)
	if len(toutiao) != 1 || toutiao[0].Engine != "头条搜索" || toutiao[0].URL != "http://m.toutiao.com/group/7700/" || toutiao[0].Title != "上海今天天气" || !strings.Contains(toutiao[0].Snippet, "北风") {
		t.Fatalf("头条页面: %#v", toutiao)
	}
}

func baiduPageWithMu() string {
	return `<div data-log='{"mu":"https://example.com/a"}'><!--s-text-->标题<!--/s-text--></div>`
}

func TestEnginesFillEmptyIndexAndStayIdleOtherwise(t *testing.T) {
	var mu sync.Mutex
	var engineCalls int
	var sawBingLang bool
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html/":
			if r.URL.Query().Get("q") == "已有结果" || r.URL.Query().Get("q") == "golang" {
				io.WriteString(w, `<a class="result__a" href="https://example.com/go">Go</a><a class="result__snippet">编程语言</a>`)
				return
			}
			io.WriteString(w, `<html></html>`)
		case "/lite/":
			http.NotFound(w, r)
		case "/wiki":
			io.WriteString(w, `{"query":{"pages":[{"title":"维基","index":1,"extract":"百科","fullurl":"https://zh.wikipedia.org/wiki/Go"}]}}`)
		case "/json":
			io.WriteString(w, `{"Heading":"即时","AbstractURL":"https://example.com/instant","AbstractText":"即时答案"}`)
		case "/binghtml":
			mu.Lock()
			engineCalls++
			sawBingLang = r.URL.Query().Get("setlang") == "zh-Hans" && r.URL.Query().Get("q") != ""
			mu.Unlock()
			io.WriteString(w, `<h2><a href="https://example.com/bing">必应天气</a></h2><p class="b_lineclamp2">来自必应</p>`)
		case "/baidu":
			mu.Lock()
			engineCalls++
			mu.Unlock()
			if r.URL.Query().Get("word") == "" {
				http.Error(w, "missing word", http.StatusBadRequest)
				return
			}
			io.WriteString(w, `<title>百度安全验证</title>`)
		case "/sogou":
			mu.Lock()
			engineCalls++
			mu.Unlock()
			io.WriteString(w, `<h3 class="vr-title"><a href="https://example.com/bing">重复</a></h3><p class="space-txt">同一条</p><h3 class="vr-title"><a linkurl=https://example.com/sogou href="/link?url=x">搜狗天气</a></h3><p class="space-txt">来自搜狗</p>`)
		case "/so":
			mu.Lock()
			engineCalls++
			mu.Unlock()
			io.WriteString(w, `<div data-pcurl="https://example.com/so"><h3 class="res-title">360天气</h3><p class="summary">来自360</p></div>`)
		case "/toutiao":
			mu.Lock()
			engineCalls++
			mu.Unlock()
			io.WriteString(w, `{"title":"头条天气","summary_text":"来自头条","url":"https://example.com/toutiao"}`)
		case "/bing":
			io.WriteString(w, `{}`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := testClient(server.URL)
	pointEngines(client, server.URL)
	result, err := client.Search(context.Background(), "上海今天天气")
	if err != nil {
		t.Fatal(err)
	}
	mu.Lock()
	bingLang := sawBingLang
	mu.Unlock()
	if !bingLang {
		t.Fatal("必应页面检索应该带上中文参数")
	}
	want := []string{
		"https://example.com/bing",
		"https://example.com/sogou",
		"https://example.com/so",
		"https://example.com/toutiao",
		"https://zh.wikipedia.org/wiki/Go",
	}
	if len(result.Hits) != len(want) {
		t.Fatalf("回退结果: %#v", result.Hits)
	}
	for i, url := range want {
		if result.Hits[i].URL != url {
			t.Fatalf("第 %d 条应该是 %s，实际 %#v", i+1, url, result.Hits)
		}
	}
	if result.Hits[0].Engine != "必应" || result.Hits[4].Engine != "维基百科" || !strings.Contains(result.Notes, "必应天气") {
		t.Fatalf("来源标记: %#v", result.Hits)
	}

	before := engineCalls
	english, err := client.Search(context.Background(), "golang")
	if err != nil {
		t.Fatal(err)
	}
	if english.Hits[0].URL != "https://example.com/go" || english.Hits[0].Engine != "DuckDuckGo" {
		t.Fatalf("英文问题应优先保留网页索引: %#v", english.Hits)
	}
	if !containsURL(english.Hits, "https://example.com/bing") {
		t.Fatalf("英文索引有结果时仍应并入搜索引擎: %#v", english.Hits)
	}
	if engineCalls <= before {
		t.Fatalf("搜索引擎应该和索引同时查询: %d -> %d", before, engineCalls)
	}

	chinese, err := client.Search(context.Background(), "已有结果")
	if err != nil {
		t.Fatal(err)
	}
	if chinese.Hits[0].URL != "https://example.com/bing" || chinese.Hits[0].Engine != "必应" {
		t.Fatalf("中文问题应优先采用搜索引擎: %#v", chinese.Hits)
	}
	if !containsURL(chinese.Hits, "https://example.com/go") {
		t.Fatalf("中文问题仍应保留网页索引: %#v", chinese.Hits)
	}

	client.provider = "bing"
	client.apiKey = "bing-secret"
	client.bingURL = server.URL + "/bing"
	fallback, err := client.Search(context.Background(), "接口为空")
	if err != nil || fallback.Hits[0].URL != "https://example.com/bing" {
		t.Fatalf("必应接口没有结果时应该回退到网页搜索: %v %#v", err, fallback.Hits)
	}
}

func TestEnginesRunWhileIndexHangs(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/html/", "/lite/":
			<-r.Context().Done()
		case "/wiki", "/json":
			io.WriteString(w, `{}`)
		case "/binghtml":
			io.WriteString(w, `<h2><a href="https://example.com/fast">及时结果</a></h2><p class="b_lineclamp2">没有干等索引</p>`)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()

	client := testClient(server.URL)
	client.bingHTML = server.URL + "/binghtml"
	started := time.Now()
	result, err := client.Search(context.Background(), "上海今天天气")
	elapsed := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Hits) != 1 || result.Hits[0].URL != "https://example.com/fast" {
		t.Fatalf("索引超时应改用搜索引擎: %#v", result.Hits)
	}
	if elapsed > 2500*time.Millisecond {
		t.Fatalf("索引卡住时不应该占满整个检索时间，用时 %s", elapsed)
	}
}

func TestEngineHostsAreAllowed(t *testing.T) {
	client := New(nil)
	for _, raw := range []string{
		"https://cn.bing.com/search?q=a",
		"https://www.bing.com/search?q=a",
		"https://m.baidu.com/s?word=a",
		"https://www.baidu.com/s?wd=a",
		"https://www.sogou.com/web?query=a",
		"https://m.so.com/s?q=a",
		"https://so.toutiao.com/search?keyword=a",
	} {
		parsed, err := http.NewRequest(http.MethodGet, raw, nil)
		if err != nil {
			t.Fatal(err)
		}
		if err := client.allow(context.Background(), parsed.URL); err != nil {
			t.Fatalf("%s 应该允许: %v", raw, err)
		}
	}
	blocked, err := http.NewRequest(http.MethodGet, "https://wappass.baidu.com/captcha", nil)
	if err != nil {
		t.Fatal(err)
	}
	if err := client.allow(context.Background(), blocked.URL); err == nil {
		t.Fatal("验证码域名不应该放进检索白名单")
	}
}

func containsURL(hits []Hit, raw string) bool {
	for _, hit := range hits {
		if hit.URL == raw {
			return true
		}
	}
	return false
}

func pointEngines(client *Client, base string) {
	client.bingHTML = base + "/binghtml"
	client.baiduURL = base + "/baidu"
	client.sogouURL = base + "/sogou"
	client.soURL = base + "/so"
	client.toutiaoURL = base + "/toutiao"
}
