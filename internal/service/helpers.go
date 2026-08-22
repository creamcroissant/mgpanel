// 文件路径: internal/service/helpers.go
// 模块说明: 这是 internal 模块里的 helpers 逻辑，下面的注释会用非常通俗的中文帮你理解每一步。
package service

func max64(a, b int64) int64 {
	if a > b {
		return a
	}
	return b
}
