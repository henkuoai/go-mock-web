package server

import (
	"net/http"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-mock-web/internal"
)

// dispatchMock 是 NoRoute 兜底处理器：匹配已启用的 mock 并返回其响应。
// Gin 只在没有任何已注册路由匹配时才进入 NoRoute，因此管理接口（已注册）
// 不会被这里处理；无需对 /api/ 前缀做特殊拦截。
func (s *Server) dispatchMock(c *gin.Context) {
	path := c.Request.URL.Path
	method := c.Request.Method

	mocks := s.store.AllEnabled()
	hit := internal.MatchMock(mocks, method, path)
	if hit == nil {
		errorJSON(c, http.StatusNotFound, "no mock matched")
		return
	}
	m := hit.Mock

	// 模拟延迟
	if m.Delay > 0 {
		time.Sleep(time.Duration(m.Delay) * time.Millisecond)
	}

	// 构建模板上下文并渲染响应体
	data := internal.BuildRequestData(c.Request, hit.Param)
	body, err := internal.RenderBody(m.Body, data)
	if err != nil {
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}

	// 写入自定义响应头
	hasContentType := false
	for k, v := range m.Headers {
		c.Header(k, v)
		if strings.EqualFold(k, "Content-Type") {
			hasContentType = true
		}
	}
	// 未显式指定时默认按 JSON 返回
	if !hasContentType {
		c.Header("Content-Type", "application/json; charset=utf-8")
	}

	c.Status(m.Status)
	if _, err := c.Writer.WriteString(body); err != nil {
		// 写入失败通常因客户端断开，记录但不再处理
		_ = err
	}
}
