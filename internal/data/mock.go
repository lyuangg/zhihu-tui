package data

import (
	"fmt"
	"time"

	"github.com/lyuangg/zhihu-tui/internal/zhihu"
)

// MockAPI 固定假数据，用于无浏览器 / 无网络时调试 TUI 布局与交互。
// 不访问 bridge；Prepare*、Invalidate* 为空操作。
type MockAPI struct{}

func NewMock() API {
	return &MockAPI{}
}

func (m *MockAPI) InvalidateHotListCache() {}

func (m *MockAPI) InvalidateQuestionCache(string, []string) {}
func (m *MockAPI) InvalidateSearchCache()                    {}
func (m *MockAPI) FetchAnswerPreview(questionID, answerID string) (string, zhihu.AnswerItem, error) {
	now := time.Now().Unix()
	return "【Mock】问题 · " + questionID, zhihu.AnswerItem{
		ID:           answerID,
		Author:       "MockUser_A",
		Voteup:       999,
		CommentCount: 12,
		CreatedTime:  now,
		ContentHTML:  "<p>这是通过回答链接直接预览的 Mock 正文。</p>",
	}, nil
}

func (m *MockAPI) PrepareQuestion(string) error { return nil }

func (m *MockAPI) Search(query string, offset, limit int) ([]zhihu.SearchItem, error) {
	if offset > 0 {
		return nil, nil
	}
	n := min(max(1, limit), 5)
	out := make([]zhihu.SearchItem, 0, n)
	for i := 0; i < n; i++ {
		qid := fmt.Sprintf("mock-search-q-%d", i+1)
		out = append(out, zhihu.SearchItem{
			Type:       "question",
			Title:      fmt.Sprintf("【Mock 搜索】%s 结果 %d", query, i+1),
			Excerpt:    "这是用于测试搜索页展示的摘要。",
			Author:     "MockUser",
			Voteup:     100 - i*3,
			URL:        zhihu.BaseURL + "/question/" + qid,
			QuestionID: qid,
		})
	}
	return out, nil
}

func (m *MockAPI) FetchHot(limit int) ([]zhihu.HotItem, error) {
	n := min(50, max(1, limit))
	count := min(n, 5)
	out := make([]zhihu.HotItem, 0, count)
	for i := 0; i < count; i++ {
		qid := fmt.Sprintf("mock-q-%d", i+1)
		out = append(out, zhihu.HotItem{
			Rank:        i + 1,
			Title:       fmt.Sprintf("【Mock】示例问题标题 %d（TUI 测试）", i+1),
			Heat:        fmt.Sprintf("%d 万热度", 100-i*10),
			AnswerCount: 42 + i,
			QuestionID:  qid,
			QuestionURL: zhihu.BaseURL + "/question/" + qid,
		})
	}
	return out, nil
}

func (m *MockAPI) FetchQuestionPage(questionID string, offset, limit int) (string, []zhihu.AnswerItem, bool, int, error) {
	title := "【Mock】问题 · " + questionID
	if offset > 0 {
		return title, nil, true, 2, nil
	}
	now := time.Now().Unix()
	answers := []zhihu.AnswerItem{
		{
			ID:           "mock-ans-1",
			Author:       "MockUser_A",
			Voteup:       1204,
			CommentCount: 88,
			CreatedTime:  now,
			ContentHTML: `<p>这是 <strong>Mock</strong> 回答正文第一段，用于测试 HTML 转终端。</p>
<p>第二段包含<code>代码</code>与列表：</p>
<ul><li>条目一</li><li>条目二</li></ul>`,
		},
		{
			ID:           "mock-ans-2",
			Author:       "MockUser_B",
			Voteup:       56,
			CommentCount: 3,
			CreatedTime:  now - 3600,
			ContentHTML:  `<p>另一条较短 mock 回答。</p>`,
		},
	}
	if limit < len(answers) {
		answers = answers[:limit]
	}
	return title, answers, true, 2, nil
}

func (m *MockAPI) FetchAnswerRootComments(questionID, answerID string, offset, limit int) ([]zhihu.CommentItem, bool, error) {
	if offset > 0 {
		return nil, true, nil
	}
	now := time.Now().Unix()
	return []zhihu.CommentItem{
		{
			ID:      "c1",
			Author:  "路人甲",
			Content: `<p>Mock 评论：<b>赞</b>一下界面。</p>`,
			Likes:   12,
			Time:    now,
		},
		{
			ID:      "c2",
			Author:  "路人乙",
			Content: `<p>第二条评论，测试多行。</p>`,
			Likes:   3,
			Time:    now - 120,
		},
	}, true, nil
}
