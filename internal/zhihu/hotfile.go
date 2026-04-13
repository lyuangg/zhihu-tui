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

type hotDiskEnvelope struct {
	FetchedAt time.Time       `json:"fetched_at"`
	Payload   json.RawMessage `json:"payload"`
}

func hotDiskFileName(apiURL string) string {
	limit := "50"
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
		b.WriteString("50")
	}
	return fmt.Sprintf("hot_limit_%s.json", b.String())
}

func (c *Client) hotDiskPath(apiURL string) string {
	if c.hotCacheDir == "" {
		return ""
	}
	return filepath.Join(c.hotCacheDir, hotDiskFileName(apiURL))
}

// loadHotListFromFile 读取本地热榜缓存；过期或不存在则返回 false。
func (c *Client) loadHotListFromFile(apiURL string) ([]byte, bool) {
	path := c.hotDiskPath(apiURL)
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

// saveHotListToFile 将热榜 JSON 写入本地（与内存 TTL 一致，按 fetched_at 判断新鲜度）。
func (c *Client) saveHotListToFile(apiURL string, payload []byte) {
	path := c.hotDiskPath(apiURL)
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

func (c *Client) removeHotListDiskFiles() {
	if c.hotCacheDir == "" {
		return
	}
	matches, err := filepath.Glob(filepath.Join(c.hotCacheDir, "hot_limit_*.json"))
	if err != nil {
		return
	}
	for _, p := range matches {
		_ = os.Remove(p)
	}
}

func initHotCacheDir() string {
	base, err := os.UserCacheDir()
	if err != nil {
		return ""
	}
	dir := filepath.Join(base, "zhihu-tui")
	if err := os.MkdirAll(dir, 0750); err != nil {
		return ""
	}
	return dir
}
