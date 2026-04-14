package ui

import (
	"fmt"
	"strings"

	"github.com/lyuangg/zhihu-tui/internal/data"
	"github.com/lyuangg/zhihu-tui/internal/zhihu"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// commentDetailHead 为顶栏展示用元信息；子评论完整列表通过 FetchCommentChildComments 分页拉取（root_comments 内嵌仅预览）。
type commentDetailHead struct {
	Author            string
	VoteCount         int
	ChildCommentCount int
	Time              int64
}

type commentDetailPage struct {
	api       data.API
	qid       string
	aid       string
	commentID string

	w, h        int
	contentHTML string
	head        commentDetailHead

	vp                 viewport.Model
	focusReplies       bool
	parentBodyRendered string

	children []zhihu.CommentItem
	cIdx     int
	cOff     int
	cEnd     bool

	cList    list.Model
	loading  bool
	errStr   string
	lastY    bool
	loadSpin spinner.Model
}

// newCommentDetailPage 用回答页传入的正文与内嵌预览初始化；若 child_comment_count>0 则再请求 child_comments 接口拉全部分页。
func newCommentDetailPage(api data.API, w, h int, qid, aid, commentID, contentHTML string, previewChildren []zhihu.CommentItem, head commentDetailHead) *commentDetailPage {
	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)
	ch := append([]zhihu.CommentItem(nil), previewChildren...)
	needFetch := strings.TrimSpace(commentID) != "" && head.ChildCommentCount > 0
	p := &commentDetailPage{
		api:          api,
		qid:          qid,
		aid:          aid,
		commentID:    strings.TrimSpace(commentID),
		w:            w,
		h:            h,
		contentHTML:  contentHTML,
		head:         head,
		vp:           vp,
		cList:        newCommentDetailList(),
		children:     ch,
		loading:      needFetch,
		focusReplies: len(ch) > 0,
		loadSpin:     newLoadSpinner(),
	}
	p.applySizes()
	p.reflowParentBody()
	p.reloadChildItems()
	return p
}

func (p *commentDetailPage) parentMarkdownWrapWidth() int {
	frame := p.vp.Style.GetHorizontalFrameSize()
	if p.vp.Width > 0 {
		return max(1, p.vp.Width-frame)
	}
	return max(1, effectiveTermWidth(p.w)-frame)
}

func (p *commentDetailPage) reflowParentBody() {
	md := normalizeCRLF(HTMLToTerminalMarkdown(p.contentHTML, p.parentMarkdownWrapWidth()))
	p.parentBodyRendered = md
	p.vp.SetContent(md)
	p.vp.GotoTop()
}

func (p *commentDetailPage) detailViewMeta() string {
	var b strings.Builder
	if p.errStr != "" {
		b.WriteString(errStyle.Render(p.errStr) + "\n\n")
	}
	h := p.head
	line := fmt.Sprintf("%s  ·  ▲ %d  ·  ↳ %d", h.Author, h.VoteCount, h.ChildCommentCount)
	if ts := formatCommentTime(h.Time); ts != "" {
		line = fmt.Sprintf("%s  ·  %s", ts, line)
	}
	b.WriteString(subStyle.Render(line))
	b.WriteString("\n\n")
	return b.String()
}

func (p *commentDetailPage) applySizes() {
	w := effectiveTermWidth(p.w)
	const (
		parentVpLines = 8
		minListRows   = 3
		viewSepLines  = 2
	)
	metaH := lipBlockHeight(w, p.detailViewMeta())
	rest := p.h - metaH - viewSepLines
	if rest < 1 {
		rest = 1
	}
	maxInnerH := rest - minListRows
	if maxInnerH < 1 {
		maxInnerH = 1
	}
	vpH := parentVpLines
	if vpH > maxInnerH {
		vpH = maxInnerH
	}
	listH := max(minListRows, rest-vpH)
	p.vp.Width = max(1, w)
	p.vp.Height = vpH
	p.cList.SetSize(w, listH)
}

