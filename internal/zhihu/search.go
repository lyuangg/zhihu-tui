package zhihu

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"strconv"
	"strings"
)

type SearchItem struct {
	Type       string
	Title      string
	Excerpt    string
	Author     string
	Voteup     int
	URL        string
	QuestionID string
}

func (c *Client) Search(query string, offset, limit int) ([]SearchItem, error) {
	q := strings.TrimSpace(query)
	if q == "" {
		return nil, nil
	}
	u := fmt.Sprintf("%s/api/v4/search_v3?q=%s&t=general&offset=%d&limit=%d",
		BaseURL,
		url.QueryEscape(q),
		max(0, offset),
		max(1, limit),
	)
	if b, ok := c.cacheGet(u); ok {
		var out []SearchItem
		err := json.Unmarshal(b, &out)
		c.apiDebugPush("search·cache", u, len(b), b, err)
		if err == nil {
			return out, nil
		}
	}
	if err := c.PrepareHome(); err != nil {
		return nil, err
	}
	raw, err := c.bridge.Exec(searchExecJS(q, max(0, offset), max(1, limit)))
	if err != nil {
		c.apiDebugPush("search·exec", u, 0, nil, err)
		return nil, err
	}
	var probe struct {
		HTTPError *float64 `json:"__httpError"`
	}
	_ = json.Unmarshal(raw, &probe)
	if probe.HTTPError != nil {
		st := int(*probe.HTTPError)
		err = fmt.Errorf("知乎搜索 API HTTP %d", st)
		if st == 401 || st == 403 {
			err = fmt.Errorf("知乎搜索 HTTP %d：请确认自动化窗口已登录 zhihu.com", st)
		}
		c.apiDebugPush("search·exec", u, len(raw), raw, err)
		return nil, err
	}

	var out []SearchItem
	if err := json.Unmarshal(raw, &out); err != nil {
		c.apiDebugPush("search·exec", u, len(raw), raw, err)
		return nil, fmt.Errorf("解析搜索结果失败: %w", err)
	}
	c.cacheSet(u, raw)
	c.apiDebugPush("search·exec", u, len(raw), raw, nil)
	return out, nil
}

func searchExecJS(query string, offset, limit int) string {
	return `(async () => {
  const strip = (html) => (html || '')
    .replace(/<[^>]+>/g, '')
    .replace(/&nbsp;/g, ' ')
    .replace(/&lt;/g, '<')
    .replace(/&gt;/g, '>')
    .replace(/&amp;/g, '&')
    .replace(/&#39;/g, "'")
    .replace(/&quot;/g, '"')
    .trim();
  const keyword = ` + strconv.Quote(query) + `;
  const offset = ` + strconv.Itoa(offset) + `;
  const limit = ` + strconv.Itoa(limit) + `;
  const api = 'https://www.zhihu.com/api/v4/search_v3?q=' + encodeURIComponent(keyword) + '&t=general&offset=' + offset + '&limit=' + limit;
  const res = await fetch(api, { credentials: 'include' });
  if (!res.ok) return { __httpError: res.status };
  const d = await res.json();
  return (d?.data || [])
    .filter(item => item?.type === 'search_result')
    .map(item => {
      const obj = item?.object || {};
      const type = (obj?.type || '').trim();
      const qid = String(obj?.question?.id || '');
      const id = String(obj?.id || '');
      let url = '';
      let questionID = '';
      let normType = type || 'question';
      if (normType === 'answer') {
        questionID = qid;
        if (qid && id) url = 'https://www.zhihu.com/question/' + encodeURIComponent(qid) + '/answer/' + encodeURIComponent(id);
      } else if (normType === 'article') {
        if (id) url = 'https://zhuanlan.zhihu.com/p/' + encodeURIComponent(id);
      } else {
        normType = 'question';
        questionID = id;
        if (id) url = 'https://www.zhihu.com/question/' + encodeURIComponent(id);
      }
      return {
        type: normType,
        title: strip(obj?.title || obj?.question?.name || ''),
        excerpt: strip(obj?.excerpt || ''),
        author: String(obj?.author?.name || ''),
        voteup: Number(obj?.voteup_count || 0),
        url,
        questionID,
      };
    });
})()`
}

var searchTagRe = regexp.MustCompile(`<[^>]+>`)

func stripSearchHTML(s string) string {
	s = strings.TrimSpace(s)
	if s == "" {
		return ""
	}
	s = searchTagRe.ReplaceAllString(s, "")
	repl := strings.NewReplacer(
		"&nbsp;", " ",
		"&lt;", "<",
		"&gt;", ">",
		"&amp;", "&",
		"&#39;", "'",
		"&quot;", `"`,
	)
	s = repl.Replace(s)
	return strings.TrimSpace(s)
}
