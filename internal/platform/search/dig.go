package search

import (
	"context"
	"html"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"
	"unicode/utf8"
)

const (
	digBatch    = 3
	digRounds   = 3
	digGoal     = 2
	digPageWait = 6 * time.Second
)

type pageDoc struct {
	text  string
	title string
	final string
}

// dig 在摘要答不上来时打开搜索结果，并顺着跳转和页内链接再读。
func (c *Client) dig(ctx context.Context, query string, hits []Hit) {
	if c == nil || !c.fetchPages || len(hits) == 0 || ctx.Err() != nil {
		return
	}
	if countAnswered(query, hits) >= digGoal {
		return
	}
	tried := map[int]struct{}{}
	for round := 0; round < digRounds && ctx.Err() == nil; round++ {
		if countAnswered(query, hits) >= digGoal {
			break
		}
		batch := nextUnread(query, hits, tried)
		if len(batch) == 0 {
			break
		}
		var group sync.WaitGroup
		for _, index := range batch {
			group.Add(1)
			go func(index int) {
				defer group.Done()
				pageCtx, cancel := context.WithTimeout(ctx, digPageWait)
				defer cancel()
				doc, err := c.browse(pageCtx, hits[index].URL, detailExcerpt, query)
				if err != nil && strings.TrimSpace(doc.text) == "" {
					return
				}
				title := strings.TrimSpace(hits[index].Title + " " + doc.title)
				focused := focusExcerpt(query, title, doc.text)
				if !answerText(query, title, focused) {
					return
				}
				hits[index].Excerpt = clip(focused, pageExcerpt)
				if doc.final != "" && doc.final != hits[index].URL {
					if abs, ok := c.acceptTarget(pageCtx, hits[index].URL, doc.final); ok {
						hits[index].URL = abs
					}
				}
			}(index)
		}
		group.Wait()
	}
	promoteReadHits(hits)
}

func nextUnread(query string, hits []Hit, tried map[int]struct{}) []int {
	type candidate struct {
		index int
		score int
	}
	list := make([]candidate, 0, len(hits))
	for i, hit := range hits {
		if _, ok := tried[i]; ok || strings.TrimSpace(hit.URL) == "" || strings.TrimSpace(hit.Excerpt) != "" {
			continue
		}
		score := 0
		if !concreteSnippet(query, hit.Title, hit.Snippet) {
			score += 3
		}
		if containsTerm(hit.Title, queryTerms(query, "")) {
			score += 2
		}
		host := strings.ToLower(hostOf(hit.URL))
		if strings.Contains(host, "weather") || strings.Contains(host, "tianqi") || strings.Contains(host, "cma.gov") {
			score += 4
		}
		list = append(list, candidate{index: i, score: score})
	}
	sort.SliceStable(list, func(i, j int) bool {
		return list[i].score > list[j].score
	})
	if len(list) > digBatch {
		list = list[:digBatch]
	}
	batch := make([]int, 0, len(list))
	for _, item := range list {
		tried[item.index] = struct{}{}
		batch = append(batch, item.index)
	}
	return batch
}

func countAnswered(query string, hits []Hit) int {
	n := 0
	for _, hit := range hits {
		if strings.TrimSpace(hit.Excerpt) != "" && answerText(query, hit.Title, hit.Excerpt) {
			n++
		}
	}
	return n
}

func hostOf(raw string) string {
	parsed, err := url.Parse(raw)
	if err != nil {
		return ""
	}
	return parsed.Hostname()
}

func concreteSnippet(query, title, text string) bool {
	text = strings.TrimSpace(text)
	if utf8.RuneCountInString(text) < 8 || !hasMeasure(text) {
		return false
	}
	terms := queryTerms(query, title)
	return containsTerm(text, terms) || containsTerm(title, terms)
}

func answerText(query, title, text string) bool {
	text = strings.TrimSpace(text)
	n := utf8.RuneCountInString(text)
	if n < 8 {
		return false
	}
	terms := queryTerms(query, title)
	overlap := containsTerm(text, terms)
	titled := containsTerm(title, terms)
	if hasMeasure(text) && (overlap || titled) && !chromeLine(text) {
		return true
	}
	if chromeLine(text) {
		return false
	}
	return overlap && n >= 80
}

func promoteReadHits(hits []Hit) {
	sort.SliceStable(hits, func(i, j int) bool {
		return excerptRank(hits[i]) > excerptRank(hits[j])
	})
}

func excerptRank(hit Hit) int {
	excerpt := strings.TrimSpace(hit.Excerpt)
	if excerpt == "" {
		return 0
	}
	if hasMeasure(excerpt) {
		return 2
	}
	return 1
}

