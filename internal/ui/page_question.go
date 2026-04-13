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

type questionPage struct {
	api  data.API
	w, h int

	qid      string
	qTitle   string
	answers  []zhihu.AnswerItem
	ansIdx   int
	ansOff   int
	ansTot   int
	ansEnd   bool
	qList    list.Model
	loading  bool
	errStr   string
	lastY    bool
	loadSpin spinner.Model
}

func newQuestionPage(api data.API, w, h int, qid string) *questionPage {
	p := &questionPage{
		api:      api,
		w:        w,
		h:        h,
		qid:      strings.TrimSpace(qid),
		qList:    newQuestionList(),
		loading:  true,
		loadSpin: newLoadSpinner(),
	}
	_ = p.qList.SetItems(nil)
	p.applyListSize()
	return p
}

func (p *questionPage) applyListSize() {
	w := effectiveTermWidth(p.w)
	h := max(5, p.h-8)
	if len(p.answers) > 0 {
		h = max(h, len(p.answers)*2+8)
	}
	p.qList.SetSize(w, h)
}

func (p *questionPage) Init() tea.Cmd {
	return tea.Batch(
		func() tea.Msg {
			title, ans, isEnd, total, err := p.api.FetchQuestionPage(p.qid, p.ansOff, answerPageSize)
			return qDone{title: title, answers: ans, total: total, isEnd: isEnd, err: err}
		},
		func() tea.Msg { return p.loadSpin.Tick() },
	)
}

func (p *questionPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.w = msg.Width
		p.h = msg.Height
		p.applyListSize()
		return p, nil

	case qDone:
		p.loading = false
		if msg.err != nil {
			p.errStr = msg.err.Error()
			return p, nil
		}
		p.errStr = ""
		p.answers = msg.answers
		p.ansTot = msg.total
		p.ansEnd = msg.isEnd
		if msg.reloadRetain {
			if len(p.answers) == 0 {
				p.ansIdx = 0
			} else {
				p.ansIdx = min(msg.prevAnsIdx, len(p.answers)-1)
			}
		} else {
			p.ansIdx = 0
		}
		p.qTitle = strings.TrimSpace(msg.title)
		p.applyListSize()
		qItems := make([]list.Item, len(p.answers))
		for i := range p.answers {
			qItems[i] = ansListItem{a: p.answers[i], disp: i + 1}
		}
		cmd := p.qList.SetItems(qItems)
		p.qList.Select(p.ansIdx)
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
		p.qList, cmd = p.qList.Update(msg)
		p.ansIdx = p.qList.Index()
		return p, cmd
	}
}

func (p *questionPage) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keyString(msg)
	n := len(p.answers)

	if p.qList.SettingFilter() {
		return p.forwardList(msg)
	}
	resetYYLatchUnlessY(&p.lastY, k)

	if n == 0 && k != "r" && k != "l" && k != "h" && k != "esc" && k != "left" && k != "right" && k != "y" && k != "e" && k != "o" && k != "n" && k != "p" {
		return p, nil
	}

	switch k {
	case "esc":
		if p.qList.IsFiltered() {
			return p.forwardList(msg)
		}
		p.qList.ResetFilter()
		return p, cmdBack()
	case "h", "left":
		p.qList.ResetFilter()
		return p, cmdBack()
	case "o":
		return p.openCurrentQuestionInBrowser()
	case "e":
		return p, execEditorCmd(p.plainTextForEditor())
	case "y":
		if p.lastY {
			p.lastY = false
			return p.copyYY()
		}
		p.lastY = true
		return p, nil
	case "r":
		if strings.TrimSpace(p.qid) == "" {
			return p, nil
		}
		p.loading = true
		p.errStr = ""
		return p, tea.Batch(p.reloadQuestionCmd(), func() tea.Msg { return p.loadSpin.Tick() })
	case "n":
		if !p.ansEnd {
			p.loading = true
			p.ansOff += answerPageSize
			return p, tea.Batch(p.fetchQuestionPageCmd(), func() tea.Msg { return p.loadSpin.Tick() })
		}
		return p, nil
	case "p":
		if p.ansOff >= answerPageSize {
			p.loading = true
			p.ansOff -= answerPageSize
			return p, tea.Batch(p.fetchQuestionPageCmd(), func() tea.Msg { return p.loadSpin.Tick() })
		}
		return p, nil
	case "enter", "l", "right":
		if n == 0 {
			return p, nil
		}
		p.ansIdx = p.qList.Index()
		return p, cmdForward(newAnswerPage(p.api, p.w, p.h, p.qid, p.qTitle, p.answers, p.ansIdx, p.ansOff, p.ansTot, p.ansEnd))
	}

	return p.forwardList(msg)
}

