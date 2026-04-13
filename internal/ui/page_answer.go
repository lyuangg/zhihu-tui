package ui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/lyuangg/zhihu-tui/internal/data"
	"github.com/lyuangg/zhihu-tui/internal/zhihu"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// answerBodySideCol 正文左侧指示列宽度（焦点在正文时为竖线，在评论时为空格占位，总宽仍为 w）
const answerBodySideCol = 1

type answerPage struct {
	api  data.API
	w, h int

	qid     string
	qTitle  string
	answers []zhihu.AnswerItem
	ansIdx  int
	ansOff  int
	ansTot  int
	ansEnd  bool

	curAnswer *zhihu.AnswerItem

	vp            viewport.Model
	focusComments bool
	comments      []zhihu.CommentItem
	cIdx          int
	cOff          int
	cEnd          bool

	answerBodyRendered string

	cList    list.Model
	loading  bool
	errStr   string
	lastY    bool
	loadSpin spinner.Model
}

func newAnswerPage(api data.API, w, h int, qid, qTitle string, answers []zhihu.AnswerItem, ansIdx, ansOff, ansTot int, ansEnd bool) *answerPage {
	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)
	p := &answerPage{
		api:       api,
		w:         w,
		h:         h,
		qid:       qid,
		qTitle:    qTitle,
		answers:   answers,
		ansIdx:    ansIdx,
		ansOff:    ansOff,
		ansTot:    ansTot,
		ansEnd:    ansEnd,
		curAnswer: nil,
		vp:        vp,
		cList:     newCommentList(),
		loading:   true,
		loadSpin:  newLoadSpinner(),
	}
	if ansIdx >= 0 && ansIdx < len(answers) {
		p.curAnswer = &answers[ansIdx]
	}
	p.applySizes()
	return p
}

// answerViewMeta 与 View 里 viewport 之上的元信息一致，用于按真实行数分配正文/评论区高度，避免总高度超过 p.h 把评论区裁出屏幕。
func (p *answerPage) answerViewMeta() string {
	if p.curAnswer == nil {
		return ""
	}
	var b strings.Builder
	a := p.curAnswer
	_, _ = fmt.Fprintf(&b, "%s  ·  ▲ %d  ·  评论约 %d 条\n\n", a.Author, a.Voteup, a.CommentCount)
	b.WriteString("\n")
	if p.errStr != "" {
		b.WriteString(errStyle.Render("评论加载: "+p.errStr) + "\n\n")
	}
	return b.String()
}

// answerBodyMarkdownWrapWidth 正文转 Markdown 时的折行宽度（扣除左侧 gutter 与 viewport.Style 水平内边距）。
func (p *answerPage) answerBodyMarkdownWrapWidth() int {
	frame := p.vp.Style.GetHorizontalFrameSize()
	if p.vp.Width > 0 {
		return max(1, p.vp.Width-frame)
	}
	return max(1, effectiveTermWidth(p.w)-answerBodySideCol-frame)
}

// lipBlockHeight 按与 View 相同的宽度折行后测高，避免 meta 折行时 applySizes 低估高度。
func lipBlockHeight(width int, s string) int {
	if width < 1 {
		width = 80
	}
	return lipgloss.Height(lipgloss.NewStyle().Width(width).Render(s))
}

// answerBodyWithLeftGutter 正文区：左侧一列 + viewport。焦点在正文时左列为与 list 选中项相同的左侧竖边框；焦点在评论时为空格占位，总宽仍为 w。
func (p *answerPage) answerBodyWithLeftGutter() string {
	h := p.vp.Height
	if h < 1 {
		return p.vp.View()
	}
	vp := p.vp.View()
	if p.focusComments {
		var col strings.Builder
		for i := 0; i < h-1; i++ {
			col.WriteString(" \n")
		}
		col.WriteString(" ")
		left := lipgloss.NewStyle().Width(answerBodySideCol).Render(col.String())
		return lipgloss.JoinHorizontal(lipgloss.Top, left, vp)
	}
	left := lipgloss.NewStyle().
		Height(h).
		Border(lipgloss.NormalBorder(), false, false, false, true).
		BorderForeground(listSelectedItemBorderFG).
		Render("")
	return lipgloss.JoinHorizontal(lipgloss.Top, left, vp)
}

