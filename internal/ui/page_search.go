package ui

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/lyuangg/zhihu-tui/internal/data"
	"github.com/lyuangg/zhihu-tui/internal/zhihu"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/textinput"
	tea "github.com/charmbracelet/bubbletea"
)

type searchPage struct {
	api  data.API
	w, h int

	input    textinput.Model
	query    string
	items    []zhihu.SearchItem
	searchOf int
	idx      int
	list     list.Model
	loading  bool
	errStr   string
	lastY    bool
	loadSpin spinner.Model
}

type searchPageSnapshot struct {
	has      bool
	inputVal string
	query    string
	items    []zhihu.SearchItem
	searchOf int
	idx      int
}

var lastSearchSnapshot searchPageSnapshot

func newSearchPage(api data.API, w, h int) *searchPage {
	in := textinput.New()
	in.Placeholder = "输入关键词后回车搜索"
	in.CharLimit = 80
	in.Prompt = "关键词: "
	in.Focus()

	p := &searchPage{
		api:      api,
		w:        w,
		h:        h,
		input:    in,
		list:     newSearchList(),
		loadSpin: newLoadSpinner(),
	}
	p.restoreFromSnapshot()
	p.applyListSize()
	return p
}

func (p *searchPage) Init() tea.Cmd { return nil }

func (p *searchPage) applyListSize() {
	w := effectiveTermWidth(p.w)
	h := max(5, p.h-10)
	if len(p.items) > 0 {
		h = max(h, len(p.items)*2+8)
	}
	p.list.SetSize(w, h)
}

func (p *searchPage) fetchSearchCmd(query string, offset int) tea.Cmd {
	query = strings.TrimSpace(query)
	api := p.api
	return func() tea.Msg {
		items, err := api.Search(query, offset, searchPageSize)
		return searchDone{query: query, offset: offset, items: items, err: err}
	}
}

func (p *searchPage) resolveLinkCmd(qid, aid string) tea.Cmd {
	api := p.api
	return func() tea.Msg {
		if strings.TrimSpace(aid) == "" {
			return searchLinkDone{qid: qid}
		}
		title, ans, err := api.FetchAnswerPreview(qid, aid)
		if err != nil {
			return searchLinkDone{err: err}
		}
		return searchLinkDone{qid: qid, qTitle: title, ans: &ans}
	}
}

func (p *searchPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.w = msg.Width
		p.h = msg.Height
		p.applyListSize()
		return p, nil
	case searchDone:
		p.loading = false
		if msg.err != nil {
			p.errStr = msg.err.Error()
			return p, nil
		}
		p.errStr = ""
		p.query = msg.query
		p.searchOf = msg.offset
		p.items = msg.items
		p.idx = 0
		itms := make([]list.Item, 0, len(p.items))
		for _, it := range p.items {
			itms = append(itms, searchListItem{it: it})
		}
		cmd := p.list.SetItems(itms)
		p.list.Select(0)
		if len(p.items) > 0 {
			p.input.Blur()
		}
		p.applyListSize()
		p.persistSnapshot()
		return p, cmd
	case searchLinkDone:
		p.loading = false
		if msg.err != nil {
			p.errStr = msg.err.Error()
			return p, nil
		}
		if strings.TrimSpace(msg.qid) == "" {
			p.errStr = "无法识别知乎问题/回答链接"
			return p, nil
		}
		if msg.ans != nil {
			return p, cmdForward(newAnswerPage(p.api, p.w, p.h, msg.qid, msg.qTitle, []zhihu.AnswerItem{*msg.ans}, 0, 0, 1, true))
		}
		return p, cmdForward(newQuestionPage(p.api, p.w, p.h, msg.qid))
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
		p.list, cmd = p.list.Update(msg)
		p.idx = p.list.Index()
		p.persistSnapshot()
		return p, cmd
	}
}

func (p *searchPage) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keyString(msg)
	resetYYLatchUnlessY(&p.lastY, k)

	if p.list.SettingFilter() {
		return p.forwardList(msg)
	}

	switch k {
	case "esc":
		if p.list.IsFiltered() {
			return p.forwardList(msg)
		}
		return p, cmdBack()
	case "enter":
		if p.input.Focused() {
			q := strings.TrimSpace(p.input.Value())
			if q == "" {
				return p, nil
			}
			if qid, aid, ok := parseZhihuQuestionOrAnswerURL(q); ok {
				p.loading = true
				p.errStr = ""
				return p, tea.Batch(p.resolveLinkCmd(qid, aid), func() tea.Msg { return p.loadSpin.Tick() })
			}
			p.loading = true
			p.errStr = ""
			return p, tea.Batch(p.fetchSearchCmd(q, 0), func() tea.Msg { return p.loadSpin.Tick() })
		}
	case "r":
		p.input.Focus()
		return p, nil
	case "tab":
		if p.input.Focused() {
			p.input.Blur()
		} else {
			p.input.Focus()
		}
		return p, nil
	case "n":
		if p.input.Focused() {
			break
		}
		if strings.TrimSpace(p.query) == "" {
			return p, nil
		}
		p.loading = true
		return p, tea.Batch(p.fetchSearchCmd(p.query, p.searchOf+searchPageSize), func() tea.Msg { return p.loadSpin.Tick() })
	case "p":
		if p.input.Focused() {
			break
		}
		if strings.TrimSpace(p.query) == "" || p.searchOf == 0 {
			return p, nil
		}
		p.loading = true
		prev := p.searchOf - searchPageSize
		if prev < 0 {
			prev = 0
		}
		return p, tea.Batch(p.fetchSearchCmd(p.query, prev), func() tea.Msg { return p.loadSpin.Tick() })
	case "l", "right":
		if p.input.Focused() {
			break
		}
		return p.openSelected()
	case "o":
		if p.input.Focused() {
			break
		}
		return p.openInBrowser()
	case "e":
		if p.input.Focused() {
			break
		}
		return p, execEditorCmd(p.plainTextForEditor())
	case "y":
		if p.input.Focused() {
			break
		}
		if p.lastY {
			p.lastY = false
			return p.copyYY()
		}
		p.lastY = true
		return p, nil
	}

	if p.input.Focused() {
		var cmd tea.Cmd
		p.input, cmd = p.input.Update(msg)
		p.persistSnapshot()
		return p, cmd
	}
	return p.forwardList(msg)
}