func (p *commentDetailPage) Init() tea.Cmd {
	if !p.loading {
		return nil
	}
	return tea.Batch(
		func() tea.Msg {
			items, end, err := p.api.FetchCommentChildComments(p.qid, p.aid, p.commentID, 0, commentChildLimit)
			if err != nil {
				return commentDone{err: err, offset: -1}
			}
			return commentDone{items: items, isEnd: end, offset: 0}
		},
		func() tea.Msg { return p.loadSpin.Tick() },
	)
}

func (p *commentDetailPage) reloadChildItems() {
	items := make([]list.Item, len(p.children))
	for i := range p.children {
		items[i] = commentListItem{c: p.children[i]}
	}
	_ = p.cList.SetItems(items)
	if p.cIdx >= len(p.children) {
		p.cIdx = max(0, len(p.children)-1)
	}
	p.cList.Select(p.cIdx)
}

func (p *commentDetailPage) fetchChildPageCmd(offset int) tea.Cmd {
	qid := p.qid
	aid := p.aid
	cid := p.commentID
	api := p.api
	return func() tea.Msg {
		cc, cEnd, err := api.FetchCommentChildComments(qid, aid, cid, offset, commentChildLimit)
		if err != nil {
			return commentDone{err: err, offset: -1}
		}
		return commentDone{items: cc, isEnd: cEnd, offset: offset}
	}
}

