package internal

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// MethodGet 等常量定义了 Mock 接口支持的 HTTP 方法。
const (
	MethodGet     = "GET"
	MethodPost    = "POST"
	MethodPut     = "PUT"
	MethodDelete  = "DELETE"
	MethodPatch   = "PATCH"
	MethodHead    = "HEAD"
	MethodOptions = "OPTIONS"
)

// ValidMethods 是允许配置的 HTTP 方法集合。
var ValidMethods = map[string]bool{
	MethodGet:     true,
	MethodPost:    true,
	MethodPut:     true,
	MethodDelete:  true,
	MethodPatch:   true,
	MethodHead:    true,
	MethodOptions: true,
}

// Mock 描述一条 mock 接口的完整定义。
type Mock struct {
	ID          string            `json:"id"`
	Name        string            `json:"name"`
	Method      string            `json:"method"`
	Path        string            `json:"path"`
	Description string            `json:"description"`
	Status      int               `json:"status"`
	Headers     map[string]string `json:"headers"`
	Body        string            `json:"body"`
	Delay       int               `json:"delay"`
	Enabled     bool              `json:"enabled"`
	CreatedAt   time.Time         `json:"createdAt"`
	UpdatedAt   time.Time         `json:"updatedAt"`
}

// Validate 校验 mock 字段是否合法，返回第一个不合法原因。
func (m *Mock) Validate() error {
	if strings.TrimSpace(m.Name) == "" {
		return errors.New("name is required")
	}
	if !ValidMethods[strings.ToUpper(m.Method)] {
		return errors.New("invalid method: " + m.Method)
	}
	if !strings.HasPrefix(m.Path, "/") {
		return errors.New("path must start with '/'")
	}
	if m.Status == 0 {
		m.Status = 200
	}
	if m.Headers == nil {
		m.Headers = map[string]string{}
	}
	return nil
}

// FillDefaults 在新建 mock 时填充 ID 与时间戳，并归一化方法大小写。
func (m *Mock) FillDefaults() {
	m.ID = newID()
	m.Method = strings.ToUpper(m.Method)
	if m.Status == 0 {
		m.Status = 200
	}
	if m.Headers == nil {
		m.Headers = map[string]string{}
	}
	now := time.Now()
	m.CreatedAt = now
	m.UpdatedAt = now
}

// NormalizeBeforeUpdate 在更新前归一化可归一化的字段。
func (m *Mock) NormalizeBeforeUpdate() {
	m.Method = strings.ToUpper(m.Method)
	if m.Status == 0 {
		m.Status = 200
	}
	if m.Headers == nil {
		m.Headers = map[string]string{}
	}
	m.UpdatedAt = time.Now()
}

// newID 生成 8 字节的十六进制随机 ID。
func newID() string {
	b := make([]byte, 8)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}
