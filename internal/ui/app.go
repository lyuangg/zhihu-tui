package ui

import (
	"github.com/lyuangg/zhihu-tui/internal/data"

	tea "github.com/charmbracelet/bubbletea"
)

// appModel 根模型：维护页面栈、数据接口与终端尺寸。
type appModel struct {
	api  data.API
	w, h int

	stack []tea.Model
	page  tea.Model
}

// NewModel 创建可运行的 Bubble Tea 根模型（首页为热榜）。
func NewModel(api data.API) tea.Model {
	w, termH := 80, 24
	oh := NavViewOverheadLines(w, nil)
	contentH := max(8, termH-oh)
	p := newHotPage(api, w, contentH)
	return &appModel{
		api:  api,
		w:    w,
		h:    contentH,
		page: p,
	}
}

func (a *appModel) Init() tea.Cmd {
	return a.page.Init()
}

func (a *appModel) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.WindowSizeMsg:
		a.w = msg.Width
		oh := NavViewOverheadLines(a.w, a.page)
		a.h = max(8, msg.Height-oh)
		next, c := a.page.Update(tea.WindowSizeMsg{Width: a.w, Height: a.h})
		a.page = next
		return a, c

	case navForwardMsg:
		a.stack = append(a.stack, a.page)
		a.page = msg.next
		return a, a.page.Init()

	case navBackMsg:
		if len(a.stack) == 0 {
			return a, nil
		}
		a.page = a.stack[len(a.stack)-1]
		a.stack = a.stack[:len(a.stack)-1]
		next, c := a.page.Update(tea.WindowSizeMsg{Width: a.w, Height: a.h})
		a.page = next
		return a, c
	}

	if km, ok := msg.(tea.KeyMsg); ok {
		if shouldGlobalQuit(km) {
			return a, tea.Quit
		}
		if keyString(km) == "?" {
			if _, ok := a.page.(*helpPage); ok {
				return a.Update(navBackMsg{})
			}
			return a.Update(navForwardMsg{next: newHelpPage(a.w, a.h)})
		}
	}

	switch msg := msg.(type) {
	case tea.KeyMsg:
		next, c := a.page.Update(msg)
		a.page = next
		return a, c
	default:
		next, c := a.page.Update(msg)
		a.page = next
		return a, c
	}
}

func (a *appModel) View() string {
	line := titleStyle.Render(NavPageName(a.page)) + "  " + subStyle.Render(AppShortcutHints())
	return line + "\n\n" + a.page.View()
}
