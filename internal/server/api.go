package server

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"go-mock-web/internal"
)

// ---- 项目处理器 ----

// listProjects GET /api/projects
func (s *Server) listProjects(c *gin.Context) {
	projects := s.store.ListProjects()
	// 附带每个项目的接口数量
	type projectWithCount struct {
		*internal.Project
		MockCount int `json:"mockCount"`
	}
	out := make([]projectWithCount, 0, len(projects))
	for _, p := range projects {
		out = append(out, projectWithCount{
			Project:   p,
			MockCount: s.store.MockCount(p.ID),
		})
	}
	c.JSON(http.StatusOK, gin.H{"data": out})
}

// getProject GET /api/projects/:id
func (s *Server) getProject(c *gin.Context) {
	id := c.Param("id")
	p, err := s.store.GetProject(id)
	if err != nil {
		errorJSON(c, http.StatusNotFound, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": p})
}

// createProject POST /api/projects
func (s *Server) createProject(c *gin.Context) {
	var p internal.Project
	if err := c.ShouldBindJSON(&p); err != nil {
		errorJSON(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	created, err := s.store.CreateProject(&p)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, err.Error())
		return
	}
	c.JSON(http.StatusCreated, gin.H{"data": created})
}

// updateProject PUT /api/projects/:id
func (s *Server) updateProject(c *gin.Context) {
	id := c.Param("id")
	var p internal.Project
	if err := c.ShouldBindJSON(&p); err != nil {
		errorJSON(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	updated, err := s.store.UpdateProject(id, &p)
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

// deleteProject DELETE /api/projects/:id —— 级联删除项目下所有 mock
func (s *Server) deleteProject(c *gin.Context) {
	id := c.Param("id")
	if err := s.store.DeleteProject(id); err != nil {
		if err == internal.ErrNotFound {
			errorJSON(c, http.StatusNotFound, err.Error())
			return
		}
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}

// importSwagger POST /api/projects/:id/import  body: {"spec": "...", "enabled": true}
func (s *Server) importSwagger(c *gin.Context) {
	id := c.Param("id")
	if _, err := s.store.GetProject(id); err != nil {
		errorJSON(c, http.StatusNotFound, "project not found")
		return
	}
	var req struct {
		Spec    string `json:"spec"`
		Enabled bool   `json:"enabled"`
	}
	if err := c.ShouldBindJSON(&req); err != nil {
		errorJSON(c, http.StatusBadRequest, "invalid request body: "+err.Error())
		return
	}
	mocks, err := internal.ImportSwagger(req.Spec, id, req.Enabled)
	if err != nil {
		errorJSON(c, http.StatusBadRequest, "swagger import failed: "+err.Error())
		return
	}
	created := s.store.CreateMocks(mocks)
	c.JSON(http.StatusCreated, gin.H{"data": created, "count": len(created)})
}

// ---- Mock 处理器 ----

// listMocks GET /api/mocks?projectId=&q=&method=
func (s *Server) listMocks(c *gin.Context) {
	projectID := c.Query("projectId")
	q := c.Query("q")
	method := c.Query("method")
	list := s.store.ListMocks(projectID, q, method)
	c.JSON(http.StatusOK, gin.H{"data": list})
}

// getMock GET /api/mocks/:id
func (s *Server) getMock(c *gin.Context) {
	id := c.Param("id")
	m, err := s.store.GetMock(id)
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
	created, err := s.store.CreateMock(&m)
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
	updated, err := s.store.UpdateMock(id, &m)
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
	if err := s.store.DeleteMock(id); err != nil {
		if err == internal.ErrNotFound {
			errorJSON(c, http.StatusNotFound, err.Error())
			return
		}
		errorJSON(c, http.StatusInternalServerError, err.Error())
		return
	}
	c.JSON(http.StatusOK, gin.H{"data": gin.H{"id": id}})
}
