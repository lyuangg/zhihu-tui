package ui

import (
	"github.com/charmbracelet/bubbles/key"
	"github.com/charmbracelet/bubbles/list"
)

// zhihuListCfg 配置 bubbles/list 的默认 delegate 与列表壳样式。
type zhihuListCfg struct {
	title              string
	showTitle          bool
	itemDesc           bool
	itemHeight         int
	itemSpacing        int
	showStatusBar      bool
	noItemsPaddingLeft bool
	extraShortHelp     func() []key.Binding
}

func newZhihuList(cfg zhihuListCfg) list.Model {
	d := list.NewDefaultDelegate()
	d.ShowDescription = cfg.itemDesc
	d.SetHeight(cfg.itemHeight)
	d.SetSpacing(cfg.itemSpacing)
	l := list.New(nil, d, 0, 0)
	if cfg.title != "" {
		l.Title = cfg.title
	}
	l.SetShowTitle(cfg.showTitle)
	l.SetShowStatusBar(cfg.showStatusBar)
	l.SetStatusBarItemName("条", "条")
	if cfg.extraShortHelp != nil {
		l.AdditionalShortHelpKeys = cfg.extraShortHelp
	}
	if cfg.noItemsPaddingLeft {
		st := l.Styles
		st.NoItems = subStyle.PaddingLeft(2)
		l.Styles = st
	}
	return l
}

func newHotList() list.Model {
	return newZhihuList(zhihuListCfg{
		showTitle:     false,
		itemDesc:      false,
		itemHeight:    1,
		itemSpacing:   0,
		showStatusBar: false,
		extraShortHelp: func() []key.Binding {
			return []key.Binding{
				key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "刷新")),
				key.NewBinding(key.WithKeys("l", "enter", "right"), key.WithHelp("l/enter", "进入问题")),
				key.NewBinding(key.WithKeys("yy"), key.WithHelp("yy", "复制")),
				key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "浏览器打开")),
				key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "外部编辑器")),
			}
		},
	})
}

func newQuestionList() list.Model {
	return newZhihuList(zhihuListCfg{
		title:              "回答列表",
		showTitle:          true,
		itemDesc:           true,
		itemHeight:         2,
		itemSpacing:        0,
		showStatusBar:      false,
		noItemsPaddingLeft: true,
		extraShortHelp: func() []key.Binding {
			return []key.Binding{
				key.NewBinding(key.WithKeys("r"), key.WithHelp("r", "刷新")),
				key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "下页回答")),
				key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "上页回答")),
				key.NewBinding(key.WithKeys("l", "enter", "right"), key.WithHelp("l/enter", "进入回答")),
				key.NewBinding(key.WithKeys("yy"), key.WithHelp("yy", "复制")),
				key.NewBinding(key.WithKeys("o"), key.WithHelp("o", "浏览器打开")),
				key.NewBinding(key.WithKeys("e"), key.WithHelp("e", "外部编辑器")),
			}
		},
	})
}

func newCommentList() list.Model {
	return newZhihuList(zhihuListCfg{
		title:              "评论",
		showTitle:          true,
		itemDesc:           true,
		itemHeight:         2,
		itemSpacing:        0,
		showStatusBar:      false,
		noItemsPaddingLeft: true,
		extraShortHelp: func() []key.Binding {
			return []key.Binding{
				key.NewBinding(key.WithKeys("n"), key.WithHelp("n", "下页评论")),
				key.NewBinding(key.WithKeys("p"), key.WithHelp("p", "上页评论")),
				key.NewBinding(key.WithKeys("l", "enter", "right"), key.WithHelp("l/enter", "阅读评论")),
				key.NewBinding(key.WithKeys("yy"), key.WithHelp("yy", "复制")),
			}
		},
	})
}