// browse 读取一篇页面。正文太短时先跟脚本跳转，再按问题点开页内链接，最多再取三次。
func (c *Client) browse(ctx context.Context, rawURL string, limit int, query string) (pageDoc, error) {
	var terms []string
	if strings.TrimSpace(query) != "" {
		terms = queryTerms(query, "")
	}
	visited := map[string]struct{}{}
	current := strings.TrimSpace(rawURL)
	var last pageDoc
	var lastErr error
	for hop := 0; hop < 4; hop++ {
		if current == "" {
			break
		}
		if _, seen := visited[current]; seen {
			break
		}
		visited[current] = struct{}{}
		if err := ctx.Err(); err != nil {
			if strings.TrimSpace(last.text) != "" {
				return last, nil
			}
			return pageDoc{}, err
		}
		decoded, title, err := c.fetchHTML(ctx, current)
		if err != nil {
			lastErr = err
			break
		}
		text := readableText(decoded, limit)
		last = pageDoc{text: text, title: title, final: current}
		if pageSettled(query, title, text, terms == nil) {
			return last, nil
		}
		if utf8.RuneCountInString(strings.TrimSpace(text)) < 40 {
			if next := scriptOrMetaRedirect(decoded); next != "" {
				if abs, ok := c.acceptTarget(ctx, current, next); ok && abs != current {
					current = abs
					continue
				}
			}
		}
		if terms == nil {
			break
		}
		next := ""
		for _, link := range contentLinks(decoded, current, terms) {
			if _, seen := visited[link]; seen {
				continue
			}
			if abs, ok := c.acceptTarget(ctx, current, link); ok && abs != current {
				next = abs
				break
			}
		}
		if next == "" {
			break
		}
		current = next
	}
	if strings.TrimSpace(last.text) == "" && lastErr != nil {
		return pageDoc{}, lastErr
	}
	return last, nil
}

func pageSettled(query, title, text string, redirectOnly bool) bool {
	if redirectOnly {
		return utf8.RuneCountInString(strings.TrimSpace(text)) >= 40
	}
	return answerText(query, title, text)
}

func (c *Client) acceptTarget(ctx context.Context, base, next string) (string, bool) {
	abs := absolutize(base, html.UnescapeString(strings.TrimSpace(next)))
	if abs == "" {
		return "", false
	}
	parsed, err := url.Parse(abs)
	if err != nil || parsed.User != nil {
		return "", false
	}
	pageCtx := context.WithValue(ctx, fetchKindKey{}, fetchPage)
	if err := c.allow(pageCtx, parsed); err != nil {
		return "", false
	}
	if skipEngineResult(parsed.String()) {
		return "", false
	}
	return parsed.String(), true
}

func scriptOrMetaRedirect(body string) string {
	head := body
	if len(head) > 12000 {
		head = head[:12000]
	}
	if match := jsRedirect.FindStringSubmatch(head); len(match) > 1 {
		return html.UnescapeString(match[1])
	}
	lower := strings.ToLower(head)
	if strings.Contains(lower, "http-equiv") && strings.Contains(lower, "refresh") {
		if match := metaRedirect.FindStringSubmatch(head); len(match) > 1 {
			target := html.UnescapeString(match[1])
			if absoluteHTTP(target) || strings.HasPrefix(target, "/") {
				return target
			}
		}
	}
	return ""
}

func contentLinks(body, base string, terms []string) []string {
	matches := anchorTag.FindAllStringSubmatch(body, 50)
	type candidate struct {
		link  string
		score int
	}
	var found []candidate
	seen := map[string]struct{}{}
	for _, match := range matches {
		if len(match) < 3 {
			continue
		}
		text := stripTags(match[2])
		if utf8.RuneCountInString(text) < 4 || noiseAnchor(text) {
			continue
		}
		score := 0
		if containsTerm(text, terms) {
			score += 2
		}
		if hasMeasure(text) {
			score += 1
		}
		if score == 0 {
			continue
		}
		link := absolutize(base, attrLoose(match[1], "href"))
		if link == "" {
			continue
		}
		if _, ok := seen[link]; ok {
			continue
		}
		seen[link] = struct{}{}
		found = append(found, candidate{link: link, score: score})
	}
	sort.SliceStable(found, func(i, j int) bool {
		return found[i].score > found[j].score
	})
	if len(found) > 2 {
		found = found[:2]
	}
	links := make([]string, 0, len(found))
	for _, item := range found {
		links = append(links, item.link)
	}
	return links
}

func noiseAnchor(text string) bool {
	switch strings.TrimSpace(text) {
	case "首页", "登录", "注册", "更多", "下载", "联系我们", "网站地图", "上一页", "下一页", "English", "关于我们":
		return true
	}
	return false
}