func (p *searchPage) forwardList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	p.list, cmd, p.idx = forwardBubbleList(p.list, msg)
	p.persistSnapshot()
	return p, cmd
}

func (p *searchPage) currentItem() *zhihu.SearchItem {
	if p.idx < 0 || p.idx >= len(p.items) {
		return nil
	}
	return &p.items[p.idx]
}

func (p *searchPage) openSelected() (tea.Model, tea.Cmd) {
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
	default:
		if strings.TrimSpace(it.QuestionID) == "" {
			p.errStr = "当前结果无法进入详情页"
			return p, nil
		}
		return p, cmdForward(newQuestionPage(p.api, p.w, p.h, it.QuestionID))
	}
}

func parseArticleIDFromURL(raw string) string {
	u, err := url.Parse(strings.TrimSpace(raw))
	if err != nil {
		return ""
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host != "zhuanlan.zhihu.com" && host != "www.zhihu.com" {
		return ""
	}
	path := strings.Trim(strings.TrimSpace(u.EscapedPath()), "/")
	if strings.HasPrefix(path, "p/") {
		return strings.TrimSpace(strings.TrimPrefix(path, "p/"))
	}
	if strings.HasPrefix(path, "zhuanlan/p/") {
		return strings.TrimSpace(strings.TrimPrefix(path, "zhuanlan/p/"))
	}
	return ""
}

func (p *searchPage) openInBrowser() (tea.Model, tea.Cmd) {
	it := p.currentItem()
	if it == nil || strings.TrimSpace(it.URL) == "" {
		p.errStr = "当前条目没有有效链接"
		return p, nil
	}
	applyErrOrClear(&p.errStr, openBrowserURL(it.URL))
	return p, nil
}

func (p *searchPage) copyYY() (tea.Model, tea.Cmd) {
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

func (p *searchPage) plainTextForEditor() string {
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

func (p *searchPage) View() string {
	var b strings.Builder
	if p.errStr != "" {
		b.WriteString(errStyle.Render(p.errStr))
		b.WriteString("\n\n")
	}
	if p.loading {
		b.WriteString(p.loadSpin.View() + " " + subStyle.Render("搜索中…") + "\n\n")
	}

	b.WriteString(p.input.View())
	b.WriteString("\n")
	b.WriteString(subStyle.Render(fmt.Sprintf("Enter 搜索/链接直达  ·  Tab 切换输入/列表  ·  当前偏移 %d", p.searchOf)))
	b.WriteString("\n\n")
	if len(p.items) == 0 {
		if strings.TrimSpace(p.query) == "" {
			b.WriteString(subStyle.Render("请输入关键词并回车开始搜索。"))
		} else {
			b.WriteString(subStyle.Render("没有结果。"))
		}
		return b.String()
	}
	b.WriteString(p.list.View())
	return b.String()
}

var (
	zhihuAnswerURLRe   = regexp.MustCompile(`^/question/([0-9]+)/answer/([0-9]+)$`)
	zhihuQuestionURLRe = regexp.MustCompile(`^/question/([0-9]+)$`)
)

func parseZhihuQuestionOrAnswerURL(raw string) (questionID, answerID string, ok bool) {
	raw = strings.TrimSpace(raw)
	if raw == "" {
		return "", "", false
	}
	u, err := url.Parse(raw)
	if err != nil {
		return "", "", false
	}
	host := strings.ToLower(strings.TrimSpace(u.Hostname()))
	if host != "zhihu.com" && host != "www.zhihu.com" {
		return "", "", false
	}
	path := strings.TrimSuffix(strings.TrimSpace(u.EscapedPath()), "/")
	if m := zhihuAnswerURLRe.FindStringSubmatch(path); len(m) == 3 {
		return m[1], m[2], true
	}
	if m := zhihuQuestionURLRe.FindStringSubmatch(path); len(m) == 2 {
		return m[1], "", true
	}
	return "", "", false
}

func (p *searchPage) restoreFromSnapshot() {
	if !lastSearchSnapshot.has {
		return
	}
	p.input.SetValue(lastSearchSnapshot.inputVal)
	p.query = lastSearchSnapshot.query
	p.searchOf = lastSearchSnapshot.searchOf
	p.items = append([]zhihu.SearchItem(nil), lastSearchSnapshot.items...)
	p.idx = lastSearchSnapshot.idx
	if p.idx < 0 {
		p.idx = 0
	}
	if p.idx >= len(p.items) && len(p.items) > 0 {
		p.idx = len(p.items) - 1
	}
	items := make([]list.Item, 0, len(p.items))
	for _, it := range p.items {
		items = append(items, searchListItem{it: it})
	}
	_ = p.list.SetItems(items)
	if len(p.items) > 0 {
		p.list.Select(p.idx)
		p.input.Blur()
	}
}

func (p *searchPage) persistSnapshot() {
	lastSearchSnapshot = searchPageSnapshot{
		has:      true,
		inputVal: p.input.Value(),
		query:    p.query,
		items:    append([]zhihu.SearchItem(nil), p.items...),
		searchOf: p.searchOf,
		idx:      p.idx,
	}
}
