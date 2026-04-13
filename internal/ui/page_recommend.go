package ui

import (
	"strings"

	"github.com/lyuangg/zhihu-tui/internal/data"
	"github.com/lyuangg/zhihu-tui/internal/zhihu"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	tea "github.com/charmbracelet/bubbletea"
)

const recommendFetchLimit = 30

type recommendPage struct {
	api  data.API
	w, h int

	items    []zhihu.RecommendItem
	idx      int
	recList  list.Model
	loading  bool
	errStr   string
	lastY    bool
	loadSpin spinner.Model
}

func newRecommendPage(api data.API, w, h int) *recommendPage {
	p := &recommendPage{
		api:      api,
		w:        w,
		h:        h,
		recList:  newRecommendList(),
		loading:  true,
		loadSpin: newLoadSpinner(),
	}
	p.applyListSize()
	return p
}

func (p *recommendPage) applyListSize() {
	w := effectiveTermWidth(p.w)
	h := max(5, p.h-6)
	if len(p.items) > 0 {
		h = max(h, len(p.items)*2+8)
	}
	p.recList.SetSize(w, h)
}

func (p *recommendPage) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			items, err := p.api.FetchRecommend(recommendFetchLimit)
			return recommendDone{items: items, err: err}
		},
		func() tea.Msg { return p.loadSpin.Tick() },
	)
}

func (p *recommendPage) resolveAnswerCmd(qid, aid string) tea.Cmd {
	api := p.api
	return func() tea.Msg {
		title, ans, err := api.FetchAnswerPreview(qid, aid)
		if err != nil {
			return recommendAnswerDone{err: err}
		}
		return recommendAnswerDone{qid: qid, qTitle: title, ans: &ans}
	}
}

func (p *recommendPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.w = msg.Width
		p.h = msg.Height
		p.applyListSize()
		return p, nil

	case recommendDone:
		p.loading = false
		if msg.err != nil {
			p.errStr = msg.err.Error()
			return p, nil
		}
		p.errStr = ""
		p.items = msg.items
		if p.idx >= len(p.items) {
			p.idx = max(0, len(p.items)-1)
		}
		p.applyListSize()
		itms := make([]list.Item, len(p.items))
		for i := range p.items {
			itms[i] = recommendListItem{it: p.items[i]}
		}
		cmd := p.recList.SetItems(itms)
		p.recList.Select(p.idx)
		return p, cmd

	case recommendAnswerDone:
		p.loading = false
		if msg.err != nil {
			p.errStr = msg.err.Error()
			return p, nil
		}
		if msg.ans == nil || strings.TrimSpace(msg.qid) == "" {
			p.errStr = "无法打开该回答"
			return p, nil
		}
		return p, cmdForward(newAnswerPage(p.api, p.w, p.h, msg.qid, msg.qTitle, []zhihu.AnswerItem{*msg.ans}, 0, 0, 1, true))

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
		p.recList, cmd = p.recList.Update(msg)
		p.idx = p.recList.Index()
		return p, cmd
	}
}

func (p *recommendPage) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keyString(msg)
	n := len(p.items)

	if p.recList.SettingFilter() {
		return p.forwardList(msg)
	}
	resetYYLatchUnlessY(&p.lastY, k)

	if n == 0 && k != "r" && k != "esc" && k != "h" && k != "left" && k != "y" && k != "e" && k != "o" {
		return p, nil
	}

	switch k {
	case "esc", "h", "left":
		if p.recList.IsFiltered() {
			return p.forwardList(msg)
		}
		return p, cmdBack()
	case "r":
		p.loading = true
		p.errStr = ""
		return p, tea.Batch(
			func() tea.Msg {
				p.api.InvalidateRecommendCache()
				items, err := p.api.FetchRecommend(recommendFetchLimit)
				return recommendDone{items: items, err: err}
			},
			func() tea.Msg { return p.loadSpin.Tick() },
		)
	case "o":
		if n == 0 {
			return p, nil
		}
		return p.openInBrowser()
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
		return p.openSelected()
	}

	return p.forwardList(msg)
}

func (p *recommendPage) forwardList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	p.recList, cmd, p.idx = forwardBubbleList(p.recList, msg)
	return p, cmd
}

func (p *recommendPage) currentItem() *zhihu.RecommendItem {
	if p.idx < 0 || p.idx >= len(p.items) {
		return nil
	}
	return &p.items[p.idx]
}

func (p *recommendPage) openSelected() (tea.Model, tea.Cmd) {
	it := p.currentItem()
	if it == nil {
		return p, nil
	}
	switch strings.TrimSpace(it.Type) {
	case "article":
		aid := parseArticleIDFromURL(it.URL)
		if aid == "" {
			p.errStr = "当前文章没有有效 ID"
			return p, nil
		}
		return p, cmdForward(newArticlePage(p.api, p.w, p.h, aid))
	case "answer":
		if strings.TrimSpace(it.QuestionID) == "" || strings.TrimSpace(it.AnswerID) == "" {
			p.errStr = "当前回答缺少问题或回答 ID"
			return p, nil
		}
		p.loading = true
		p.errStr = ""
		return p, tea.Batch(
			p.resolveAnswerCmd(it.QuestionID, it.AnswerID),
			func() tea.Msg { return p.loadSpin.Tick() },
		)
	default:
		if strings.TrimSpace(it.QuestionID) == "" {
			p.errStr = "当前条目无法进入详情页"
			return p, nil
		}
		return p, cmdForward(newQuestionPage(p.api, p.w, p.h, it.QuestionID))
	}
}

func (p *recommendPage) openInBrowser() (tea.Model, tea.Cmd) {
	it := p.currentItem()
	if it == nil || strings.TrimSpace(it.URL) == "" {
		p.errStr = "当前条目没有有效链接"
		return p, nil
	}
	applyErrOrClear(&p.errStr, openBrowserURL(it.URL))
	return p, nil
}

func (p *recommendPage) copyYY() (tea.Model, tea.Cmd) {
	it := p.currentItem()
	if it == nil {
		return p, nil
	}
	line := strings.TrimSpace(it.Title)
	if strings.TrimSpace(it.URL) != "" {
		line += "\n" + strings.TrimSpace(it.URL)
	}
	applyErrOrClear(&p.errStr, copyToClipboard(strings.TrimSpace(line)))
	return p, nil
}

func (p *recommendPage) plainTextForEditor() string {
	it := p.currentItem()
	if it == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(it.Title)
	if it.Author != "" {
		b.WriteString("\n作者: " + it.Author)
	}
	if it.Excerpt != "" {
		b.WriteString("\n\n" + it.Excerpt)
	}
	if it.URL != "" {
		b.WriteString("\n\n" + it.URL)
	}
	return b.String()
}

func (p *recommendPage) View() string {
	var b strings.Builder
	if p.loading {
		b.WriteString(p.loadSpin.View())
		b.WriteString(" ")
		b.WriteString(subStyle.Render("加载推荐…"))
		b.WriteString("\n")
		return b.String()
	}
	if p.errStr != "" {
		b.WriteString(errStyle.Render(p.errStr) + "\n\n")
	}
	if len(p.items) == 0 {
		b.WriteString(subStyle.Render("无推荐数据。请检查登录态或稍后按 r 重试。"))
		return b.String()
	}
	b.WriteString(p.recList.View())
	b.WriteString("\n")
	return b.String()
}
