package ui

import (
	"fmt"
	"strings"

	"github.com/lyuangg/zhihu-tui/internal/zhihu"
)

func listTitleOneLine(s string) string {
	s = normalizeCRLF(s)
	s = strings.ReplaceAll(s, "\n", " ")
	return strings.TrimSpace(s)
}

type hotListItem struct {
	it zhihu.HotItem
}

func (i hotListItem) FilterValue() string {
	return i.it.Title + " " + strings.TrimSpace(i.it.Heat)
}

func (i hotListItem) Title() string {
	t := listTitleOneLine(strings.TrimSpace(i.it.Title))
	h := listTitleOneLine(strings.TrimSpace(i.it.Heat))
	if h != "" {
		return fmt.Sprintf("%2d  %s  ·  %s", i.it.Rank, t, h)
	}
	return fmt.Sprintf("%2d  %s", i.it.Rank, t)
}

func (i hotListItem) Description() string { return "" }

type ansListItem struct {
	a    zhihu.AnswerItem
	disp int
}

func (i ansListItem) FilterValue() string {
	p := StripHTMLShort(i.a.ContentHTML, 2000)
	return i.a.Author + " " + p
}

func (i ansListItem) Title() string {
	line := fmt.Sprintf("%2d  ▲ %d  %s", i.disp, i.a.Voteup, listTitleOneLine(i.a.Author))
	if ts := formatCommentTime(i.a.CreatedTime); ts != "" {
		return fmt.Sprintf("%s  ·  %s", line, ts)
	}
	return line
}

func (i ansListItem) Description() string {
	return listTitleOneLine(StripHTMLShort(i.a.ContentHTML, 2000))
}

type commentListItem struct {
	c zhihu.CommentItem
}

func (i commentListItem) FilterValue() string {
	return i.c.Author + " " + stripHTMLFallback(i.c.Content)
}

func (i commentListItem) Title() string {
	au := listTitleOneLine(i.c.Author)
	var meta string
	if i.c.ReplyTo != "" {
		meta = fmt.Sprintf("→ %s · ▲ %d", listTitleOneLine(i.c.ReplyTo), i.c.VoteCount)
	} else {
		meta = fmt.Sprintf("▲ %d · ↳ %d", i.c.VoteCount, i.c.ChildCommentCount)
	}
	if ts := formatCommentTime(i.c.Time); ts != "" {
		return fmt.Sprintf("%s · %s · %s", ts, au, meta)
	}
	return fmt.Sprintf("%s · %s", au, meta)
}

func (i commentListItem) Description() string {
	return listTitleOneLine(collapseText(stripHTMLFallback(i.c.Content)))
}

type searchListItem struct {
	it zhihu.SearchItem
}

func (i searchListItem) FilterValue() string {
	return strings.TrimSpace(i.it.Title + " " + i.it.Excerpt + " " + i.it.Author + " " + i.it.Type)
}

func (i searchListItem) Title() string {
	t := listTitleOneLine(strings.TrimSpace(i.it.Title))
	typ := listTitleOneLine(strings.TrimSpace(i.it.Type))
	if typ != "" {
		return fmt.Sprintf("%s  ·  %s", t, typ)
	}
	return t
}

func (i searchListItem) Description() string {
	desc := listTitleOneLine(strings.TrimSpace(i.it.Excerpt))
	author := listTitleOneLine(strings.TrimSpace(i.it.Author))
	if author == "" {
		return desc
	}
	if i.it.Voteup > 0 {
		return fmt.Sprintf("%s  ·  %s  ·  ▲ %d", author, desc, i.it.Voteup)
	}
	return fmt.Sprintf("%s  ·  %s", author, desc)
}

type recommendListItem struct {
	it zhihu.RecommendItem
}

func (i recommendListItem) FilterValue() string {
	return strings.TrimSpace(i.it.Title + " " + i.it.Excerpt + " " + i.it.Author + " " + i.it.Type)
}

func (i recommendListItem) Title() string {
	t := listTitleOneLine(strings.TrimSpace(i.it.Title))
	typ := listTitleOneLine(strings.TrimSpace(i.it.Type))
	if typ != "" {
		return fmt.Sprintf("%s  ·  %s", t, typ)
	}
	return t
}

func (i recommendListItem) Description() string {
	desc := listTitleOneLine(strings.TrimSpace(i.it.Excerpt))
	author := listTitleOneLine(strings.TrimSpace(i.it.Author))
	if author == "" {
		return desc
	}
	if i.it.Voteup > 0 {
		return fmt.Sprintf("%s  ·  %s  ·  ▲ %d", author, desc, i.it.Voteup)
	}
	return fmt.Sprintf("%s  ·  %s", author, desc)
}
