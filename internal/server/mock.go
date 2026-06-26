package server

import (
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/gin-gonic/gin"

	"go-mock-web/internal"
)

// dispatchMock 是 NoRoute 兜底处理器：匹配已启用的 mock 并返回其响应。
// 保留前缀 /api/ 永不当作 mock 处理，避免误吞管理接口。
func (s *Server) dispatchMock(c *gin.Context) {
	path := c.Request.URL.Path
	method := c.Request.Method

	if strings.HasPrefix(path, "/api/") {
		errorJSON(c, http.StatusNotFound, "api not found")
		return
	}

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

// 保留以避免未使用导入告警（strconv 用于未来扩展按字符串配置延迟等）。
var _ = strconv.Atoi
