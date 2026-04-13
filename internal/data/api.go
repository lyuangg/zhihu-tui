package data

import "github.com/lyuangg/zhihu-tui/internal/zhihu"

// API 是 UI 使用的数据访问抽象：真实环境由 *zhihu.Client 实现，测试 TUI 可用 MockAPI。
type API interface {
	FetchHot(limit int) ([]zhihu.HotItem, error)
	InvalidateHotListCache()
	Search(query string, offset, limit int) ([]zhihu.SearchItem, error)
	InvalidateSearchCache()
	FetchAnswerPreview(questionID, answerID string) (questionTitle string, answer zhihu.AnswerItem, err error)

	PrepareQuestion(questionID string) error
	FetchQuestionPage(questionID string, offset, limit int) (title string, answers []zhihu.AnswerItem, isEnd bool, total int, err error)
	InvalidateQuestionCache(questionID string, answerIDs []string)

	FetchAnswerRootComments(questionID, answerID string, offset, limit int) ([]zhihu.CommentItem, bool, error)
}