func (p *questionPage) reloadQuestionCmd() tea.Cmd {
	prevIdx := p.ansIdx
	qid := p.qid
	off := p.ansOff
	ids := make([]string, len(p.answers))
	for i := range p.answers {
		ids[i] = p.answers[i].ID
	}
	api := p.api
	return func() tea.Msg {
		api.InvalidateQuestionCache(qid, ids)
		title, ans, isEnd, total, err := api.FetchQuestionPage(qid, off, answerPageSize)
		if err != nil {
			return qDone{err: err}
		}
		return qDone{
			title: title, answers: ans, total: total, isEnd: isEnd,
			reloadRetain: true, prevAnsIdx: prevIdx,
		}
	}
}

func (p *questionPage) fetchQuestionPageCmd() tea.Cmd {
	qid := p.qid
	off := p.ansOff
	api := p.api
	return func() tea.Msg {
		t, ans, isEnd, total, err := api.FetchQuestionPage(qid, off, answerPageSize)
		if err != nil {
			return qDone{err: err}
		}
		return qDone{title: t, answers: ans, total: total, isEnd: isEnd}
	}
}

func (p *questionPage) forwardList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	p.qList, cmd, p.ansIdx = forwardBubbleList(p.qList, msg)
	return p, cmd
}

func (p *questionPage) openCurrentQuestionInBrowser() (tea.Model, tea.Cmd) {
	qid := strings.TrimSpace(p.qid)
	if qid == "" {
		p.errStr = "当前条目没有有效链接"
		return p, nil
	}
	u := fmt.Sprintf("%s/question/%s", zhihu.BaseURL, url.PathEscape(qid))
	applyErrOrClear(&p.errStr, openBrowserURL(u))
	return p, nil
}

func (p *questionPage) plainTextForEditor() string {
	if sel := p.qList.SelectedItem(); sel != nil {
		if ai, ok := sel.(ansListItem); ok {
			return fmt.Sprintf("%s\n%s", ai.Title(), ai.Description())
		}
	}
	return ""
}

func (p *questionPage) copyYY() (tea.Model, tea.Cmd) {
	if sel := p.qList.SelectedItem(); sel != nil {
		if ai, ok := sel.(ansListItem); ok {
			line := fmt.Sprintf("%s\n%s", ai.Title(), ai.Description())
			applyErrOrClear(&p.errStr, copyToClipboard(line))
			return p, nil
		}
	}
	return p, nil
}

func (p *questionPage) View() string {
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

	b.WriteString(titleStyle.Render(collapseText(strings.TrimSpace(p.qTitle))))
	b.WriteString("\n")
	for _, line := range p.questionDetailLines() {
		b.WriteString(subStyle.Render(line))
		b.WriteString("\n")
	}
	b.WriteString("\n")

	if len(p.answers) == 0 {
		b.WriteString(subStyle.Render("暂无回答。"))
		return b.String()
	}

	b.WriteString(p.qList.View())
	return b.String()
}

func (p *questionPage) questionDetailLines() []string {
	lines := make([]string, 0, 4)
	qid := strings.TrimSpace(p.qid)
	if qid != "" {
		lines = append(lines, "问题 ID: "+qid)
		lines = append(lines, zhihu.BaseURL+"/question/"+qid)
	}
	page := p.ansOff/answerPageSize + 1
	if page < 1 {
		page = 1
	}
	endState := "否"
	if p.ansEnd {
		endState = "是"
	}
	lines = append(lines, fmt.Sprintf("回答总数: %d · 本页: %d", p.ansTot, len(p.answers)))
	lines = append(lines, fmt.Sprintf("分页: 第 %d 页 · 偏移 %d · 已到末页: %s", page, p.ansOff, endState))
	return lines
}
