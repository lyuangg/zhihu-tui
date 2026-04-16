package ui

import tea "github.com/charmbracelet/bubbletea"

// cmdStackBackOnLoading 在仅显示加载动画时：esc / h / left 与正常浏览一致，返回上一级。
// 根页（栈空）时 cmdBack 在 app 层为 no-op。
func cmdStackBackOnLoading(msg tea.Msg) (tea.Cmd, bool) {
	km, ok := msg.(tea.KeyMsg)
	if !ok {
		return nil, false
	}
	switch keyString(km) {
	case "esc", "h", "left":
		return cmdBack(), true
	default:
		return nil, false
	}
}
