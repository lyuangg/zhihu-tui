// Package bridge talks to the local daemon: HTTP POST /command.
// The daemon forwards to the Browser Bridge extension, which runs
// navigate/exec in Chrome so fetch() uses credentials: 'include' (logged-in session).
package bridge

import (
	"bytes"
	_ "embed"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"sync/atomic"
	"time"
)

// stealth_inject.js 注入浏览器环境以减少自动化特征。
//
//go:embed stealth_inject.js
var stealthInjectJS string

// settle.js 与 src/browser/dom-helpers.ts waitForDomStableJs 等价，Navigate 后等待 SPA 稳定再 fetch。
//
//go:embed settle.js
var settleJS string

const defaultPort = "19860"
const defaultDaemonBin = "zhihu-tui-daemon"

// Client drives the automation tab via daemon + extension.
type Client struct {
	baseURL   string
	workspace string
	http      *http.Client
	tabID     int
	hasTab    bool
	cmdID     atomic.Uint64
}

// NewClient returns a bridge client. workspace isolates the automation window.
func NewClient(workspace string) *Client {
	port := os.Getenv("ZHIHU_TUI_DAEMON_PORT")
	if port == "" {
		port = defaultPort
	}
	return &Client{
		baseURL:   "http://127.0.0.1:" + port,
		workspace: workspace,
		http: &http.Client{
			Timeout: 125 * time.Second,
		},
	}
}

