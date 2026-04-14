package ui

import (
	"fmt"
	"strings"

	"github.com/lyuangg/zhihu-tui/internal/zhihu"

	"github.com/charmbracelet/bubbles/list"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

// commentDetailHead 为顶栏展示用元信息，与正文、子评论均由回答页传入，详情页不请求接口。
type commentDetailHead struct {
	Author            string
	VoteCount         int
	ChildCommentCount int
	Time              int64
}

type commentDetailPage struct {
	w, h        int
	contentHTML string
	head        commentDetailHead

	vp                 viewport.Model
	focusReplies       bool
	parentBodyRendered string

	children []zhihu.CommentItem
	cIdx     int

	cList  list.Model
	errStr string
	lastY  bool
}

// newCommentDetailPage 仅渲染传入的正文与子评论列表，不发起网络请求。
func newCommentDetailPage(w, h int, contentHTML string, children []zhihu.CommentItem, head commentDetailHead) *commentDetailPage {
	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)
	ch := append([]zhihu.CommentItem(nil), children...)
	p := &commentDetailPage{
		w:            w,
		h:            h,
		contentHTML:  contentHTML,
		head:         head,
		vp:           vp,
		cList:        newCommentDetailList(),
		focusReplies: len(ch) > 0,
		children:     ch,
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

func (p *commentDetailPage) Init() tea.Cmd { return nil }

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

func (p *commentDetailPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.w = msg.Width
		p.h = msg.Height
		p.applySizes()
		p.reflowParentBody()
		return p, nil

	case editorDoneMsg:
		applyEditorDoneMsg(&p.errStr, msg)
		p.applySizes()
		return p, nil
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
	var b strings.Builder
	b.WriteString(p.detailViewMeta())
	b.WriteString(p.vp.View())
	b.WriteString("\n")
	b.WriteString(p.cList.View())
	b.WriteString("\n")
	return b.String()
}
