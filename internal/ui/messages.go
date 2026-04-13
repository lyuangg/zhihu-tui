package ui

import "github.com/lyuangg/zhihu-tui/internal/zhihu"

type hotDone struct {
	items []zhihu.HotItem
	err   error
}

type qDone struct {
	title        string
	answers      []zhihu.AnswerItem
	total        int
	isEnd        bool
	err          error
	reloadRetain bool
	prevAnsIdx   int
}

type ansDone struct {
	md       string
	comments []zhihu.CommentItem
	cEnd     bool
	err      error
}

type commentDone struct {
	items  []zhihu.CommentItem
	isEnd  bool
	err    error
	offset int
}

type editorDoneMsg struct {
	err error
}
