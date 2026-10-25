package search

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type engineJob struct {
	run func(context.Context, string) ([]Hit, error)
}

// indexBudget 给网页索引一段较短的时间。中文搜索引擎同时进行，索引卡住时不会占满整段检索。
func indexBudget(ctx context.Context) time.Duration {
	const cap = 3500 * time.Millisecond
	deadline, ok := ctx.Deadline()
	if !ok {
		return cap
	}
	remain := time.Until(deadline)
	if remain <= 0 {
		return time.Millisecond
	}
	if remain > 7*time.Second {
		return cap
	}
	budget := remain * 45 / 100
	if budget < 800*time.Millisecond {
		return remain
	}
	return budget
}

func (c *Client) engineJobs() []engineJob {
	if c == nil {
		return nil
	}
	var jobs []engineJob
	if strings.TrimSpace(c.bingHTML) != "" {
		jobs = append(jobs, engineJob{run: c.bingHTMLSearch})
	}
	if strings.TrimSpace(c.baiduURL) != "" {
		jobs = append(jobs, engineJob{run: c.baiduSearch})
	}
	if strings.TrimSpace(c.sogouURL) != "" {
		jobs = append(jobs, engineJob{run: c.sogouSearch})
	}
	if strings.TrimSpace(c.soURL) != "" {
		jobs = append(jobs, engineJob{run: c.soSearch})
	}
	if strings.TrimSpace(c.toutiaoURL) != "" {
		jobs = append(jobs, engineJob{run: c.toutiaoSearch})
	}
	return jobs
}

func (c *Client) searchEngines(ctx context.Context, query string) ([]Hit, error) {
	jobs := c.engineJobs()
	if len(jobs) == 0 || ctx.Err() != nil {
		return nil, ctx.Err()
	}
	found := make([][]Hit, len(jobs))
	errs := make([]error, len(jobs))
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()
	var mu sync.Mutex
	var group sync.WaitGroup
	for i, job := range jobs {
		group.Add(1)
		go func(i int, job engineJob) {
			defer group.Done()
			hits, err := job.run(ctx, query)
			mu.Lock()
			found[i] = hits
			errs[i] = err
			enough := len(mergeHits(found...)) >= 5
			mu.Unlock()
			if enough {
				cancel()
			}
		}(i, job)
	}
	group.Wait()
	hits := mergeHits(found...)
	if len(hits) > 0 {
		return hits, nil
	}
	for _, err := range errs {
		if err != nil && !errors.Is(err, context.Canceled) {
			return nil, err
		}
	}
	if err := ctx.Err(); err != nil && !errors.Is(err, context.Canceled) {
		return nil, err
	}
	return nil, nil
}

func (c *Client) bingHTMLSearch(ctx context.Context, query string) ([]Hit, error) {
	body, err := c.engineGet(ctx, c.bingHTML, url.Values{
		"q":        {query},
		"setlang":  {"zh-Hans"},
		"ensearch": {"0"},
	})
	if err != nil {
		return nil, err
	}
	return parseBingHTML(body), nil
}

func (c *Client) baiduSearch(ctx context.Context, query string) ([]Hit, error) {
	body, err := c.engineGet(ctx, c.baiduURL, url.Values{"word": {query}})
	if err != nil {
		return nil, err
	}
	return parseBaiduHTML(body), nil
}

func (c *Client) sogouSearch(ctx context.Context, query string) ([]Hit, error) {
	body, err := c.engineGet(ctx, c.sogouURL, url.Values{
		"query": {query},
		"ie":    {"utf8"},
	})
	if err != nil {
		return nil, err
	}
	return parseSogouHTML(body), nil
}

func (c *Client) soSearch(ctx context.Context, query string) ([]Hit, error) {
	body, err := c.engineGet(ctx, c.soURL, url.Values{"q": {query}})
	if err != nil {
		return nil, err
	}
	return parseSoHTML(body), nil
}

func (c *Client) toutiaoSearch(ctx context.Context, query string) ([]Hit, error) {
	body, err := c.engineGet(ctx, c.toutiaoURL, url.Values{"keyword": {query}})
	if err != nil {
		return nil, err
	}
	return parseToutiaoHTML(body), nil
}

func (c *Client) engineGet(ctx context.Context, endpoint string, values url.Values) (string, error) {
	parsed, err := url.Parse(endpoint)
	if err != nil {
		return "", err
	}
	query := parsed.Query()
	for key, list := range values {
		for _, value := range list {
			query.Set(key, value)
		}
	}
	parsed.RawQuery = query.Encode()
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, parsed.String(), nil)
	if err != nil {
		return "", err
	}
	raw, _, err := c.roundTrip(req, fetchHTML, 4<<20)
	if err != nil {
		return "", err
	}
	body := string(raw)
	if engineChallenge(body) {
		return "", fmt.Errorf("检索服务要求验证")
	}
	return body, nil
}

func engineChallenge(body string) bool {
	if strings.Contains(body, `"mu":"`) || strings.Contains(body, "b_algo") || strings.Contains(body, "vr-title") || strings.Contains(body, "data-pcurl") || strings.Contains(body, `"summary_text"`) {
		return false
	}
	return strings.Contains(body, "百度安全验证") || strings.Contains(body, "wappass.baidu.com")
}
