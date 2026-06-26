package internal

import "strings"

// MatchResult 表示一次路径匹配的结果。
type MatchResult struct {
	Mock  *Mock
	Param map[string]string
}

// segments 将路径按 '/' 切分为非空段。
func segments(path string) []string {
	parts := strings.Split(path, "/")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}

// matchPattern 尝试用单条 mock 的模式段匹配请求段，
// 成功返回提取的路径参数，失败返回 nil。
// 段类型：":param" 匹配单段并捕获；"*" 匹配剩余所有段；其余为字面量。
func matchPattern(pattern, request []string) map[string]string {
	if len(pattern) == 0 && len(request) == 0 {
		return map[string]string{}
	}
	params := map[string]string{}
	i := 0
	for i < len(pattern) {
		seg := pattern[i]
		switch {
		case seg == "*":
			// 通配剩余全部，无需捕获
			return params
		case strings.HasPrefix(seg, ":"):
			// 参数段需要对应一个请求段
			if i >= len(request) {
				return nil
			}
			params[seg[1:]] = request[i]
			i++
			continue
		default:
			if i >= len(request) || seg != request[i] {
				return nil
			}
			i++
			continue
		}
	}
	// 模式已耗尽，请求也必须恰好耗尽
	if i != len(request) {
		return nil
	}
	return params
}

// specificity 返回模式的特异性分值，用于多命中时排序取最优。
// 字面量段权重最高，参数段次之，通配符最低。
func specificity(pattern []string) (literal int, param int) {
	for _, seg := range pattern {
		switch {
		case seg == "*":
			// 通配符不贡献字面量/参数分
		case strings.HasPrefix(seg, ":"):
			param++
		default:
			literal++
		}
	}
	return
}

// MatchMock 在候选列表中找出与给定 method+path 匹配的最优 mock。
func MatchMock(mocks []*Mock, method, path string) *MatchResult {
	req := segments(path)
	var best *MatchResult
	bestLit, bestParam := -1, -1

	for _, m := range mocks {
		if m.Method != method {
			continue
		}
		pat := segments(m.Path)
		params := matchPattern(pat, req)
		if params == nil {
			continue
		}
		lit, param := specificity(pat)
		// 取字面量多者；并列时取参数多者；再并列取首个（保持稳定）
		if best == nil ||
			lit > bestLit ||
			(lit == bestLit && param > bestParam) {
			best = &MatchResult{Mock: m, Param: params}
			bestLit, bestParam = lit, param
		}
	}
	return best
}
