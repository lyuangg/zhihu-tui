package zhihu

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// RecommendItem 为首页推荐流中的一条可展示条目（问题 / 回答 / 文章）。
type RecommendItem struct {
	Type       string
	Title      string
	Excerpt    string
	Author     string
	Voteup     int
	URL        string
	QuestionID string
	AnswerID   string
}

type recommendAPI struct {
	Data []struct {
		Target *struct {
			Type        string `json:"type"`
			ID          any    `json:"id"`
			Title       string `json:"title"`
			Excerpt     string `json:"excerpt"`
			VoteupCount int    `json:"voteup_count"`
			Author      struct {
				Name string `json:"name"`
			} `json:"author"`
			Question *struct {
				ID    any    `json:"id"`
				Title string `json:"title"`
			} `json:"question"`
		} `json:"target"`
	} `json:"data"`
}

func stringsContainsRecommend(u string) bool {
	return strings.Contains(u, "feed/topstory/recommend")
}

func recommendDiskFileName(apiURL string) string {
	limit := "20"
	if parsed, err := url.Parse(apiURL); err == nil {
		if q := parsed.Query().Get("limit"); q != "" {
			limit = strings.TrimSpace(q)
		}
	}
	var b strings.Builder
	for _, r := range limit {
		if r >= '0' && r <= '9' {
			b.WriteRune(r)
		}
	}
	if b.Len() == 0 {
		b.WriteString("20")
	}
	return fmt.Sprintf("recommend_limit_%s.json", b.String())
}

func (c *Client) recommendDiskPath(apiURL string) string {
	if c == nil || c.hotCacheDir == "" {
		return ""
	}
	return filepath.Join(c.hotCacheDir, recommendDiskFileName(apiURL))
}

// loadRecommendFromFile 读取推荐流本地缓存；过期或不存在则返回 false。
func (c *Client) loadRecommendFromFile(apiURL string) ([]byte, bool) {
	path := c.recommendDiskPath(apiURL)
	if path == "" {
		return nil, false
	}
	data, err := os.ReadFile(path)
	if err != nil || len(data) == 0 {
		return nil, false
	}
	var env hotDiskEnvelope
	if err := json.Unmarshal(data, &env); err != nil {
		return nil, false
	}
	if time.Since(env.FetchedAt) > cacheTTL {
		_ = os.Remove(path)
		return nil, false
	}
	if len(env.Payload) == 0 {
		return nil, false
	}
	return []byte(env.Payload), true
}

func (c *Client) saveRecommendToFile(apiURL string, payload []byte) {
	path := c.recommendDiskPath(apiURL)
	if path == "" {
		return
	}
	env := hotDiskEnvelope{
		FetchedAt: time.Now(),
		Payload:   append(json.RawMessage(nil), payload...),
	}
	data, err := json.Marshal(env)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0600)
}

func (c *Client) removeRecommendDiskFiles() {
	if c.hotCacheDir == "" {
		return
	}
	matches, err := filepath.Glob(filepath.Join(c.hotCacheDir, "recommend_limit_*.json"))
	if err != nil {
		return
	}
	for _, p := range matches {
		_ = os.Remove(p)
	}
}

// InvalidateRecommendCache 清除推荐接口内存与本地文件缓存。
func (c *Client) InvalidateRecommendCache() {
	if c.defaultCache != nil {
		c.defaultCache.invalidateMatching("topstory/recommend")
	}
	c.removeRecommendDiskFiles()
}

// FetchRecommend 拉取首页推荐流（需已登录；与热榜相同走 bridge + 大整数 id 修复）。
func (c *Client) FetchRecommend(limit int) ([]RecommendItem, error) {
	lim := min(50, max(1, limit))
	u := fmt.Sprintf("%s/api/v3/feed/topstory/recommend?desktop=true&limit=%d", BaseURL, lim)
	var raw recommendAPI
	if c.jsonFromCache(u, &raw) {
		return recommendItemsFromRaw(&raw), nil
	}
	if err := c.PrepareHome(); err != nil {
		return nil, err
	}
	if err := c.getJSONHot(u, &raw); err != nil {
		return nil, err
	}
	return recommendItemsFromRaw(&raw), nil
}

func recommendItemsFromRaw(raw *recommendAPI) []RecommendItem {
	out := make([]RecommendItem, 0, len(raw.Data))
	for _, item := range raw.Data {
		tg := item.Target
		if tg == nil {
			continue
		}
		typ := strings.ToLower(strings.TrimSpace(tg.Type))
		title := strings.TrimSpace(stripSearchHTML(tg.Title))
		excerpt := strings.TrimSpace(stripSearchHTML(tg.Excerpt))
		author := strings.TrimSpace(tg.Author.Name)
		vote := tg.VoteupCount

		switch typ {
		case "answer":
			qid := ""
			qtitle := ""
			if tg.Question != nil {
				qid = idString(tg.Question.ID)
				qtitle = strings.TrimSpace(stripSearchHTML(tg.Question.Title))
			}
			aid := idString(tg.ID)
			if qid == "" || aid == "" {
				continue
			}
			t := firstNonEmpty(title, qtitle)
			if t == "" {
				t = "回答"
			}
			u := fmt.Sprintf("%s/question/%s/answer/%s", BaseURL, url.PathEscape(qid), url.PathEscape(aid))
			out = append(out, RecommendItem{
				Type:       "answer",
				Title:      t,
				Excerpt:    excerpt,
				Author:     author,
				Voteup:     vote,
				URL:        u,
				QuestionID: qid,
				AnswerID:   aid,
			})
		case "article":
			aid := idString(tg.ID)
			if aid == "" {
				continue
			}
			t := title
			if t == "" {
				t = "文章"
			}
			u := fmt.Sprintf("https://zhuanlan.zhihu.com/p/%s", url.PathEscape(aid))
			out = append(out, RecommendItem{
				Type:    "article",
				Title:   t,
				Excerpt: excerpt,
				Author:  author,
				Voteup:  vote,
				URL:     u,
			})
		case "question":
			qid := idString(tg.ID)
			if qid == "" {
				continue
			}
			t := title
			if t == "" {
				t = "问题"
			}
			u := fmt.Sprintf("%s/question/%s", BaseURL, url.PathEscape(qid))
			out = append(out, RecommendItem{
				Type:       "question",
				Title:      t,
				Excerpt:    excerpt,
				Author:     author,
				Voteup:     vote,
				URL:        u,
				QuestionID: qid,
			})
		default:
			continue
		}
	}
	return out
}

func firstNonEmpty(a, b string) string {
	a = strings.TrimSpace(a)
	if a != "" {
		return a
	}
	return strings.TrimSpace(b)
}