// CheckDaemon returns nil if daemon responds and Browser Bridge extension is connected.
func (c *Client) CheckDaemon() error {
	if err := c.ensureDaemonRunning(); err != nil {
		return err
	}
	deadline := time.Now().Add(15 * time.Second)
	connected := false
	for {
		ok, err := c.extensionConnected()
		if err != nil {
			return err
		}
		if ok {
			connected = true
			break
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(400 * time.Millisecond)
	}
	if !connected {
		return fmt.Errorf("Browser Bridge 扩展未连接（daemon: %s）：请打开 Chrome/Chromium 并启用 zhihu-tui 扩展，且保持已登录 zhihu.com", c.baseURL)
	}
	return nil
}

func (c *Client) extensionConnected() (bool, error) {
	resp, err := c.status()
	if err != nil {
		return false, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 512))
		return false, fmt.Errorf("daemon /status HTTP %d: %s", resp.StatusCode, string(b))
	}
	var st struct {
		ExtensionConnected bool `json:"extensionConnected"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&st); err != nil {
		return false, err
	}
	return st.ExtensionConnected, nil
}

func (c *Client) status() (*http.Response, error) {
	req, err := http.NewRequest(http.MethodGet, c.baseURL+"/status", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("X-Zhihu-TUI", "1")
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("无法连接 daemon（%s）: %w", c.baseURL, err)
	}
	return resp, nil
}

func (c *Client) ensureDaemonRunning() error {
	resp, err := c.status()
	if err == nil {
		resp.Body.Close()
		return nil
	}
	if err := c.startDaemon(); err != nil {
		return err
	}
	deadline := time.Now().Add(3 * time.Second)
	for time.Now().Before(deadline) {
		resp, pingErr := c.status()
		if pingErr == nil {
			resp.Body.Close()
			return nil
		}
		time.Sleep(150 * time.Millisecond)
	}
	return fmt.Errorf("daemon 启动超时：%s", c.baseURL)
}

func daemonBin() string {
	if p := strings.TrimSpace(os.Getenv("ZHIHU_TUI_DAEMON_BIN")); p != "" {
		return p
	}
	return defaultDaemonBin
}

func resolveDaemonBin(bin string) string {
	// If env provides absolute/relative path, use it directly.
	if strings.Contains(bin, "/") {
		return bin
	}
	// Prefer PATH lookup first.
	if p, err := exec.LookPath(bin); err == nil && p != "" {
		return p
	}
	// Fallback to current working directory binary: ./zhihu-tui-daemon
	local := "." + string(os.PathSeparator) + bin
	if st, err := os.Stat(local); err == nil && !st.IsDir() {
		return local
	}
	return bin
}

func (c *Client) startDaemon() error {
	bin := resolveDaemonBin(daemonBin())
	cmd := exec.Command(bin)
	cmd.Stdout = io.Discard
	cmd.Stderr = io.Discard
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("启动 daemon 失败（%s）: %w", bin, err)
	}
	return cmd.Process.Release()
}

type cmdResult struct {
	ID    string          `json:"id"`
	OK    bool            `json:"ok"`
	Data  json.RawMessage `json:"data"`
	Error string          `json:"error"`
}

func (c *Client) send(action string, fields map[string]any) (json.RawMessage, error) {
	if err := c.ensureDaemonRunning(); err != nil {
		return nil, err
	}
	var lastErr error
	for attempt := 0; attempt < 2; attempt++ {
		id := fmt.Sprintf("zhihu_tui_%d_%d", time.Now().UnixMilli(), c.cmdID.Add(1))
		body := map[string]any{
			"id":        id,
			"action":    action,
			"workspace": c.workspace,
		}
		for k, v := range fields {
			body[k] = v
		}
		raw, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		req, err := http.NewRequest(http.MethodPost, c.baseURL+"/command", bytes.NewReader(raw))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("X-Zhihu-TUI", "1")

		resp, err := c.http.Do(req)
		if err != nil {
			lastErr = err
			if attempt == 0 && c.waitExtensionReconnect(2500*time.Millisecond) {
				continue
			}
			return nil, err
		}
		b, readErr := io.ReadAll(resp.Body)
		_ = resp.Body.Close()
		if readErr != nil {
			lastErr = readErr
			if attempt == 0 && c.waitExtensionReconnect(2500*time.Millisecond) {
				continue
			}
			return nil, readErr
		}
		if resp.StatusCode == http.StatusServiceUnavailable {
			lastErr = fmt.Errorf("扩展未连接：请启用 Browser Bridge 扩展")
			if attempt == 0 && c.waitExtensionReconnect(2500*time.Millisecond) {
				continue
			}
			return nil, lastErr
		}

		var cr cmdResult
		if err := json.Unmarshal(b, &cr); err != nil {
			return nil, fmt.Errorf("daemon 响应解析失败: %w: %s", err, string(b[:min(len(b), 200)]))
		}
		if !cr.OK {
			e := strings.TrimSpace(cr.Error)
			if e == "" {
				e = "daemon 命令失败"
			}
			lastErr = fmt.Errorf("%s", e)
			if attempt == 0 && isExtensionDisconnectMsg(e) && c.waitExtensionReconnect(2500*time.Millisecond) {
				continue
			}
			return nil, lastErr
		}
		return cr.Data, nil
	}
	if lastErr != nil {
		return nil, lastErr
	}
	return nil, fmt.Errorf("daemon 命令失败")
}

func isExtensionDisconnectMsg(msg string) bool {
	s := strings.ToLower(strings.TrimSpace(msg))
	return strings.Contains(s, "extension disconnected") ||
		strings.Contains(s, "扩展未连接") ||
		strings.Contains(s, "extension send failed")
}

func (c *Client) waitExtensionReconnect(timeout time.Duration) bool {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		ok, err := c.extensionConnected()
		if err == nil && ok {
			return true
		}
		time.Sleep(120 * time.Millisecond)
	}
	return false
}

// Navigate opens the automation tab to url (creates window on first use). Stores tabId for later exec.
func (c *Client) Navigate(url string) error {
	data, err := c.send("navigate", map[string]any{
		"url": url,
	})
	if err != nil {
		return err
	}
	var nav struct {
		TabID int `json:"tabId"`
	}
	if err := json.Unmarshal(data, &nav); err != nil {
		return fmt.Errorf("navigate 解析 tabId: %w", err)
	}
	if nav.TabID == 0 {
		return fmt.Errorf("navigate 未返回 tabId")
	}
	c.tabID = nav.TabID
	c.hasTab = true
	// 与 src/browser/page.ts goto 一致：navigate 完成后立刻 exec(stealth + waitForDomStable)，再单独 fetch。
	if err := c.postNavigateStealthAndSettle(); err != nil {
		return err
	}
	return nil
}

func (c *Client) postNavigateStealthAndSettle() error {
	code := strings.TrimSpace(stealthInjectJS) + ";\n" + strings.TrimSpace(settleJS)
	_, err := c.Exec(code)
	return err
}

// Exec runs JavaScript in the page via CDP Runtime.evaluate (async IIFE supported).
func (c *Client) Exec(js string) (json.RawMessage, error) {
	if !c.hasTab || c.tabID == 0 {
		return nil, fmt.Errorf("内部错误：尚未 navigate")
	}
	return c.send("exec", map[string]any{
		"code":  js,
		"tabId": c.tabID,
	})
}

// FetchJSON 与 clis/zhihu/question.ts 中 evaluate 完全一致：仅 fetch + credentials + r.json()。
// stealth/settle 已在 Navigate 后的 postNavigateStealthAndSettle 中执行。
func (c *Client) FetchJSON(apiURL string, dest any) error {
	if !c.hasTab {
		return fmt.Errorf("内部错误：尚未 Navigate 到任意页面")
	}
	code := `(async () => {
  const url = ` + strconv.Quote(apiURL) + `;
  const r = await fetch(url, { credentials: 'include' });
  if (!r.ok) return { __httpError: r.status };
  return await r.json();
})()`
	return c.fetchExecAndDecode(code, dest)
}

// FetchJSONHot 与 clis/zhihu/hot.yaml 一致：热榜 JSON 需对大整数 id 做字符串化再 parse。
func (c *Client) FetchJSONHot(apiURL string, dest any) error {
	if !c.hasTab {
		return fmt.Errorf("内部错误：尚未 Navigate 到任意页面")
	}
	code := `(async () => {
  const url = ` + strconv.Quote(apiURL) + `;
  const res = await fetch(url, { credentials: 'include' });
  if (!res.ok) return { __httpError: res.status };
  const text = await res.text();
  const fixed = text.replace(/("id"\s*:\s*)(\d{16,})/g, '$1"$2"');
  return JSON.parse(fixed);
})()`
	return c.fetchExecAndDecode(code, dest)
}

func (c *Client) fetchExecAndDecode(js string, dest any) error {
	raw, err := c.Exec(js)
	if err != nil {
		return err
	}
	var probe struct {
		HTTPError *float64 `json:"__httpError"`
	}
	_ = json.Unmarshal(raw, &probe)
	if probe.HTTPError != nil {
		st := int(*probe.HTTPError)
		if st == 401 || st == 403 {
			return fmt.Errorf("知乎 HTTP %d：请在该自动化窗口登录；若仍失败，可先在浏览器中确认登录态与扩展连接是否正常", st)
		}
		return fmt.Errorf("知乎 API HTTP %d", st)
	}
	if err := json.Unmarshal(raw, dest); err != nil {
		return fmt.Errorf("解析 JSON: %w", err)
	}
	return nil
}
