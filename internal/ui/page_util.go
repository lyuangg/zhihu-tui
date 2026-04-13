package ui

import (
	"github.com/charmbracelet/bubbles/list"
	tea "github.com/charmbracelet/bubbletea"
)

// effectiveTermWidth 终端列数过小时用默认宽度，避免 layout 异常。
func effectiveTermWidth(w int) int {
	if w < 20 {
		return 80
	}
	return w
}

// resetYYLatchUnlessY 在非「y」键时清除 yy 连按状态；过滤模式下应在转发给列表之前 return，勿调用本函数。
func resetYYLatchUnlessY(lastY *bool, k string) {
	if k != "y" {
		*lastY = false
	}
}

func applyEditorDoneMsg(errStr *string, msg editorDoneMsg) {
	if msg.err != nil {
		*errStr = msg.err.Error()
	} else {
		*errStr = ""
	}
}

// applyErrOrClear 将 err 写入 errStr；err 为 nil 时清空提示。
func applyErrOrClear(errStr *string, err error) {
	if err != nil {
		*errStr = err.Error()
	} else {
		*errStr = ""
	}
}

func forwardBubbleList(l list.Model, msg tea.KeyMsg) (list.Model, tea.Cmd, int) {
	var cmd tea.Cmd
	l, cmd = l.Update(msg)
	return l, cmd, l.Index()
}