func (p *commentDetailPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.w = msg.Width
		p.h = msg.Height
		p.applySizes()
		p.reflowParentBody()
		return p, nil

	case commentDone:
		p.loading = false
		if msg.err != nil {
			p.errStr = "加载子评论失败：" + msg.err.Error()
			p.focusReplies = len(p.children) > 0
			p.applySizes()
			return p, nil
		}
		p.errStr = ""
		if len(msg.items) == 0 && msg.offset > 0 && len(p.children) > 0 {
			p.cEnd = true
			p.errStr = "没有更多子评论，已停留在当前页"
			return p, nil
		}
		p.children = msg.items
		p.cEnd = msg.isEnd
		p.cIdx = 0
		if msg.offset >= 0 {
			p.cOff = msg.offset
		}
		p.focusReplies = len(p.children) > 0
		p.applySizes()
		p.reloadChildItems()
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
		if p.focusReplies {
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

func (p *commentDetailPage) forwardReplyList(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	var cmd tea.Cmd
	p.cList, cmd, p.cIdx = forwardBubbleList(p.cList, msg)
	return p, cmd
}

func (p *commentDetailPage) replyListAtTop() bool {
	if len(p.children) == 0 {
		return true
	}
	return p.cList.Index() == 0 && p.cList.Paginator.Page == 0
}

func (p *commentDetailPage) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keyString(msg)

	if p.focusReplies && p.cList.SettingFilter() {
		return p.forwardReplyList(msg)
	}
	resetYYLatchUnlessY(&p.lastY, k)

	switch k {
	case "esc", "h", "left":
		if p.focusReplies && p.cList.IsFiltered() {
			return p.forwardReplyList(msg)
		}
		p.cList.ResetFilter()
		return p, cmdBack()
	case "tab":
		p.focusReplies = !p.focusReplies
		return p, nil
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
		if p.head.ChildCommentCount > 0 && strings.TrimSpace(p.commentID) != "" {
			return p.childCommentPageKey(k)
		}
	}

	if isOpenKey(k) {
		if p.focusReplies && len(p.children) > 0 && p.cIdx < len(p.children) {
			c := p.children[p.cIdx]
			meta := c.Author
			if c.ReplyTo != "" {
				meta += "  →  " + c.ReplyTo
			}
			if ts := formatCommentTime(c.Time); ts != "" {
				meta += "  ·  " + ts
			}
			meta += fmt.Sprintf("  ·  ▲ %d", c.VoteCount)
			content := normalizeCRLF(HTMLToTerminalMarkdown(c.Content, p.w))
			return p, cmdForward(newReaderPage("子评论全文", meta, content, p.w, p.h))
		}
		if !p.focusReplies && strings.TrimSpace(p.parentBodyRendered) != "" {
			h := p.head
			meta := h.Author
			if ts := formatCommentTime(h.Time); ts != "" {
				meta += "  ·  " + ts
			}
			meta += fmt.Sprintf("  ·  ▲ %d  ·  ↳ %d", h.VoteCount, h.ChildCommentCount)
			content := normalizeCRLF(HTMLToTerminalMarkdown(p.contentHTML, p.w))
			return p, cmdForward(newReaderPage("评论全文", meta, content, p.w, p.h))
		}
		return p, nil
	}

	if !p.focusReplies && k == "j" && p.vp.AtBottom() && len(p.children) > 0 {
		p.focusReplies = true
		p.cList.Select(0)
		p.cIdx = 0
		return p, nil
	}
	if p.focusReplies && k == "k" && !p.cList.SettingFilter() && p.replyListAtTop() {
		p.focusReplies = false
		var cmd tea.Cmd
		p.vp, cmd = p.vp.Update(msg)
		return p, cmd
	}

	if p.focusReplies {
		var cmd tea.Cmd
		p.cList, cmd = p.cList.Update(msg)
		p.cIdx = p.cList.Index()
		return p, cmd
	}
	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	return p, cmd
}

func (p *commentDetailPage) childCommentPageKey(k string) (tea.Model, tea.Cmd) {
	n := len(p.children)
	switch k {
	case "n":
		fullPage := n >= commentChildLimit
		if !p.cEnd || fullPage {
			p.errStr = ""
			nextOff := p.cOff + commentChildLimit
			p.loading = true
			return p, tea.Batch(p.fetchChildPageCmd(nextOff), func() tea.Msg { return p.loadSpin.Tick() })
		}
		p.errStr = "已在最后一页子评论"
		return p, nil
	case "p":
		if p.cOff >= commentChildLimit {
			p.errStr = ""
			prevOff := p.cOff - commentChildLimit
			p.loading = true
			return p, tea.Batch(p.fetchChildPageCmd(prevOff), func() tea.Msg { return p.loadSpin.Tick() })
		}
		p.errStr = "已在第一页子评论"
		return p, nil
	default:
		return p, nil
	}
}

func (p *commentDetailPage) plainTextForEditor() string {
	if p.focusReplies && len(p.children) > 0 && p.cIdx >= 0 && p.cIdx < len(p.children) {
		c := p.children[p.cIdx]
		return fmt.Sprintf("%s\n%s\n", c.Author, stripHTMLFallback(c.Content))
	}
	return stripANSI(p.parentBodyRendered)
}

func (p *commentDetailPage) copyYY() (tea.Model, tea.Cmd) {
	if p.focusReplies && len(p.children) > 0 && p.cIdx >= 0 && p.cIdx < len(p.children) {
		c := p.children[p.cIdx]
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
	body := normalizeCRLF(p.parentBodyRendered)
	body = stripANSI(body)
	body = strings.TrimRight(body, "\n")
	if body == "" {
		return p, nil
	}
	applyErrOrClear(&p.errStr, copyToClipboard(body))
	return p, nil
}

func (p *commentDetailPage) View() string {
	if p.loading {
		var b strings.Builder
		b.WriteString(p.detailViewMeta())
		b.WriteString(p.vp.View())
		b.WriteString("\n")
		b.WriteString(p.loadSpin.View())
		b.WriteString(" ")
		b.WriteString(subStyle.Render("加载子评论…"))
		b.WriteString("\n")
		return b.String()
	}

	var b strings.Builder
	b.WriteString(p.detailViewMeta())
	b.WriteString(p.vp.View())
	b.WriteString("\n")
	b.WriteString(p.cList.View())
	b.WriteString("\n")
	return b.String()
}
