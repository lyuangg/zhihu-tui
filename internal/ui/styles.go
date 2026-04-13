package ui

import "github.com/charmbracelet/lipgloss"

// listSelectedItemBorderFG 与 bubbles/list.NewDefaultItemStyles().SelectedTitle 的 BorderForeground 一致（选中行左侧竖线）。
var listSelectedItemBorderFG = lipgloss.AdaptiveColor{Light: "#F793FF", Dark: "#AD58B4"}

var (
	titleStyle = lipgloss.NewStyle().Bold(true).Foreground(lipgloss.Color("205"))
	subStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("241"))
	errStyle   = lipgloss.NewStyle().Foreground(lipgloss.Color("196"))
	// 帮助页圆角框：灰色边框（与 subStyle 同色 241）
	boxStyle = lipgloss.NewStyle().
			Border(lipgloss.RoundedBorder()).
			BorderForeground(lipgloss.Color("241")).
			Padding(1, 2)
)
