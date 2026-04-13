package ui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/lyuangg/zhihu-tui/internal/data"
	"github.com/lyuangg/zhihu-tui/internal/zhihu"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

type hotPage struct {
	api  data.API
	w, h int

	hot      []zhihu.HotItem
	hotIdx   int
	hotList  list.Model
	loading  bool
	errStr   string
	lastY    bool
	loadSpin spinner.Model
}

func newHotPage(api data.API, w, h int) *hotPage {
	p := &hotPage{
		api:      api,
		w:        w,
		h:        h,
		hotList:  newHotList(),
		loading:  true,
		loadSpin: newLoadSpinner(),
	}
	p.applyListSize()
	return p
}

func (p *hotPage) applyListSize() {
	w := effectiveTermWidth(p.w)
	h := max(5, p.h-6)
	if len(p.hot) > 0 {
		// Hot item delegate is one-line; enlarge list height to avoid pagination.
		h = max(h, len(p.hot)+6)
	}
	p.hotList.SetSize(w, h)
}

func (p *hotPage) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			items, err := p.api.FetchHot(50)
			return hotDone{items: items, err: err}
		},
		func() tea.Msg { return p.loadSpin.Tick() },
	)
}

func (p *hotPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.w = msg.Width
		p.h = msg.Height
		p.applyListSize()
		return p, nil

	case hotDone:
		p.loading = false
		if msg.err != nil {
			p.errStr = msg.err.Error()
			return p, nil
		}
		p.errStr = ""
		p.hot = msg.items
		if p.hotIdx >= len(p.hot) {
			p.hotIdx = max(0, len(p.hot)-1)
		}
		p.applyListSize()
		items := make([]list.Item, len(p.hot))
		for i := range p.hot {
			items[i] = hotListItem{it: p.hot[i]}
		}
		cmd := p.hotList.SetItems(items)
		p.hotList.Select(p.hotIdx)
		return p, cmd

	case editorDoneMsg:
		applyEditorDoneMsg(&p.errStr, msg)
		return p, nil
	}

	if p.loading {
		var spinCmd tea.Cmd
		p.loadSpin, spinCmd = p.loadSpin.Update(msg)
		return p, spinCmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return p.updateKey(msg)
	default:
		var cmd tea.Cmd
		p.hotList, cmd = p.hotList.Update(msg)
		p.hotIdx = p.hotList.Index()
		return p, cmd
	}
}

func (p *hotPage) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keyString(msg)
	n := len(p.hot)

	if p.hotList.SettingFilter() {
		return p.forwardList(msg)
	}
	resetYYLatchUnlessY(&p.lastY, k)

	if n == 0 && k != "r" && k != "l" && k != "h" && k != "esc" && k != "left" && k != "right" && k != "y" && k != "e" && k != "o" && k != "f" {
		return p, nil
	}

	switch k {
	case "r":
		p.loading = true
		p.errStr = ""
		return p, tea.Batch(
			func() tea.Msg {
				p.api.InvalidateHotListCache()
				items, err := p.api.FetchHot(50)
				return hotDone{items: items, err: err}
			},
			func() tea.Msg { return p.loadSpin.Tick() },
		)
	case "o":
		if n == 0 {
			return p, nil
		}
		return p.openCurrentInBrowser()
	case "f":
		return p, cmdForward(newSearchPage(p.api, p.w, p.h))
	case "e":
		return p, execEditorCmd(p.plainTextForEditor())
	case "y":
		if p.lastY {
			p.lastY = false
			return p.copyYY()
		}
		p.lastY = true
		return p, nil
	case "enter", "l", "right":
		if n == 0 {
			return p, nil
		}
		return p, cmdForward(newQuestionPage(p.api, p.w, p.h, p.hot[p.hotIdx].QuestionID))
	case "esc", "h", "left":
		if p.hotList.IsFiltered() {
			return p.forwardList(msg)
		}
		return p, nil
	}

	return p.forwardList(msg)
}

func (p *hotPage) forwardList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	p.hotList, cmd, p.hotIdx = forwardBubbleList(p.hotList, msg)
	return p, cmd
}

func (p *hotPage) plainTextForEditor() string {
	if len(p.hot) == 0 {
		return ""
	}
	if sel := p.hotList.SelectedItem(); sel != nil {
		if hi, ok := sel.(hotListItem); ok {
			return hi.Title()
		}
	}
	return ""
}

func hotItemQuestionURL(it zhihu.HotItem) string {
	if u := strings.TrimSpace(it.QuestionURL); u != "" {
		return u
	}
	if id := strings.TrimSpace(it.QuestionID); id != "" {
		return fmt.Sprintf("%s/question/%s", zhihu.BaseURL, url.PathEscape(id))
	}
	return ""
}

func (p *hotPage) openCurrentInBrowser() (tea.Model, tea.Cmd) {
	var u string
	if sel := p.hotList.SelectedItem(); sel != nil {
		if hi, ok := sel.(hotListItem); ok {
			u = hotItemQuestionURL(hi.it)
		}
	}
	if u == "" && p.hotIdx >= 0 && p.hotIdx < len(p.hot) {
		u = hotItemQuestionURL(p.hot[p.hotIdx])
	}
	if u == "" {
		p.errStr = "当前条目没有有效链接"
		return p, nil
	}
	applyErrOrClear(&p.errStr, openBrowserURL(u))
	return p, nil
}

func (p *hotPage) copyYY() (tea.Model, tea.Cmd) {
	if sel := p.hotList.SelectedItem(); sel != nil {
		if hi, ok := sel.(hotListItem); ok {
			applyErrOrClear(&p.errStr, copyToClipboard(hi.Title()))
			return p, nil
		}
	}
	return p, nil
}

func (p *hotPage) View() string {
	var b strings.Builder
	if p.loading {
		b.WriteString(p.loadSpin.View())
		b.WriteString(" ")
		b.WriteString(subStyle.Render("加载中…"))
		b.WriteString("\n")
		return b.String()
	}
	if p.errStr != "" {
		b.WriteString(errStyle.Render(p.errStr) + "\n\n")
	}
	if len(p.hot) == 0 {
		b.WriteString(subStyle.Render("无数据。请检查 Cookie 或网络。"))
		return b.String()
	}

	b.WriteString(p.hotList.View())
	b.WriteString("\n")
	return b.String()
}
