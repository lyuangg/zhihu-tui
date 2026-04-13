package zhihu

import (
	"container/list"
	"strings"
	"sync"
	"time"
)

// 所有条目统一 TTL；answer / 评论 再按「最近访问」条数上限淘汰。
const cacheTTL = 24 * time.Hour

const (
	cacheLimitAnswers  = 100
	cacheLimitComments = 1000
	cacheLimitArticles = 300
)

type cacheEnt struct {
	raw []byte
	exp time.Time
}

// simpleTTLCache 无条数上限（用于热榜等键很少的请求）。
type simpleTTLCache struct {
	mu sync.RWMutex
	m  map[string]cacheEnt
}

func newSimpleTTLCache() *simpleTTLCache {
	return &simpleTTLCache{m: make(map[string]cacheEnt)}
}

func (c *simpleTTLCache) get(key string) ([]byte, bool) {
	c.mu.RLock()
	e, ok := c.m[key]
	c.mu.RUnlock()
	if !ok || time.Now().After(e.exp) {
		return nil, false
	}
	return e.raw, true
}

func (c *simpleTTLCache) set(key string, raw []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.m == nil {
		c.m = make(map[string]cacheEnt)
	}
	c.m[key] = cacheEnt{raw: append([]byte(nil), raw...), exp: time.Now().Add(ttl)}
}

func (c *simpleTTLCache) invalidateMatching(substr string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	for k := range c.m {
		if strings.Contains(k, substr) {
			delete(c.m, k)
		}
	}
}

// lruTTLCache：LRU（最近访问在前）+ TTL；超上限时淘汰最久未访问的条目。
type lruTTLCache struct {
	mu     sync.Mutex
	limit  int
	data   map[string]cacheEnt
	lru    *list.List
	elemOf map[string]*list.Element
}

func newLRUTTLCache(limit int) *lruTTLCache {
	return &lruTTLCache{
		limit:  limit,
		data:   make(map[string]cacheEnt),
		lru:    list.New(),
		elemOf: make(map[string]*list.Element),
	}
}

func (c *lruTTLCache) get(key string) ([]byte, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	e, ok := c.data[key]
	if !ok || time.Now().After(e.exp) {
		if ok {
			c.removeKeyLocked(key)
		}
		return nil, false
	}
	if el, ok := c.elemOf[key]; ok {
		c.lru.MoveToFront(el)
	}
	return e.raw, true
}

func (c *lruTTLCache) set(key string, raw []byte, ttl time.Duration) {
	c.mu.Lock()
	defer c.mu.Unlock()
	exp := time.Now().Add(ttl)
	ent := cacheEnt{raw: append([]byte(nil), raw...), exp: exp}
	if el, exists := c.elemOf[key]; exists {
		c.data[key] = ent
		c.lru.MoveToFront(el)
		return
	}
	for c.limit > 0 && c.lru.Len() >= c.limit {
		c.evictOldestLocked()
	}
	c.data[key] = ent
	el := c.lru.PushFront(key)
	c.elemOf[key] = el
}

func (c *lruTTLCache) removeKeyLocked(key string) {
	if el, ok := c.elemOf[key]; ok {
		c.lru.Remove(el)
		delete(c.elemOf, key)
	}
	delete(c.data, key)
}

func (c *lruTTLCache) evictOldestLocked() {
	el := c.lru.Back()
	if el == nil {
		return
	}
	key := el.Value.(string)
	c.lru.Remove(el)
	delete(c.elemOf, key)
	delete(c.data, key)
}

// invalidateMatching 删除 key 中包含 substr 的条目（用于按问题 id / 回答 id 批量失效缓存）。
func (c *lruTTLCache) invalidateMatching(substr string) {
	if c == nil || substr == "" {
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	var keys []string
	for k := range c.data {
		if strings.Contains(k, substr) {
			keys = append(keys, k)
		}
	}
	for _, k := range keys {
		c.removeKeyLocked(k)
	}
}

func isAnswersAPIURL(u string) bool {
	return strings.Contains(u, "/answers?")
}

func isRootCommentsURL(u string) bool {
	return strings.Contains(u, "/root_comments")
}

// articleDetailCacheKeyPrefix 文章详情仅走专栏页 DOM，缓存键不用真实 HTTP URL。
const articleDetailCacheKeyPrefix = "zhihu-tui:article-detail:"

func articleDetailCacheKey(articleID string) string {
	return articleDetailCacheKeyPrefix + strings.TrimSpace(articleID)
}

func isArticleDetailCacheURL(u string) bool {
	return strings.HasPrefix(u, articleDetailCacheKeyPrefix)
}

func stringsContainsHotLists(u string) bool {
	return strings.Contains(u, "hot-lists")
}
