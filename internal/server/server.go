// Package server 组装 Gin 路由，提供管理 API 与 mock 请求分发。
package server

import (
	"path/filepath"

	"github.com/gin-gonic/gin"

	"go-mock-web/internal"
)

// Server 持有所有依赖，负责路由注册。
type Server struct {
	store  *internal.Store
	webDir string
}

// New 创建 Server 实例。
func New(store *internal.Store, webDir string) *Server {
	return &Server{store: store, webDir: webDir}
}

// Engine 组装并返回 Gin 引擎。
func (s *Server) Engine() *gin.Engine {
	r := gin.New()
	r.Use(gin.Logger(), gin.Recovery())

	// 项目管理 API
	projects := r.Group("/api/projects")
	{
		projects.GET("", s.listProjects)
		projects.POST("", s.createProject)
		projects.GET("/:id", s.getProject)
		projects.PUT("/:id", s.updateProject)
		projects.DELETE("/:id", s.deleteProject)
		projects.POST("/:id/import", s.importSwagger)
	}

	// Mock 管理 API
	mocks := r.Group("/api/mocks")
	{
		mocks.GET("", s.listMocks)
		mocks.POST("", s.createMock)
		mocks.GET("/:id", s.getMock)
		mocks.PUT("/:id", s.updateMock)
		mocks.DELETE("/:id", s.deleteMock)
	}

	// 首页即管理 UI
	r.GET("/", s.serveIndex)

	// 一切未匹配路由都交由 mock 分发器处理
	r.NoRoute(s.dispatchMock)
	return r
}

// serveIndex 返回前端单页应用。
func (s *Server) serveIndex(c *gin.Context) {
	c.File(filepath.Join(s.webDir, "index.html"))
}

// errorJSON 返回统一格式的错误响应。
func errorJSON(c *gin.Context, status int, msg string) {
	c.JSON(status, gin.H{"error": msg})
}
