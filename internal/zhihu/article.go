package zhihu

import (
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
)

type ArticleItem struct {
	ID           string
	Title        string
	Author       string
	Voteup       int
	CommentCount int
	ContentHTML  string
	CreatedTime  int64
	UpdatedTime  int64
	URL          string
}

// FetchArticleDetail 从专栏文章页 DOM 拉取详情（不再请求 articles JSON API）。
func (c *Client) FetchArticleDetail(articleID string) (ArticleItem, error) {
	id := strings.TrimSpace(articleID)
	if id == "" {
		return ArticleItem{}, fmt.Errorf("文章 ID 不能为空")
	}
	cacheKey := articleDetailCacheKey(id)
	if b, ok := c.cacheGet(cacheKey); ok {
		var out ArticleItem
		if json.Unmarshal(b, &out) == nil && (strings.TrimSpace(out.Title) != "" || strings.TrimSpace(out.ContentHTML) != "") {
			return out, nil
		}
	}
	if err := c.PrepareArticlePage(id); err != nil {
		return ArticleItem{}, err
	}
	out, err := c.fetchArticleDetailFromPage(id)
	if err != nil {
		return ArticleItem{}, err
	}
	if b, mErr := json.Marshal(out); mErr == nil {
		c.cacheSet(cacheKey, b)
	}
	return out, nil
}

func (c *Client) fetchArticleDetailFromPage(articleID string) (ArticleItem, error) {
	raw, err := c.bridge.Exec(articleDomExecJS(articleID))
	if err != nil {
		return ArticleItem{}, err
	}
	var probe struct {
		Err string `json:"__error"`
	}
	_ = json.Unmarshal(raw, &probe)
	if strings.TrimSpace(probe.Err) != "" {
		return ArticleItem{}, errors.New(probe.Err)
	}
	var out ArticleItem
	if err := json.Unmarshal(raw, &out); err != nil {
		return ArticleItem{}, fmt.Errorf("解析文章页面失败: %w", err)
	}
	if strings.TrimSpace(out.ID) == "" {
		out.ID = strings.TrimSpace(articleID)
	}
	if strings.TrimSpace(out.URL) == "" && strings.TrimSpace(out.ID) != "" {
		out.URL = "https://zhuanlan.zhihu.com/p/" + url.PathEscape(out.ID)
	}
	return out, nil
}

func articleDomExecJS(articleID string) string {
	return `(async () => {
  const targetId = ` + strconv.Quote(strings.TrimSpace(articleID)) + `;
  const txt = (el) => (el && el.textContent ? el.textContent.trim() : '');
  const num = (s) => {
    const m = String(s || '').replace(/,/g, '').match(/(\d+)/);
    return m ? Number(m[1]) : 0;
  };
  const title =
    txt(document.querySelector('h1.Post-Title')) ||
    txt(document.querySelector('h1')) ||
    String(document.title || '').replace(/\s*-\s*知乎.*/, '').trim();
  const author =
    txt(document.querySelector('.AuthorInfo-name')) ||
    txt(document.querySelector('.Post-Author .AuthorInfo-name')) ||
    txt(document.querySelector('a[href*="/people/"]'));
  const contentEl =
    document.querySelector('.Post-RichTextContainer') ||
    document.querySelector('.RichText.ztext') ||
    document.querySelector('article');
  const content = contentEl ? contentEl.innerHTML : '';

  let voteup = 0;
  let commentCount = 0;
  const footer = document.querySelector('.ContentItem-actions') || document.body;
  if (footer) {
    const btns = Array.from(footer.querySelectorAll('button'));
    for (const b of btns) {
      const t = txt(b);
      if (!voteup && /赞同|赞/.test(t)) voteup = num(t);
      if (!commentCount && /评论/.test(t)) commentCount = num(t);
    }
  }
  const idFromURL = (() => {
    try {
      const p = String(location.pathname || '').replace(/^\/+|\/+$/g, '');
      if (p.startsWith('p/')) return p.slice(2);
      return '';
    } catch {
      return '';
    }
  })();
  const id = idFromURL || targetId || '';
  if (!title && !content) return { __error: '文章页面内容为空' };
  return {
    id,
    title,
    author,
    voteup,
    commentCount,
    contentHTML: content,
    createdTime: 0,
    updatedTime: 0,
    url: id ? ('https://zhuanlan.zhihu.com/p/' + encodeURIComponent(id)) : '',
  };
})()`
}
