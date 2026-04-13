package ui

import (
	"regexp"
	"strings"
	"time"

	"github.com/charmbracelet/x/ansi"
)

var ansiStrip = regexp.MustCompile(`\x1b\[[0-?]*[ -/]*[@-~]`)

func stripANSI(s string) string {
	return ansiStrip.ReplaceAllString(s, "")
}

// normalizeCRLF 统一换行为 \n。
func normalizeCRLF(s string) string {
	return strings.ReplaceAll(s, "\r\n", "\n")
}

var (
	// 块级/换行标签：在剥除其它标签前先变成换行，避免多段 <p> 粘成一行。
	reHTMLBr       = regexp.MustCompile(`(?i)<br\s*/?>`)
	reHTMLPclose   = regexp.MustCompile(`(?i)</p>`)
	reHTMLPopen    = regexp.MustCompile(`(?i)<p[^>]*>`)
	reHTMLDivClose = regexp.MustCompile(`(?i)</div>`)
	reHTMLDivOpen  = regexp.MustCompile(`(?i)<div[^>]*>`)
	reHTMLUlOpen   = regexp.MustCompile(`(?i)<ul[^>]*>`)
	reHTMLUlClose  = regexp.MustCompile(`(?i)</ul>`)
	reHTMLOlOpen   = regexp.MustCompile(`(?i)<ol[^>]*>`)
	reHTMLOlClose  = regexp.MustCompile(`(?i)</ol>`)
	reHTMLLiOpen   = regexp.MustCompile(`(?i)<li[^>]*>`)
	reHTMLLiClose  = regexp.MustCompile(`(?i)</li>`)
	reHTMLImg      = regexp.MustCompile(`(?is)<img[^>]*>`)
	reHTMLAnchor   = regexp.MustCompile(`(?is)<a\b[^>]*>(.*?)</a>`)
	reAnchorHref   = regexp.MustCompile(`(?i)\bhref\s*=\s*["']([^"']+)["']`)
	reImgDataOrig  = regexp.MustCompile(`(?i)data-original\s*=\s*["']([^"']+)["']`)
	reImgSrc       = regexp.MustCompile(`(?i)\bsrc\s*=\s*["']([^"']+)["']`)
)

// normalizeHTMLBlockBreaks 将段落、换行等块边界转为换行，便于剥 tag 后仍保留分段。
func normalizeHTMLBlockBreaks(s string) string {
	s = normalizeCRLF(s)
	s = reHTMLBr.ReplaceAllString(s, "\n")
	s = reHTMLPclose.ReplaceAllString(s, "\n")
	s = reHTMLPopen.ReplaceAllString(s, "\n")
	s = reHTMLDivClose.ReplaceAllString(s, "\n")
	s = reHTMLDivOpen.ReplaceAllString(s, "\n")
	s = reHTMLUlOpen.ReplaceAllString(s, "\n")
	s = reHTMLUlClose.ReplaceAllString(s, "\n")
	s = reHTMLOlOpen.ReplaceAllString(s, "\n")
	s = reHTMLOlClose.ReplaceAllString(s, "\n")
	s = reHTMLLiOpen.ReplaceAllString(s, "\n- ")
	s = reHTMLLiClose.ReplaceAllString(s, "\n")
	for strings.Contains(s, "\n\n\n") {
		s = strings.ReplaceAll(s, "\n\n\n", "\n\n")
	}
	return s
}

// replaceImgsWithURLLines 将每个 <img> 替换为单独一行图片 URL（知乎常用 data-original）。
func replaceImgsWithURLLines(html string) string {
	return reHTMLImg.ReplaceAllStringFunc(html, func(tag string) string {
		var url string
		if m := reImgDataOrig.FindStringSubmatch(tag); len(m) > 1 {
			url = strings.TrimSpace(m[1])
		}
		if url == "" {
			if m := reImgSrc.FindStringSubmatch(tag); len(m) > 1 {
				url = strings.TrimSpace(m[1])
			}
		}
		if url == "" {
			return "\n（图片）\n"
		}
		return "\n" + url + "\n"
	})
}

// replaceAnchorsWithURLs 将 <a> 链接替换为“文本 (URL)”或单独 URL，避免 strip 后丢失链接地址。
func replaceAnchorsWithURLs(html string) string {
	return reHTMLAnchor.ReplaceAllStringFunc(html, func(tag string) string {
		href := ""
		if m := reAnchorHref.FindStringSubmatch(tag); len(m) > 1 {
			href = strings.TrimSpace(m[1])
		}
		text := ""
		if m := reHTMLAnchor.FindStringSubmatch(tag); len(m) > 1 {
			text = strings.TrimSpace(stripHTMLFallback(m[1]))
		}
		switch {
		case text != "" && href != "":
			return text + " (" + href + ")"
		case href != "":
			return href
		default:
			return text
		}
	})
}

func termWrapWidth(termCols int) int {
	w := termCols - 4
	if w < 40 {
		return 40
	}
	return w
}

func htmlToPlainCore(html string) string {
	s := replaceImgsWithURLLines(html)
	s = replaceAnchorsWithURLs(s)
	s = stripHTMLFallback(s)
	return strings.TrimSpace(s)
}

func termHardWrap(s string, width int) string {
	if width < 8 {
		width = 8
	}
	s = normalizeCRLF(s)
	lines := strings.Split(s, "\n")
	var b strings.Builder
	for i, line := range lines {
		if i > 0 {
			b.WriteByte('\n')
		}
		b.WriteString(ansi.Hardwrap(line, width, true))
	}
	return b.String()
}

// HTMLToTerminalMarkdown 将知乎回答/评论 HTML 转为终端纯文本（不再做 Markdown 渲染）。
// 图片以单独一行 URL 展示；段落通过 <p> 等已规范为换行。
func HTMLToTerminalMarkdown(html string, termCols int) string {
	return termHardWrap(htmlToPlainCore(html), termWrapWidth(termCols))
}

func stripHTMLFallback(s string) string {
	s = normalizeHTMLBlockBreaks(s)
	re := regexp.MustCompile(`(?s)<script.*?</script>|<style.*?</style>|<[^>]+>`)
	t := re.ReplaceAllString(s, "")
	t = strings.NewReplacer("&nbsp;", " ", "&lt;", "<", "&gt;", ">", "&amp;", "&").Replace(t)
	return strings.TrimSpace(t)
}

// collapseText joins Fields to remove leading/trailing/multiple whitespace (fixes Zhihu HTML → odd leading newlines).
func collapseText(s string) string {
	return strings.Join(strings.Fields(s), " ")
}

// StripHTMLShort strips tags for one-line previews; collapses whitespace.
func StripHTMLShort(s string, max int) string {
	t := collapseText(stripHTMLFallback(s))
	runes := []rune(t)
	if len(runes) > max {
		return string(runes[:max]) + "…"
	}
	return t
}

// formatCommentTime 将知乎 created_time（秒或毫秒时间戳）格式化为本地时间；无效则返回空串（不展示）。
func formatCommentTime(t int64) string {
	if t <= 0 {
		return ""
	}
	var tm time.Time
	if t > 9999999999 {
		tm = time.UnixMilli(t)
	} else {
		tm = time.Unix(t, 0)
	}
	return tm.Local().Format("2006-01-02 15:04")
}
