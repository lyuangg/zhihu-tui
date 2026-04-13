package ui

import (
	tea "github.com/charmbracelet/bubbletea"
)

func keyString(msg tea.KeyMsg) string {
	return msg.String()
}

func isQuitKey(k string) bool {
	return k == "ctrl+c" || k == "q"
}

func shouldGlobalQuit(msg tea.KeyMsg) bool {
	return isQuitKey(keyString(msg))
}

func isBackKey(k string) bool {
	return k == "esc" || k == "h" || k == "left"
}

func isOpenKey(k string) bool {
	return k == "enter" || k == "l" || k == "right"
}