func (p *answerPage) applySizes() {
	w := effectiveTermWidth(p.w)
	const (
		answerBodyViewportLines = 10 // 回答正文 viewport 内固定行数
		minListRows             = 3  // 评论列表最小高度
		// View: meta + 左 gutter + vp + "\n" + commentListView() + "\n"
		viewSepLines = 2
	)
	metaH := lipBlockHeight(w, p.answerViewMeta())
	rest := p.h - metaH - viewSepLines
	if rest < 1 {
		rest = 1
	}
	maxInnerH := rest - minListRows
	if maxInnerH < 1 {
		maxInnerH = 1
	}
	vpH := answerBodyViewportLines
	if vpH > maxInnerH {
		vpH = maxInnerH
	}
	listH := max(minListRows, rest-vpH)
	p.vp.Width = max(1, w-answerBodySideCol)
	p.vp.Height = vpH
	p.cList.SetSize(w, listH)
}

func (p *answerPage) Init() tea.Cmd {
	if p.curAnswer == nil {
		return nil
	}
	a := p.curAnswer
	return tea.Batch(
		func() tea.Msg {
			md := HTMLToTerminalMarkdown(a.ContentHTML, p.answerBodyMarkdownWrapWidth())
			cc, cEnd, err2 := p.api.FetchAnswerRootComments(p.qid, a.ID, p.cOff, commentLimit)
			if err2 != nil {
				return ansDone{md: md, comments: nil, cEnd: true, err: err2}
			}
			return ansDone{md: md, comments: cc, cEnd: cEnd}
		},
		func() tea.Msg { return p.loadSpin.Tick() },
	)
}

func (p *answerPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.w = msg.Width
		p.h = msg.Height
		p.applySizes()
		p.reflowBodyMarkdown()
		return p, nil

	case ansDone:
		p.loading = false
		if msg.err != nil && msg.md == "" {
			p.errStr = msg.err.Error()
			return p, nil
		}
		if msg.md != "" {
			md := normalizeCRLF(msg.md)
			p.answerBodyRendered = md
			p.vp.SetContent(md)
			p.vp.GotoTop()
		}
		if msg.comments != nil {
			p.comments = msg.comments
			p.cEnd = msg.cEnd
			p.cIdx = 0
		} else {
			p.comments = nil
			p.cEnd = true
			p.cIdx = 0
		}
		if msg.err != nil {
			p.errStr = msg.err.Error()
		} else {
			p.errStr = ""
		}
		p.reloadCommentItems()
		p.applySizes()
		return p, nil

	case commentDone:
		p.loading = false
		if msg.err != nil {
			p.errStr = msg.err.Error()
			return p, nil
		}
		p.errStr = ""
		if len(msg.items) == 0 && msg.offset > 0 && len(p.comments) > 0 {
			p.cEnd = true
			return p, nil
		}
		p.comments = msg.items
		p.cEnd = msg.isEnd
		p.cIdx = 0
		if msg.offset >= 0 {
			p.cOff = msg.offset
		}
		p.reloadCommentItems()
		p.applySizes()
		return p, nil

	case editorDoneMsg:
		applyEditorDoneMsg(&p.errStr, msg)
		p.applySizes()
		return p, nil
	}

	if p.loading {
		var spinCmd tea.Cmd
		p.loadSpin, spinCmd = p.loadSpin.Update(msg)
		if key, ok := msg.(tea.KeyMsg); ok && shouldGlobalQuit(key) {
			return p, tea.Quit
		}
		return p, spinCmd
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		return p.updateKey(msg)
	default:
		if p.focusComments {
			var cmd tea.Cmd
			p.cList, cmd = p.cList.Update(msg)
			p.cIdx = p.cList.Index()
			return p, cmd
		}
		var cmd tea.Cmd
		p.vp, cmd = p.vp.Update(msg)
		return p, cmd
	}
}

