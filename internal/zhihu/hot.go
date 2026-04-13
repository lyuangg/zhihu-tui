package zhihu

import (
	"fmt"
	"net/url"
)

type HotItem struct {
	Rank         int
	Title        string
	Heat         string
	AnswerCount  int
	QuestionID   string
	QuestionURL  string
}

type hotAPI struct {
	Data []struct {
		Target struct {
			Title       string `json:"title"`
			ID          any    `json:"id"` // number or string after fix
			AnswerCount int    `json:"answer_count"`
		} `json:"target"`
		DetailText string `json:"detail_text"`
	} `json:"data"`
}

func idString(v any) string {
	switch t := v.(type) {
	case string:
		return t
	case float64:
		return fmt.Sprintf("%.0f", t)
	case nil:
		return ""
	default:
		return fmt.Sprint(t)
	}
}

// FetchHot returns Zhihu hot list.
func (c *Client) FetchHot(limit int) ([]HotItem, error) {
	u := fmt.Sprintf("%s/api/v3/feed/topstory/hot-lists/total?limit=%d", BaseURL, min(50, max(1, limit)))
	var raw hotAPI
	if c.jsonFromCache(u, &raw) {
		return hotItemsFromRaw(&raw), nil
	}
	if err := c.PrepareHome(); err != nil {
		return nil, err
	}
	if err := c.getJSONHot(u, &raw); err != nil {
		return nil, err
	}
	return hotItemsFromRaw(&raw), nil
}

func hotItemsFromRaw(raw *hotAPI) []HotItem {
	out := make([]HotItem, 0, len(raw.Data))
	for i, item := range raw.Data {
		qid := idString(item.Target.ID)
		out = append(out, HotItem{
			Rank:        i + 1,
			Title:       item.Target.Title,
			Heat:        item.DetailText,
			AnswerCount: item.Target.AnswerCount,
			QuestionID:  qid,
			QuestionURL: "https://www.zhihu.com/question/" + url.PathEscape(qid),
		})
	}
	return out
}
