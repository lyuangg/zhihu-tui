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

  const parseISOSec = (s) => {
    const t = Date.parse(String(s || ''));
    return !isNaN(t) ? Math.floor(t / 1000) : 0;
  };
  const numSec = (v) => {
    if (typeof v !== 'number' || !isFinite(v)) return 0;
    if (v > 1e12 && v < 1e15) return Math.floor(v / 1000);
    if (v > 1e9 && v <= 1e12) return Math.floor(v);
    return 0;
  };
  const pad2 = (x) => String(x).padStart(2, '0');

  let createdTime = 0;
  let updatedTime = 0;
  const idStr = String(id || '');

  const applyMetaContent = (c) => parseISOSec(c);
  const mp = document.querySelector('meta[property="article:published_time"]');
  const mm = document.querySelector('meta[property="article:modified_time"]');
  if (mp && mp.content) createdTime = applyMetaContent(mp.content);
  if (mm && mm.content) updatedTime = applyMetaContent(mm.content);

  const mCreated = document.querySelector('meta[itemprop="dateCreated"], meta[itemprop="datePublished"]');
  const mModified = document.querySelector('meta[itemprop="dateModified"], meta[itemprop="dateUpdated"]');
  if (!createdTime && mCreated && mCreated.content) createdTime = applyMetaContent(mCreated.content);
  if (!updatedTime && mModified && mModified.content) updatedTime = applyMetaContent(mModified.content);

  try {
    document.querySelectorAll('script[type="application/ld+json"]').forEach((s) => {
      let j;
      try {
        j = JSON.parse(s.textContent || '');
      } catch {
        return;
      }
      const list = Array.isArray(j) ? j : [j];
      for (const x of list) {
        if (!x || typeof x !== 'object') continue;
        const typ = x['@type'];
        const isArticle =
          typ === 'Article' ||
          typ === 'NewsArticle' ||
          typ === 'BlogPosting' ||
          (Array.isArray(typ) && typ.some((t) => t === 'Article' || t === 'NewsArticle'));
        if (!isArticle) continue;
        if (x.datePublished && !createdTime) createdTime = parseISOSec(x.datePublished);
        if (x.dateModified && !updatedTime) updatedTime = parseISOSec(x.dateModified);
      }
    });
  } catch {
    /* ignore */
  }

  try {
    document.querySelectorAll('meta[content]').forEach((m) => {
      const ip = (m.getAttribute('itemprop') || '').toLowerCase();
      const prop = (m.getAttribute('property') || '').toLowerCase();
      const c = m.getAttribute('content');
      if (!c) return;
      const key = ip + ' ' + prop;
      if (!/(date|time|publish|create|modif|article)/.test(key)) return;
      const sec = parseISOSec(c);
      if (!sec) return;
      if (
        ip === 'datecreated' ||
        ip === 'datepublished' ||
        prop === 'article:published_time' ||
        prop === 'og:published_time'
      ) {
        if (!createdTime) createdTime = sec;
      }
      if (ip === 'datemodified' || ip === 'dateupdated' || prop === 'article:modified_time' || prop === 'og:updated_time') {
        if (!updatedTime) updatedTime = sec;
      }
    });
  } catch {
    /* ignore */
  }

  (() => {
    const el = document.querySelector('.ContentItem-time');
    if (!el) return;
    const raw = String(el.innerText || el.textContent || '').replace(/\s+/g, ' ');
    let m = raw.match(/发布于\s*(\d{4}-\d{2}-\d{2})(?:\s+(\d{1,2}):(\d{2}))?/);
    if (m) {
      const sec = parseISOSec(m[2] ? m[1] + 'T' + pad2(m[2]) + ':' + m[3] + ':00' : m[1] + 'T12:00:00');
      if (sec && !createdTime) createdTime = sec;
    }
    m = raw.match(/编辑于\s*(\d{4}-\d{2}-\d{2})(?:\s+(\d{1,2}):(\d{2}))?/);
    if (m) {
      const sec = parseISOSec(m[2] ? m[1] + 'T' + pad2(m[2]) + ':' + m[3] + ':00' : m[1] + 'T12:00:00');
      if (sec && !updatedTime) updatedTime = sec;
    }
  })();

  const parseCnPublished = (raw) => {
    const s = String(raw || '').replace(/\s+/g, ' ');
    let m = s.match(/(?:发布于|发表于)\s*[·\s]*(\d{4})[年/.-](\d{1,2})[月/.-](\d{1,2})日?(?:\s+(\d{1,2}):(\d{1,2}))?/);
    if (m) {
      const iso = m[4]
        ? m[1] + '-' + pad2(m[2]) + '-' + pad2(m[3]) + 'T' + pad2(m[4]) + ':' + pad2(m[5]) + ':00'
        : m[1] + '-' + pad2(m[2]) + '-' + pad2(m[3]) + 'T12:00:00';
      return parseISOSec(iso);
    }
    m = s.match(/(?:发布于|发表于)\s*(\d{4}-\d{2}-\d{2})(?:\s+(\d{1,2}):(\d{2}))?/);
    if (m) {
      const iso = m[2] ? m[1] + 'T' + pad2(m[2]) + ':' + m[3] + ':00' : m[1] + 'T12:00:00';
      return parseISOSec(iso);
    }
    return 0;
  };
  const parseCnUpdated = (raw) => {
    const s = String(raw || '').replace(/\s+/g, ' ');
    let m = s.match(/(?:编辑于|更新于)\s*(\d{4}-\d{2}-\d{2})(?:\s+(\d{1,2}):(\d{2}))?/);
    if (m) {
      const iso = m[2] ? m[1] + 'T' + pad2(m[2]) + ':' + m[3] + ':00' : m[1] + 'T12:00:00';
      return parseISOSec(iso);
    }
    m = s.match(/(?:编辑于|更新于)\s*[·\s]*(\d{4})[年/.-](\d{1,2})[月/.-](\d{1,2})日?/);
    if (!m) return 0;
    const iso = m[1] + '-' + pad2(m[2]) + '-' + pad2(m[3]) + 'T12:00:00';
    return parseISOSec(iso);
  };

  const textScope =
    document.querySelector('.Post-Header') ||
    document.querySelector('.Post-Main') ||
    document.querySelector('.Post-NormalMain') ||
    document.querySelector('article') ||
    document.body;
  if (textScope) {
    const blob = textScope.innerText.replace(/\s+/g, ' ').slice(0, 12000);
    if (!createdTime) {
      const t = parseCnPublished(blob);
      if (t) createdTime = t;
    }
    if (!updatedTime) {
      const t = parseCnUpdated(blob);
      if (t) updatedTime = t;
    }
  }

  const tryArticleObj = (o, depth) => {
    if (!o || depth > 22) return;
    if (typeof o !== 'object') return;
    if (Array.isArray(o)) {
      for (const x of o) tryArticleObj(x, depth + 1);
      return;
    }
    const oid = o.id != null ? String(o.id) : '';
    const ou = typeof o.url === 'string' ? o.url : '';
    const match =
      (idStr && oid === idStr) ||
      (idStr && ou.includes('/p/' + idStr)) ||
      (idStr && ou.includes('p/' + idStr)) ||
      (idStr && ou.includes('zhuanlan.zhihu.com/p/' + idStr));
    if (match) {
      const c =
        numSec(o.created) ||
        numSec(o.created_time) ||
        numSec(o.createdTime) ||
        numSec(o.created_at) ||
        numSec(o.publishTime);
      const u =
        numSec(o.updated) || numSec(o.updated_time) || numSec(o.updatedTime) || numSec(o.updated_at);
      if (c && !createdTime) createdTime = c;
      if (u && !updatedTime) updatedTime = u;
      if (!createdTime && !updatedTime) {
        const t = numSec(o.published) || numSec(o.publishedAt);
        if (t) createdTime = t;
      }
    }
    for (const v of Object.values(o)) tryArticleObj(v, depth + 1);
  };

  try {
    const nd = document.getElementById('__NEXT_DATA__');
    if (nd && nd.textContent) {
      const data = JSON.parse(nd.textContent);
      tryArticleObj(data, 0);
    }
  } catch {
    /* ignore */
  }

  if (!createdTime) {
    const hdr =
      document.querySelector('.Post-Header') ||
      document.querySelector('.Post-Main') ||
      document.querySelector('article') ||
      document.body;
    if (hdr) {
      const times = hdr.querySelectorAll('time[datetime]');
      for (const tm of times) {
        const ds = tm.getAttribute('datetime');
        if (ds) {
          const s = parseISOSec(ds);
          if (s) {
            createdTime = s;
            break;
          }
        }
      }
    }
  }

  if (!createdTime && !updatedTime) {
    const anyTime = document.querySelector('time[datetime]');
    if (anyTime) {
      const s = parseISOSec(anyTime.getAttribute('datetime'));
      if (s) createdTime = s;
    }
  }

  if (updatedTime && !createdTime) createdTime = updatedTime;
  if (createdTime && !updatedTime) updatedTime = createdTime;

  return {
    id,
    title,
    author,
    voteup,
    commentCount,
    contentHTML: content,
    createdTime,
    updatedTime,
    url: id ? ('https://zhuanlan.zhihu.com/p/' + encodeURIComponent(id)) : '',
  };
})()`
}
