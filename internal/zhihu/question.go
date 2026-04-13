package zhihu

import (
	"fmt"
	"net/url"
)

type AnswerItem struct {
	ID           string
	Author       string
	Voteup       int
	CommentCount int
	ContentHTML  string
	// CreatedTime 创建时间（Unix 秒或毫秒）；API 未返回则为 0。
	CreatedTime int64
}

// answersAPI 与 clis/zhihu/question.ts 使用的 answers 接口一致；可含 question 标题（用于展示，避免再请求易 403 的 questions?include=title）。
type answersAPI struct {
	Data []struct {
		ID           any    `json:"id"`
		Content      string `json:"content"`
		VoteupCount  int    `json:"voteup_count"`
		CommentCount int    `json:"comment_count"`
		CreatedTime  int64  `json:"created_time"`
		Author       struct {
			Name string `json:"name"`
		} `json:"author"`
		Question *struct {
			Title string `json:"title"`
		} `json:"question"`
	} `json:"data"`
	Paging struct {
		IsEnd  bool `json:"is_end"`
		Totals int  `json:"totals"`
	} `json:"paging"`
	Question *struct {
		Title string `json:"title"`
	} `json:"question"`
}

// FetchQuestionPage 仅请求 answers 接口（同一 URL、同一 include），
// 不再单独请求 /api/v4/questions/{id}?include=title（该接口在自动化环境下易 403）。
func (c *Client) FetchQuestionPage(questionID string, offset, limit int) (title string, answers []AnswerItem, isEnd bool, total int, err error) {
	// 请求基础字段，并请求 created_time 供列表展示（无则 JSON 为 0）。
	inc := "data[*].content,voteup_count,comment_count,author,created_time"
	u := fmt.Sprintf("%s/api/v4/questions/%s/answers?limit=%d&offset=%d&sort_by=default&include=%s",
		BaseURL, url.PathEscape(questionID), max(1, limit), max(0, offset), url.QueryEscape(inc))
	var raw answersAPI
	if c.jsonFromCache(u, &raw) {
		return pageFromAnswersAPI(questionID, &raw)
	}
	if err := c.PrepareQuestion(questionID); err != nil {
		return "", nil, false, 0, err
	}
	if err := c.getJSON(u, &raw); err != nil {
		return "", nil, false, 0, err
	}
	return pageFromAnswersAPI(questionID, &raw)
}

func pageFromAnswersAPI(questionID string, raw *answersAPI) (title string, answers []AnswerItem, isEnd bool, total int, err error) {
	title = resolveQuestionTitle(questionID, raw)
	out := make([]AnswerItem, 0, len(raw.Data))
	for _, a := range raw.Data {
		out = append(out, AnswerItem{
			ID:           idString(a.ID),
			Author:       a.Author.Name,
			Voteup:       a.VoteupCount,
			CommentCount: a.CommentCount,
			ContentHTML:  a.Content,
			CreatedTime:  a.CreatedTime,
		})
	}
	return title, out, raw.Paging.IsEnd, raw.Paging.Totals, nil
}

func resolveQuestionTitle(questionID string, raw *answersAPI) string {
	if raw.Question != nil && raw.Question.Title != "" {
		return raw.Question.Title
	}
	for _, a := range raw.Data {
		if a.Question != nil && a.Question.Title != "" {
			return a.Question.Title
		}
	}
	return "问题 " + questionID
}
