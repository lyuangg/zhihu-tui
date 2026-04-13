package ui

import (
	"fmt"
	"net/url"
	"strings"

	"github.com/lyuangg/zhihu-tui/internal/data"
	"github.com/lyuangg/zhihu-tui/internal/zhihu"

	"github.com/charmbracelet/bubbles/spinner"
	"github.com/charmbracelet/bubbles/viewport"
	tea "github.com/charmbracelet/bubbletea"
	"github.com/charmbracelet/lipgloss"
)

type articlePage struct {
	api  data.API
	w, h int

	articleID string
	item      *zhihu.ArticleItem

	vp      viewport.Model
	loading bool
	errStr  string
	lastY   bool

	bodyRendered string
	loadSpin     spinner.Model
}

func newArticlePage(api data.API, w, h int, articleID string) *articlePage {
	vp := viewport.New(0, 0)
	vp.Style = lipgloss.NewStyle().Padding(0, 1)
	p := &articlePage{
		api:       api,
		w:         w,
		h:         h,
		articleID: strings.TrimSpace(articleID),
		vp:        vp,
		loading:   true,
		loadSpin:  newLoadSpinner(),
	}
	p.applySizes()
	return p
}

func (p *articlePage) Init() tea.Cmd {
	id := p.articleID
	api := p.api
	return tea.Batch(
		func() tea.Msg {
			it, err := api.FetchArticleDetail(id)
			return articleDone{item: it, err: err}
		},
		func() tea.Msg { return p.loadSpin.Tick() },
	)
}

func (p *articlePage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.w = msg.Width
		p.h = msg.Height
		p.applySizes()
		p.reflowMarkdown()
		return p, nil
	case articleDone:
		p.loading = false
		if msg.err != nil {
			p.errStr = msg.err.Error()
			return p, nil
		}
		p.errStr = ""
		p.item = &msg.item
		p.articleID = strings.TrimSpace(msg.item.ID)
		p.reflowMarkdown()
		p.vp.GotoTop()
		return p, nil
	case editorDoneMsg:
		applyEditorDoneMsg(&p.errStr, msg)
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
		var cmd tea.Cmd
		p.vp, cmd = p.vp.Update(msg)
		return p, cmd
	}
}

func (p *articlePage) updateKey(msg tea.KeyMsg) (tea.Model, tea.Cmd) {
	k := keyString(msg)
	resetYYLatchUnlessY(&p.lastY, k)

	switch k {
	case "esc", "h", "left":
		return p, cmdBack()
	case "o":
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
	}

	if isOpenKey(k) && strings.TrimSpace(p.bodyRendered) != "" {
		return p, cmdForward(newReaderPage("文章正文", p.articleMetaLine(), p.bodyRendered, p.w, p.h))
	}

	var cmd tea.Cmd
	p.vp, cmd = p.vp.Update(msg)
	return p, cmd
}

func (p *articlePage) applySizes() {
	p.vp.Width = max(1, effectiveTermWidth(p.w))
	p.vp.Height = max(6, p.h-7)
}

func (p *articlePage) markdownWrapWidth() int {
	frame := p.vp.Style.GetHorizontalFrameSize()
	if p.vp.Width > 0 {
		return max(1, p.vp.Width-frame)
	}
	return max(1, effectiveTermWidth(p.w)-frame)
}

func (p *articlePage) reflowMarkdown() {
	if p.item == nil {
		p.bodyRendered = ""
		p.vp.SetContent("")
		return
	}
	md := normalizeCRLF(HTMLToTerminalMarkdown(p.item.ContentHTML, p.markdownWrapWidth()))
	p.bodyRendered = md
	p.vp.SetContent(md)
}

func (p *articlePage) articleMetaLine() string {
	if p.item == nil {
		return ""
	}
	meta := strings.TrimSpace(p.item.Author)
	if meta == "" {
		meta = "匿名作者"
	}
	meta += fmt.Sprintf("  ·  ▲ %d  ·  评论约 %d 条", p.item.Voteup, p.item.CommentCount)
	if ts := formatCommentTime(p.item.UpdatedTime); ts != "" {
		meta += "  ·  更新于 " + ts
	}
	return meta
}

func (p *articlePage) openInBrowser() (tea.Model, tea.Cmd) {
	if p.item != nil && strings.TrimSpace(p.item.URL) != "" {
		applyErrOrClear(&p.errStr, openBrowserURL(p.item.URL))
		return p, nil
	}
	id := strings.TrimSpace(p.articleID)
	if id == "" {
		p.errStr = "当前条目没有有效链接"
		return p, nil
	}
	u := fmt.Sprintf("https://zhuanlan.zhihu.com/p/%s", url.PathEscape(id))
	applyErrOrClear(&p.errStr, openBrowserURL(u))
	return p, nil
}

func (p *articlePage) plainTextForEditor() string {
	if p.item == nil {
		return ""
	}
	var b strings.Builder
	b.WriteString(strings.TrimSpace(p.item.Title))
	if meta := p.articleMetaLine(); meta != "" {
		b.WriteString("\n")
		b.WriteString(meta)
	}
	body := strings.TrimSpace(stripANSI(p.bodyRendered))
	if body != "" {
		b.WriteString("\n\n")
		b.WriteString(body)
	}
	if strings.TrimSpace(p.item.URL) != "" {
		b.WriteString("\n\n")
		b.WriteString(strings.TrimSpace(p.item.URL))
	}
	return b.String()
}

func (p *articlePage) copyYY() (tea.Model, tea.Cmd) {
	src := strings.TrimRight(stripANSI(normalizeCRLF(p.bodyRendered)), "\n")
	if src == "" {
		return p, nil
	}
	applyErrOrClear(&p.errStr, copyToClipboard(src))
	return p, nil
}

func (p *articlePage) View() string {
	var b strings.Builder
	if p.loading {
		b.WriteString(p.loadSpin.View())
		b.WriteString(" ")
		b.WriteString(subStyle.Render("加载文章详情…"))
		b.WriteString("\n")
		return b.String()
	}
	if p.errStr != "" {
		b.WriteString(errStyle.Render(p.errStr))
		b.WriteString("\n\n")
	}
	if p.item == nil {
		b.WriteString(subStyle.Render("文章为空。"))
		return b.String()
	}

	b.WriteString(titleStyle.Render(collapseText(strings.TrimSpace(p.item.Title))))
	b.WriteString("\n")
	if meta := p.articleMetaLine(); meta != "" {
		b.WriteString(subStyle.Render(meta))
		b.WriteString("\n")
	}
	if strings.TrimSpace(p.item.URL) != "" {
		b.WriteString(subStyle.Render(strings.TrimSpace(p.item.URL)))
		b.WriteString("\n")
	}
	b.WriteString("\n")
	b.WriteString(p.vp.View())
	return b.String()
}
