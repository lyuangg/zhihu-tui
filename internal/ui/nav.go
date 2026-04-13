package ui

import (
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// NavViewOverheadLines 与 appModel.View 中 a.page 之上的块一致（顶栏 + "\n\n"），
// 按 width 折行后测高。子页 p.h 必须扣掉该值，否则会超出终端可视高度，底部内容（如「评论」标题）被裁掉。
func NavViewOverheadLines(width int, cur tea.Model) int {
	if width < 1 {
		width = 80
	}
	line := titleStyle.Render(NavPageName(cur)) + "  " + subStyle.Render(AppShortcutHints())
	block := lipgloss.NewStyle().Width(width).Render(line + "\n\n")
	return max(2, lipgloss.Height(block))
}

// AppShortcutHints 全局快捷键片段（与粗体页面名同一行展示，使用 subStyle 渲染）。
func AppShortcutHints() string {
	return "? 帮助  ·  q 退出  "
}

// NavPageName 当前页类型名称（仅顶栏粗体展示，不含路径与详情）。
func NavPageName(cur tea.Model) string {
	switch cur.(type) {
	case *hotPage:
		return "热榜"
	case *questionPage:
		return "问题"
	case *answerPage:
		return "回答"
	case *readerPage:
		return "阅读"
	case *helpPage:
		return "帮助"
	default:
		return "知乎"
	}
}

// navForwardMsg 由子页面通过 tea.Cmd 发给根模型：压栈并切换到 next。
type navForwardMsg struct {
	next tea.Model
}

// navBackMsg 关闭当前页，弹出栈顶上一页。
type navBackMsg struct{}

func cmdForward(next tea.Model) tea.Cmd {
	return func() tea.Msg {
		return navForwardMsg{next: next}
	}
}

func cmdBack() tea.Cmd {
	return func() tea.Msg { return navBackMsg{} }
}