func (p *answerPage) reloadCommentItems() {
	items := make([]list.Item, len(p.comments))
	for i := range p.comments {
		items[i] = commentListItem{c: p.comments[i]}
	}
	_ = p.cList.SetItems(items)
	if p.cIdx >= len(p.comments) {
		p.cIdx = max(0, len(p.comments)-1)
	}
	p.cList.Select(p.cIdx)
}

func (p *answerPage) reflowBodyMarkdown() {
	if p.curAnswer == nil || p.loading {
		return
	}
	md := normalizeCRLF(HTMLToTerminalMarkdown(p.curAnswer.ContentHTML, p.answerBodyMarkdownWrapWidth()))
	p.answerBodyRendered = md
	p.vp.SetContent(md)
}

func (p *answerPage) fetchCommentPageCmd(offset int) tea.Cmd {
	a := p.curAnswer
	if a == nil {
		return nil
	}
	qid := p.qid
	api := p.api
	aid := a.ID
	return func() tea.Msg {
		cc, cEnd, err := api.FetchAnswerRootComments(qid, aid, offset, commentLimit)
		if err != nil {
			return commentDone{err: err, offset: -1}
		}
		return commentDone{items: cc, isEnd: cEnd, offset: offset}
	}
}

func (p *answerPage) forwardCommentList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	p.cList, cmd, p.cIdx = forwardBubbleList(p.cList, msg)
	return p, cmd
}

func (p *answerPage) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keyString(msg)

	if p.focusComments && p.cList.SettingFilter() {
		return p.forwardCommentList(msg)
	}
	resetYYLatchUnlessY(&p.lastY, k)

	switch k {
	case "esc", "h", "left":
		if p.focusComments && p.cList.IsFiltered() {
			return p.forwardCommentList(msg)
		}
		p.cList.ResetFilter()
		return p, cmdBack()
	case "tab":
		p.focusComments = !p.focusComments
		return p, nil
	case "o":
		return p.openCurrentAnswerInBrowser()
	case "e":
		return p, execEditorCmd(p.plainTextForEditor())
	case "y":
		if p.lastY {
			p.lastY = false
			return p.copyYY()
		}
		p.lastY = true
		return p, nil
	}

	if k == "n" || k == "p" {
		return p.commentPageKey(k)
	}

	if isOpenKey(k) {
		if p.focusComments && len(p.comments) > 0 && p.cIdx < len(p.comments) {
			c := p.comments[p.cIdx]
			meta := c.Author
			if ts := formatCommentTime(c.Time); ts != "" {
				meta += "  ·  " + ts
			}
			meta += fmt.Sprintf("  ·  ▲ %d", c.Likes)
			content := normalizeCRLF(HTMLToTerminalMarkdown(c.Content, p.w))
			return p, cmdForward(newReaderPage("评论全文", meta, content, p.w, p.h))
		}
		if !p.focusComments && strings.TrimSpace(p.answerBodyRendered) != "" {
			return p, cmdForward(newReaderPage("回答正文", "", p.answerBodyRendered, p.w, p.h))
		}
		return p, nil
	}

	// j：正文滚到底后继续 j → 聚焦评论区首条
	if !p.focusComments && k == "j" && p.vp.AtBottom() && len(p.comments) > 0 {
		p.focusComments = true
		p.cList.Select(0)
		p.cIdx = 0
		return p, nil
	}
	// k：评论列表已在顶部继续 k → 回到正文并上移视口
	if p.focusComments && k == "k" && !p.cList.SettingFilter() && p.commentListAtTop() {
		p.focusComments = false
		var cmd tea.Cmd
		p.vp, cmd = p.vp.Update(msg)
		return p, cmd
	}

	if p.focusComments {
		var cmd tea.Cmd
		p.cList, cmd = p.cList.Update(msg)
		p.cIdx = p.cList.Index()
		return p, cmd
	}
	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	return p, cmd
}

