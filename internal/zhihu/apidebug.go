package zhihu

import (
	"fmt"
	"net/url"
	"strings"
	"time"
)

const apiDebugMaxEntries = 14

type apiDebugEntry struct {
	t       time.Time
	kind    string
	url     string
	bytes   int
	err     string
	preview string
}

// SetDebug 开启后记录 navigate / 缓存命中 / 网络 JSON 的请求 URL、体积与响应片段（供 TUI 底部展示）。
func (c *Client) SetDebug(on bool) {
	if c == nil {
		return
	}
	c.dbgMu.Lock()
	defer c.dbgMu.Unlock()
	c.debug = on
	if !on {
		c.dbgRing = nil
	}
}

func (c *Client) appendDebugEnt(ent apiDebugEntry) {
	if c == nil || !c.debug {
		return
	}
	c.dbgMu.Lock()
	defer c.dbgMu.Unlock()
	c.dbgRing = append(c.dbgRing, ent)
	if len(c.dbgRing) > apiDebugMaxEntries {
		c.dbgRing = c.dbgRing[len(c.dbgRing)-apiDebugMaxEntries:]
	}
}

func (c *Client) apiDebugPush(kind, reqURL string, nbytes int, body []byte, err error) {
	if c == nil || !c.debug {
		return
	}
	ent := apiDebugEntry{
		t:     time.Now(),
		kind:  kind,
		url:   reqURL,
		bytes: nbytes,
	}
	if err != nil {
		ent.err = err.Error()
	}
	if len(body) > 0 {
		ent.preview = previewJSONBody(body, 220)
	}
	c.appendDebugEnt(ent)
}

func previewJSONBody(b []byte, maxRunes int) string {
	s := strings.Map(func(r rune) rune {
		if r == '\n' || r == '\r' || r == '\t' {
			return ' '
		}
		return r
	}, string(b))
	runes := []rune(strings.TrimSpace(s))
	if len(runes) > maxRunes {
		return string(runes[:maxRunes]) + "…"
	}
	return string(runes)
}

func truncateRunesOneLine(s string, max int) string {
	r := []rune(s)
	if len(r) <= max {
		return s
	}
	if max <= 1 {
		return "…"
	}
	return string(r[:max-1]) + "…"
}

// APIDebugLines 返回最近 API 轨迹（含请求 URL、字节数、错误、响应片段），每事件 1～2 行。
func (c *Client) APIDebugLines(trunc int) []string {
	if c == nil || !c.debug {
		return nil
	}
	c.dbgMu.Lock()
	defer c.dbgMu.Unlock()
	if len(c.dbgRing) == 0 {
		return nil
	}
	if trunc < 24 {
		trunc = 80
	}
	var out []string
	for _, e := range c.dbgRing {
		u := e.url
		if p, err := url.Parse(e.url); err == nil && p.RawQuery != "" {
			u = p.Path + " ? " + p.RawQuery
		}
		u = truncateRunesOneLine(u, trunc)
		line := fmt.Sprintf("%s %s · %dB · %s", e.t.Format("15:04:05"), e.kind, e.bytes, u)
		if e.err != "" {
			line += " · err=" + truncateRunesOneLine(e.err, min(100, trunc/2))
		}
		out = append(out, truncateRunesOneLine(line, trunc))
		if e.preview != "" {
			out = append(out, "  ↳ "+truncateRunesOneLine(e.preview, trunc-2))
		}
	}
	return out
}

func (c *Client) apiDebugNavigate(label, target string, err error) {
	if c == nil || !c.debug {
		return
	}
	ent := apiDebugEntry{t: time.Now(), kind: "nav·" + label, url: target}
	if err != nil {
		ent.err = err.Error()
	}
	c.appendDebugEnt(ent)
}
