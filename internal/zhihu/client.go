package zhihu

import (
	"encoding/json"
	"fmt"
	"net/url"
	"sync"

	"github.com/lyuangg/zhihu-tui/internal/bridge"
)

// BaseURL is the Zhihu site origin used in API URLs.
const BaseURL = "https://www.zhihu.com"

// Client loads Zhihu JSON APIs through Browser Bridge (daemon + extension):
// navigate to zhihu.com then fetch(..., { credentials: 'include' }).
type Client struct {
	bridge       *bridge.Client
	defaultCache *simpleTTLCache // 热榜等（内存）
	answerCache  *lruTTLCache    // 问题回答列表 API，最近 100 条
	commentCache *lruTTLCache    // root_comments，最近 1000 条
	hotCacheDir  string          // 热榜落盘目录，如 $CACHE/zhihu-tui

	debug   bool
	dbgMu   sync.Mutex
	dbgRing []apiDebugEntry
}

func NewClient(b *bridge.Client) *Client {
	return &Client{
		bridge:       b,
		defaultCache: newSimpleTTLCache(),
		answerCache:  newLRUTTLCache(cacheLimitAnswers),
		commentCache: newLRUTTLCache(cacheLimitComments),
		hotCacheDir:  initHotCacheDir(),
	}
}

// HotCacheDir 返回热榜本地文件缓存目录（调试用）；若用户缓存目录不可用则为空字符串。
func (c *Client) HotCacheDir() string {
	if c == nil {
		return ""
	}
	return c.hotCacheDir
}

// InvalidateHotListCache 清除热榜接口缓存（手动刷新前调用）。
func (c *Client) InvalidateHotListCache() {
	if c.defaultCache != nil {
		c.defaultCache.invalidateMatching("hot-lists")
	}
	c.removeHotListDiskFiles()
}

// InvalidateSearchCache 清除 search_v3 接口缓存（搜索页手动刷新时调用）。
func (c *Client) InvalidateSearchCache() {
	if c.defaultCache != nil {
		c.defaultCache.invalidateMatching("/search_v3")
	}
}

// InvalidateQuestionCache 清除该问题下 answers 分页缓存；answerIDs 非空时同时清除对应 root_comments 缓存。
// questionID / answerID 为业务 id（与 API URL 中 PathEscape 一致）。
func (c *Client) InvalidateQuestionCache(questionID string, answerIDs []string) {
	if c == nil || questionID == "" {
		return
	}
	qpat := "/questions/" + url.PathEscape(questionID) + "/answers"
	if c.answerCache != nil {
		c.answerCache.invalidateMatching(qpat)
	}
	if c.commentCache != nil {
		for _, aid := range answerIDs {
			if aid == "" {
				continue
			}
			cpat := "/answers/" + url.PathEscape(aid) + "/root_comments"
			c.commentCache.invalidateMatching(cpat)
		}
	}
}

func (c *Client) cacheGet(url string) ([]byte, bool) {
	switch {
	case isAnswersAPIURL(url):
		if c.answerCache == nil {
			return nil, false
		}
		return c.answerCache.get(url)
	case isRootCommentsURL(url):
		if c.commentCache == nil {
			return nil, false
		}
		return c.commentCache.get(url)
	default:
		if c.defaultCache == nil {
			return nil, false
		}
		return c.defaultCache.get(url)
	}
}

func (c *Client) cacheSet(url string, raw []byte) {
	switch {
	case isAnswersAPIURL(url):
		c.answerCache.set(url, raw, cacheTTL)
	case isRootCommentsURL(url):
		c.commentCache.set(url, raw, cacheTTL)
	default:
		c.defaultCache.set(url, raw, cacheTTL)
	}
}

// jsonFromCache 若 URL 在缓存中且未过期则反序列化到 v 并返回 true。
// 热榜在内存未命中时会尝试本地文件（进程重启后仍可秒开）。
func (c *Client) jsonFromCache(apiURL string, v any) bool {
	if b, ok := c.cacheGet(apiURL); ok {
		if json.Unmarshal(b, v) == nil {
			c.apiDebugPush("json·cache(mem)", apiURL, len(b), b, nil)
			return true
		}
	}
	if stringsContainsHotLists(apiURL) {
		if b, ok := c.loadHotListFromFile(apiURL); ok {
			if json.Unmarshal(b, v) == nil {
				c.cacheSet(apiURL, b)
				c.apiDebugPush("json·cache(disk)", apiURL, len(b), b, nil)
				return true
			}
		}
	}
	return false
}

// getJSON：仅 credentials + r.json()（Navigate 后已由 bridge 注入 stealth+settle）。
func (c *Client) getJSON(url string, v any) error {
	if b, ok := c.cacheGet(url); ok {
		err := json.Unmarshal(b, v)
		c.apiDebugPush("getJSON·cache", url, len(b), b, err)
		return err
	}
	var raw json.RawMessage
	if err := c.bridge.FetchJSON(url, &raw); err != nil {
		c.apiDebugPush("getJSON·fetch", url, 0, nil, err)
		return err
	}
	payload := []byte(raw)
	c.cacheSet(url, payload)
	err := json.Unmarshal(payload, v)
	c.apiDebugPush("getJSON·fetch", url, len(payload), payload, err)
	return err
}

// getJSONHot：热榜大整数 id 需先正则再 parse。
func (c *Client) getJSONHot(url string, v any) error {
	if b, ok := c.cacheGet(url); ok {
		err := json.Unmarshal(b, v)
		c.apiDebugPush("getJSONHot·cache", url, len(b), b, err)
		return err
	}
	var raw json.RawMessage
	if err := c.bridge.FetchJSONHot(url, &raw); err != nil {
		c.apiDebugPush("getJSONHot·fetch", url, 0, nil, err)
		return err
	}
	payload := []byte(raw)
	c.cacheSet(url, payload)
	if stringsContainsHotLists(url) {
		c.saveHotListToFile(url, payload)
	}
	err := json.Unmarshal(payload, v)
	c.apiDebugPush("getJSONHot·fetch", url, len(payload), payload, err)
	return err
}

// PrepareHome opens the Zhihu homepage (hot API expects this context).
func (c *Client) PrepareHome() error {
	u := BaseURL + "/"
	err := c.bridge.Navigate(u)
	c.apiDebugNavigate("home", u, err)
	return err
}

// PrepareQuestion opens a question page before calling question/answers APIs.
func (c *Client) PrepareQuestion(questionID string) error {
	u := fmt.Sprintf("%s/question/%s", BaseURL, questionID)
	err := c.bridge.Navigate(u)
	c.apiDebugNavigate("question", u, err)
	return err
}

// PrepareAnswerPage opens an answer page before comment APIs.
func (c *Client) PrepareAnswerPage(questionID, answerID string) error {
	u := fmt.Sprintf("%s/question/%s/answer/%s", BaseURL, questionID, answerID)
	err := c.bridge.Navigate(u)
	c.apiDebugNavigate("answer", u, err)
	return err
}
