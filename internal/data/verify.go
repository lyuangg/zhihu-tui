package data

import "github.com/lyuangg/zhihu-tui/internal/zhihu"

// 编译期断言：*zhihu.Client 实现 API。
var _ API = (*zhihu.Client)(nil)
