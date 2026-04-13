package ui

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	tea "github.com/charmbracelet/bubbletea"
)

func execEditorCmd(plain string) tea.Cmd {
	if strings.TrimSpace(plain) == "" {
		return func() tea.Msg {
			return editorDoneMsg{err: fmt.Errorf("当前无可编辑内容")}
		}
	}
	f, e := os.CreateTemp("", "zhihu-tui-*.md")
	if e != nil {
		return func() tea.Msg { return editorDoneMsg{err: e} }
	}
	path := f.Name()
	if _, e = f.WriteString(plain); e != nil {
		_ = f.Close()
		_ = os.Remove(path)
		return func() tea.Msg { return editorDoneMsg{err: e} }
	}
	if e = f.Close(); e != nil {
		_ = os.Remove(path)
		return func() tea.Msg { return editorDoneMsg{err: e} }
	}
	cmd := editorCommand(path)
	if cmd == nil {
		_ = os.Remove(path)
		return func() tea.Msg {
			return editorDoneMsg{err: fmt.Errorf("无法解析 $EDITOR")}
		}
	}
	return tea.ExecProcess(cmd, func(err error) tea.Msg {
		_ = os.Remove(path)
		return editorDoneMsg{err: err}
	})
}

func editorCommand(path string) *exec.Cmd {
	ed := strings.TrimSpace(os.Getenv("EDITOR"))
	if ed == "" {
		ed = "vim"
	}
	if strings.ContainsAny(ed, " \t") {
		return exec.Command("sh", "-c", ed+" "+shellQuote(path))
	}
	return exec.Command(ed, path)
}

func shellQuote(s string) string {
	return `'` + strings.ReplaceAll(s, `'`, `'"'"'`) + `'`
}