func (p *answerPage) commentListAtTop() bool {
	if len(p.comments) == 0 {
		return true
	}
	return p.cList.Index() == 0 && p.cList.Paginator.Page == 0
}

func (p *answerPage) commentPageKey(k string) (tea.Model, tea.Cmd) {
	n := len(p.comments)
	switch k {
	case "n":
		fullPage := n >= commentLimit
		if !p.cEnd || fullPage {
			nextOff := p.cOff + commentLimit
			p.loading = true
			return p, tea.Batch(p.fetchCommentPageCmd(nextOff), func() tea.Msg { return p.loadSpin.Tick() })
		}
	case "p":
		if p.cOff >= commentLimit {
			prevOff := p.cOff - commentLimit
			p.loading = true
			return p, tea.Batch(p.fetchCommentPageCmd(prevOff), func() tea.Msg { return p.loadSpin.Tick() })
		}
	}
	return p, nil
}

func (p *answerPage) openCurrentAnswerInBrowser() (tea.Model, tea.Cmd) {
	a := p.curAnswer
	if a == nil {
		p.errStr = "当前没有回答"
		return p, nil
	}
	qid := strings.TrimSpace(p.qid)
	aid := strings.TrimSpace(a.ID)
	if qid == "" || aid == "" {
		p.errStr = "当前条目没有有效链接"
		return p, nil
	}
	u := fmt.Sprintf("%s/question/%s/answer/%s", zhihu.BaseURL, url.PathEscape(qid), url.PathEscape(aid))
	applyErrOrClear(&p.errStr, openBrowserURL(u))
	return p, nil
}

func (p *answerPage) plainTextForEditor() string {
	if p.focusComments && len(p.comments) > 0 && p.cIdx >= 0 && p.cIdx < len(p.comments) {
		c := p.comments[p.cIdx]
		return fmt.Sprintf("%s\n%s\n", c.Author, stripHTMLFallback(c.Content))
	}
	return stripANSI(p.answerBodyRendered)
}

func (p *answerPage) copyYY() (tea.Model, tea.Cmd) {
	if p.focusComments && len(p.comments) > 0 && p.cIdx >= 0 && p.cIdx < len(p.comments) {
		c := p.comments[p.cIdx]
		plain := collapseText(stripHTMLFallback(c.Content))
		var line string
		if ts := formatCommentTime(c.Time); ts != "" {
			line = fmt.Sprintf("%s · %s: %s", ts, c.Author, plain)
		} else {
			line = fmt.Sprintf("%s: %s", c.Author, plain)
		}
		applyErrOrClear(&p.errStr, copyToClipboard(line))
		return p, nil
	}
	body := normalizeCRLF(p.answerBodyRendered)
	body = stripANSI(body)
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return p, nil
	}
	applyErrOrClear(&p.errStr, copyToClipboard(body))
	return p, nil
}

func (p *answerPage) View() string {
	if p.curAnswer == nil {
		return ""
	}
	a := p.curAnswer
	if p.loading {
		var b strings.Builder
		_, _ = fmt.Fprintf(&b, "%s  ·  ▲ %d  ·  评论约 %d 条\n\n", a.Author, a.Voteup, a.CommentCount)
		b.WriteString(p.loadSpin.View())
		b.WriteString(" ")
		b.WriteString(subStyle.Render("加载正文与评论…"))
		b.WriteString("\n")
		return b.String()
	}

	var b strings.Builder
	b.WriteString(p.answerViewMeta())
	b.WriteString(p.answerBodyWithLeftGutter())
	b.WriteString("\n")
	b.WriteString("\n")
	b.WriteString(p.commentListView())
	b.WriteString("\n")
	return b.String()
}

// commentListView 渲染评论列表。
// 之前未聚焦时通过 Select(n) 取消高亮会触发越界索引，导致偶发空列表显示。
// 这里统一直接渲染当前 cList，保证首屏稳定可见。
func (p *answerPage) commentListView() string {
	return p.cList.View()
}
