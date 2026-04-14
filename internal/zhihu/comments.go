package zhihu

import (
	"fmt"
	"net/url"
)

type CommentItem struct {
	ID                string
	Author            string
	Content           string
	VoteCount         int
	ChildCommentCount int
	Time              int64
}

type rootCommentsAPI struct {
	Data []struct {
		ID                any    `json:"id"`
		Content           string `json:"content"`
		VoteCount         int    `json:"vote_count"`
		ChildCommentCount int    `json:"child_comment_count"`
		CreatedTime       int64  `json:"created_time"`
		Author            struct {
			Member *struct {
				Name string `json:"name"`
			} `json:"member"`
			Name string `json:"name"`
		} `json:"author"`
	} `json:"data"`
	Paging struct {
		IsEnd bool `json:"is_end"`
	} `json:"paging"`
}

// FetchAnswerRootComments fetches one page of top-level comments for an answer.
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
	for _, x := range raw.Data {
		name := x.Author.Name
		if x.Author.Member != nil && x.Author.Member.Name != "" {
			name = x.Author.Member.Name
		}
		out = append(out, CommentItem{
			ID:                idString(x.ID),
			Author:            name,
			Content:           x.Content,
			VoteCount:         x.VoteCount,
			ChildCommentCount: x.ChildCommentCount,
			Time:              x.CreatedTime,
		})
	}
	return out, raw.Paging.IsEnd, nil
}
