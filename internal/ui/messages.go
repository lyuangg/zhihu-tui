package ui

import "github.com/lyuangg/zhihu-tui/internal/zhihu"

type hotDone struct {
	items []zhihu.HotItem
	err   error
}

type recommendDone struct {
	items []zhihu.RecommendItem
	err   error
}

type recommendAnswerDone struct {
	qid    string
	qTitle string
	ans    *zhihu.AnswerItem
	err    error
}

type searchDone struct {
	query  string
	offset int
	items  []zhihu.SearchItem
	err    error
}

type searchLinkDone struct {
	qid    string
	qTitle string
	ans    *zhihu.AnswerItem
	err    error
}

type qDone struct {
	reqID        uint64
	title        string
	answers      []zhihu.AnswerItem
	total        int
	isEnd        bool
	err          error
	reloadRetain bool
	prevAnsIdx   int
}

type ansDone struct {
	reqID    uint64
	md       string
	comments []zhihu.CommentItem
	cEnd     bool
	err      error
}

type commentDone struct {
	reqID  uint64
	items  []zhihu.CommentItem
	isEnd  bool
	err    error
	offset int
}

type editorDoneMsg struct {
	err error
}

type articleDone struct {
	item zhihu.ArticleItem
	err  error
}
