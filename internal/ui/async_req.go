package ui

import "sync/atomic"

var asyncReqSeq atomic.Uint64

// newAsyncReqID 为一次异步拉取生成唯一 ID；用于丢弃乱序或过期的 tea.Msg。
func newAsyncReqID() uint64 {
	return asyncReqSeq.Add(1)
}
