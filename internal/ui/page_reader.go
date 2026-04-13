package ui

import (
	"fmt"
	"strings"

	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type readerPage struct {
	w, h int

	vp   viewport.Model

	title string
	meta  string

	readerYySource string

	lastY  bool
	errStr string
}

func newReaderPage(title, meta, content string, w, h int) *readerPage {
	md := normalizeCRLF(content)
	v := viewport.New(0, 0)
	v.Style = lipgloss.NewStyle().Padding(0, 1)
	v.Width = effectiveTermWidth(w)
	v.Height = max(6, h-5)
	v.SetContent(md)
	v.GotoTop()
	return &readerPage{
		w:              w,
		h:              h,
		vp:             v,
		title:          strings.TrimSpace(title),
		meta:           strings.TrimSpace(meta),
		readerYySource: stripANSI(md),
	}
}

func (p *readerPage) Init() tea.Cmd { return nil }

func (p *readerPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.w = msg.Width
		p.h = msg.Height
		p.vp.Width = max(40, p.w)
		p.vp.Height = max(6, p.h-5)
		return p, nil

	case editorDoneMsg:
		applyEditorDoneMsg(&p.errStr, msg)
		return p, nil
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return p.updateKey(msg)
	default:
		var cmd tea.Cmd
		p.vp, cmd = p.vp.Update(msg)
		return p, cmd
	}
}

func (p *readerPage) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keyString(msg)
	if shouldGlobalQuit(msg) {
		return p, tea.Quit
	}
	resetYYLatchUnlessY(&p.lastY, k)
	if isBackKey(k) {
		return p, cmdBack()
	}
	if k == "e" {
		return p, execEditorCmd(p.readerYySource)
	}
	if k == "y" {
		if p.lastY {
			p.lastY = false
			return p.copyYY()
		}
		p.lastY = true
		return p, nil
	}
	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	return p, cmd
}

func (p *readerPage) copyYY() (tea.Model, tea.Cmd) {
	src := normalizeCRLF(p.readerYySource)
	src = stripANSI(src)
	src = strings.TrimRight(src, "\n")
	if src == "" {
		return p, nil
	}
	applyErrOrClear(&p.errStr, copyToClipboard(src))
	return p, nil
}

func (p *readerPage) View() string {
	var b strings.Builder
	if p.title != "" {
		b.WriteString(titleStyle.Render(p.title))
		b.WriteString("\n\n")
	}
	if p.meta != "" {
		b.WriteString(p.meta)
		b.WriteString("\n\n")
	}
	if p.errStr != "" {
		b.WriteString(errStyle.Render(p.errStr) + "\n\n")
	}
	b.WriteString(p.vp.View())
	src := normalizeCRLF(p.readerYySource)
	n := len(strings.Split(src, "\n"))
	if strings.TrimSpace(src) == "" {
		n = 0
	}
	if h := readerScrollHint(p.vp.YOffset, p.vp.Height, n); h != "" {
		b.WriteString("\n" + subStyle.Render(h))
	}
	return b.String()
}

func readerScrollHint(yOffset, vpHeight, totalLines int) string {
	if totalLines <= 0 || vpHeight <= 0 {
		return ""
	}
	vis := min(vpHeight, max(0, totalLines-yOffset))
	if vis <= 0 {
		return fmt.Sprintf("共 %d 行", totalLines)
	}
	last := yOffset + vis
	return fmt.Sprintf("第 %d–%d 行 / 共 %d 行", yOffset+1, last, totalLines)
}
