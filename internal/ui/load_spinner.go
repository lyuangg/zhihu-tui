package ui

import "github.com/charmbracelet/bubbles/spinner"

// newLoadSpinner 与热榜页一致的加载动画（MiniDot）。
func newLoadSpinner() spinner.Model {
	return spinner.New(spinner.WithSpinner(spinner.MiniDot))
}
