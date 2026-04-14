package zhihu

import (
	"fmt"
	"net/url"
)

type CommentItem struct {
	ID                string
	Author            string
	ReplyTo           string // 子评论回复对象；根评论列表为空
	Content           string
	VoteCount         int
	ChildCommentCount int
	Time              int64
	ChildComments     []CommentItem // root_comments 内嵌的子评论（可能少于 child_comment_count）
}

type commentRow struct {
	ID                any          `json:"id"`
	Content           string       `json:"content"`
	VoteCount         int          `json:"vote_count"`
	ChildCommentCount int          `json:"child_comment_count"`
	CreatedTime       int64        `json:"created_time"`
	ChildComments     []commentRow `json:"child_comments"`
	Author            struct {
		Member *struct {
			Name string `json:"name"`
		} `json:"member"`
		Name string `json:"name"`
	} `json:"author"`
	ReplyToAuthor *struct {
		Member *struct {
			Name string `json:"name"`
		} `json:"member"`
		Name string `json:"name"`
	} `json:"reply_to_author"`
}

func authorNameFromRow(r *commentRow) string {
	if r == nil {
		return ""
	}
	name := r.Author.Name
	if r.Author.Member != nil && r.Author.Member.Name != "" {
		name = r.Author.Member.Name
	}
	return name
}

func replyToFromRow(r *commentRow) string {
	if r == nil || r.ReplyToAuthor == nil {
		return ""
	}
	n := r.ReplyToAuthor.Name
	if r.ReplyToAuthor.Member != nil && r.ReplyToAuthor.Member.Name != "" {
		n = r.ReplyToAuthor.Member.Name
	}
	return n
}

func rowToCommentItem(r *commentRow, withReplyTo bool) CommentItem {
	it := CommentItem{
		ID:                idString(r.ID),
		Author:            authorNameFromRow(r),
		Content:           r.Content,
		VoteCount:         r.VoteCount,
		ChildCommentCount: r.ChildCommentCount,
		Time:              r.CreatedTime,
	}
	if withReplyTo {
		it.ReplyTo = replyToFromRow(r)
	}
	return it
}

func commentRowTree(r *commentRow, depth int) CommentItem {
	it := rowToCommentItem(r, depth > 0)
	if len(r.ChildComments) > 0 {
		it.ChildComments = make([]CommentItem, len(r.ChildComments))
		for i := range r.ChildComments {
			it.ChildComments[i] = commentRowTree(&r.ChildComments[i], depth+1)
		}
	}
	return it
}

type rootCommentsAPI struct {
	Data   []commentRow `json:"data"`
	Paging struct {
		IsEnd bool `json:"is_end"`
	} `json:"paging"`
}

// FetchAnswerRootComments fetches one page of top-level comments for an answer（含每条下的子评论预览 child_comments）。
func (c *Client) FetchAnswerRootComments(questionID, answerID string, offset, limit int) ([]CommentItem, bool, error) {
	u := fmt.Sprintf("%s/api/v4/answers/%s/root_comments?offset=%d&limit=%d&order=normal",
		BaseURL, url.PathEscape(answerID), max(0, offset), max(1, limit))
	var raw rootCommentsAPI
	if c.jsonFromCache(u, &raw) {
		return commentsFromRootAPI(&raw)
	}
	if err := c.PrepareAnswerPage(questionID, answerID); err != nil {
		return nil, false, err
	}
	if err := c.getJSON(u, &raw); err != nil {
		return nil, false, err
	}
	return commentsFromRootAPI(&raw)
}

func commentsFromRootAPI(raw *rootCommentsAPI) ([]CommentItem, bool, error) {
	out := make([]CommentItem, 0, len(raw.Data))
	for i := range raw.Data {
		out = append(out, commentRowTree(&raw.Data[i], 0))
	}
	return out, raw.Paging.IsEnd, nil
}
