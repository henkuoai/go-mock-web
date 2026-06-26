package server

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"

	"go-mock-web/internal"
)

// listMocks GET /api/mocks —— 支持 ?q= 与 ?method= 过滤。
func (s *Server) listMocks(c *gin.Context) {
	q := c.Query("q")
	method := c.Query("method")
	list := s.store.List(q, method)
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// getMock GET /api/mocks/:id
func (s *Server) getMock(c *gin.Context) {
	id := c.Param("id")
	m, err := s.store.Get(id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": m})
}

// createMock POST /api/mocks
func (s *Server) createMock(c *gin.Context) {
	var m internal.Mock
	if err := c.ShouldBindJSON(&m); err != nil {
		errorJSON(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	created, err := s.store.Create(&m)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

// updateMock PUT /api/mocks/:id
func (s *Server) updateMock(c *gin.Context) {
	id := c.Param("id")
	var m internal.Mock
	if err := c.ShouldBindJSON(&m); err != nil {
		errorJSON(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	updated, err := s.store.Update(id, &m)
	if err != nil {
		if err == internal.ErrNotFound {
			errorJSON(c, http.StatusNotFound, err.Error())
			return
		}
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": updated})
}

// deleteMock DELETE /api/mocks/:id
func (s *Server) deleteMock(c *gin.Context) {
	id := c.Param("id")
	if err := s.store.Delete(id); err != nil {
		if err == internal.ErrNotFound {
			errorJSON(c, http.StatusNotFound, err.Error())
			return
		}
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

// parseStatus 解析状态码字符串，便于测试工具复用。
func parseStatus(s string) int {
	s = strings.TrimSpace(s)
	if s == "" {
		return 200
	}
	n, err := strconv.Atoi(s)
	if err != nil || n < 100 || n > 599 {
		return 200
	}
	return n
}
