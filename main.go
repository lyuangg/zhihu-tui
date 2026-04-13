package main

import (
	"flag"
	"fmt"
	"os"
	"strings"

	"github.com/lyuangg/zhihu-tui/internal/bridge"
	"github.com/lyuangg/zhihu-tui/internal/data"
	"github.com/lyuangg/zhihu-tui/internal/ui"
	"github.com/lyuangg/zhihu-tui/internal/zhihu"

	tea "github.com/charmbracelet/bubbletea"
)

// 默认 workspace 为 site:zhihu。
// 若使用独立 workspace，会打开另一自动化窗口，往往未登录导致 API 403。
// 可通过环境变量 ZHIHU_TUI_WORKSPACE 覆盖（一般无需改）。
func workspace() string {
	if w := os.Getenv("ZHIHU_TUI_WORKSPACE"); w != "" {
		return w
	}
	return "site:zhihu"
}

// mockMode 为 true 时使用假数据，不连接浏览器桥接，便于单独测 TUI。
func mockMode() bool {
	v := strings.TrimSpace(os.Getenv("ZHIHU_TUI_MOCK"))
	if v == "" {
		return false
	}
	switch strings.ToLower(v) {
	case "1", "true", "yes", "on":
		return true
	default:
		return false
	}
}

func main() {
	mock := flag.Bool("mock", false, "使用 Mock 数据，不连接浏览器（也可设环境变量 ZHIHU_TUI_MOCK=1）")
	flag.Parse()

	var api data.API
	if *mock || mockMode() {
		api = data.NewMock()
	} else {
		br := bridge.NewClient(workspace())
		if err := br.CheckDaemon(); err != nil {
			fmt.Fprintln(os.Stderr, err)
			os.Exit(1)
		}
		api = zhihu.NewClient(br)
	}

	p := tea.NewProgram(ui.NewModel(api), tea.WithAltScreen())
	if _, err := p.Run(); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
