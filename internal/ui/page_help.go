package ui

import (
	"strings"

	tea "github.com/charmbracelet/bubbletea"
	"github.com/mattn/go-runewidth"
)

type helpPage struct {
	w, h int
}

func newHelpPage(w, h int) *helpPage {
	return &helpPage{w: w, h: h}
}

func (p *helpPage) Init() tea.Cmd { return nil }

func (p *helpPage) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		p.w = msg.Width
		p.h = msg.Height
		return p, nil
	case tea.KeyMsg:
		if shouldGlobalQuit(msg) {
			return p, tea.Quit
		}
		k := keyString(msg)
		if isBackKey(k) || k == "?" {
			return p, cmdBack()
		}
		return p, nil
	default:
		return p, nil
	}
}

// helpKeyCol 为快捷键列显示宽度（runewidth），说明从同一列起始对齐。
const helpKeyCol = 20
const helpKeyDescGap = "  "

func helpLine(key, desc string) string {
	if key == "" {
		return strings.Repeat(" ", helpKeyCol+len(helpKeyDescGap)) + desc + "\n"
	}
	pad := helpKeyCol - runewidth.StringWidth(key)
	if pad < 1 {
		pad = 1
	}
	return key + strings.Repeat(" ", pad) + helpKeyDescGap + desc + "\n"
}

func helpPageBody() string {
	var b strings.Builder
	w := func(key, desc string) { b.WriteString(helpLine(key, desc)) }
	n := func(desc string) { b.WriteString(helpLine("", desc)) }

	b.WriteString("【全局】\n")
	w("?", "帮助")
	w("q", "退出")
	w("Ctrl+C", "同 q")
	w("h / Esc", "返回上一层；阅读页 h / Esc / ← 关闭阅读")
	w("Enter / l / →", "打开选中（热榜→问题、问题→回答、回答→阅读）")
	w("o", "浏览器当前上下文")
	w("r", "刷新（热榜、问题页）")
	w("n / p", "翻页（热榜列表、问题回答、回答评论）")
	w("yy", "复制")
	w("y", "连按两次同 yy")
	w("e", "$EDITOR 打开内容见各页说明")
	w("Tab", "回答页正文 / 评论")
	w("j / k", "滚动（回答页正文与评论衔接见下）")
	w("↑↓ / j / k", "热榜、问题、评论列表内移光标；/ 过滤；Esc 清过滤或交给列表")
	b.WriteString("\n")

	b.WriteString("【热榜】\n")
	n("无返回；过滤中 Esc 先给列表")
	w("f", "进入搜索页")
	w("e / yy", "当前列表展示行；列表更多键见列表底")
	b.WriteString("\n")

	b.WriteString("【搜索】\n")
	w("Esc", "返回上一层（搜索页仅 Esc 可返回）")
	w("Enter", "输入框聚焦时执行搜索；若为问题/回答链接则直达预览")
	w("Tab", "切换输入框与结果列表焦点")
	w("r", "聚焦输入框，编辑关键词")
	w("n / p", "搜索结果翻页")
	w("l / →", "结果为问题/回答时进入问题页")
	b.WriteString("\n")

	b.WriteString("【问题】\n")
	w("e / yy", "当前回答摘要行")
	w("Esc / h / ←", "过滤中先清过滤，否则回热榜")
	b.WriteString("\n")

	b.WriteString("【回答】\n")
	w("e", "全文")
	w("yy", "正文焦点复制全文，评论焦点复制摘要")
	n("正文滚底再 j → 评论首条；评论顶且非过滤再 k → 正文")
	w("Esc / h / ←", "过滤优先，否则回问题")
	b.WriteString("\n")

	b.WriteString("【阅读】\n")
	w("yy", "复制当前阅读全文")

	return b.String()
}

func (p *helpPage) View() string {
	return boxStyle.Width(effectiveTermWidth(p.w) - 4).Render(helpPageBody())
}
