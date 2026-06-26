package internal

import (
	"bytes"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"text/template"
)

// RequestData 是响应模板可访问的上下文结构。
//
// 模板字段示例：
//   - {{.Method}}              请求方法
//   - {{.Path.id}}             路径参数 id
//   - {{.Query.page}}          查询参数 page
//   - {{.Body.name}}           请求体 JSON 字段 name
//   - {{.Body.user.id}}        请求体 JSON 嵌套字段
//   - {{.Form.field}}          表单字段
//   - {{.RawBody}}             原始请求体字符串
//   - {{index .Header "X-Token"}}  请求头（键含特殊字符用 index）
type RequestData struct {
	Method  string
	Path    map[string]string
	Query   map[string]string
	Header  http.Header
	Body    map[string]any
	Form    map[string]string
	RawBody string
}

// BuildRequestData 从 http.Request 与已匹配的路径参数构建模板上下文。
func BuildRequestData(r *http.Request, pathParams map[string]string) *RequestData {
	raw, _ := io.ReadAll(r.Body)
	r.Body = nil // 已读取，后续不再使用

	data := &RequestData{
		Method:  r.Method,
		Path:    pathParams,
		Query:   map[string]string{},
		Header:  r.Header,
		Body:    map[string]any{},
		Form:    map[string]string{},
		RawBody: string(raw),
	}
	for k, v := range r.URL.Query() {
		data.Query[k] = v[0]
	}
	// 尝试将请求体解析为 JSON，失败则忽略（可能是表单或纯文本）
	if len(raw) > 0 {
		_ = json.Unmarshal(raw, &data.Body)
	}
	// 表单字段
	if err := r.ParseForm(); err == nil {
		for k, v := range r.Form {
			data.Form[k] = v[0]
		}
	}
	return data
}

// RenderBody 用模板上下文渲染 mock 响应体；模板语法错误时返回错误。
func RenderBody(body string, data *RequestData) (string, error) {
	if !strings.Contains(body, "{{") {
		return body, nil
	}
	tmpl, err := template.New("mock").Parse(body)
	if err != nil {
		return "", errors.New("invalid response template: " + err.Error())
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return "", errors.New("template render failed: " + err.Error())
	}
	return buf.String(), nil
}
