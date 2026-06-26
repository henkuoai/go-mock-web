package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
)

// ErrNotFound 表示按 ID 未找到对应 mock。
var ErrNotFound = errors.New("mock not found")

// Store 负责 mock 数据的持久化加载与增删改查，并发安全。
type Store struct {
	mu       sync.RWMutex
	filePath string
	mocks    map[string]*Mock
}

// NewStore 创建并初始化存储；若数据文件存在则加载，否则使用空集合。
func NewStore(filePath string) (*Store, error) {
	s := &Store{
		filePath: filePath,
		mocks:    map[string]*Mock{},
	}
	if err := s.load(); err != nil {
		return nil, err
	}
	return s, nil
}

// load 从磁盘加载数据文件到内存。
func (s *Store) load() error {
	b, err := os.ReadFile(s.filePath)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	var list []*Mock
	if err := json.Unmarshal(b, &list); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, m := range list {
		s.mocks[m.ID] = m
	}
	return nil
}

// save 将内存中的 mock 持久化到磁盘，采用临时文件 + rename 的原子写入。
func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return err
	}
	list := s.snapshot()
	b, err := json.MarshalIndent(list, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.filePath), "mocks-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName) // rename 成功后临时文件已不存在，删除是 no-op
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.filePath)
}

// snapshot 返回按更新时间倒序排列的 mock 列表副本。
func (s *Store) snapshot() []*Mock {
	list := make([]*Mock, 0, len(s.mocks))
	for _, m := range s.mocks {
		list = append(list, m)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list
}

// List 返回所有 mock（按更新时间倒序），可选按关键字与方法过滤。
func (s *Store) List(query, method string) []*Mock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	method = strings.ToUpper(strings.TrimSpace(method))

	list := s.snapshot()
	if q == "" && method == "" {
		return list
	}

	filtered := list[:0]
	for _, m := range list {
		if method != "" && m.Method != method {
			continue
		}
		if q != "" {
			hay := strings.ToLower(m.Name + " " + m.Path + " " + m.Description)
			if !strings.Contains(hay, q) {
				continue
			}
		}
		filtered = append(filtered, m)
	}
	return filtered
}

// Get 按 ID 返回单个 mock。
func (s *Store) Get(id string) (*Mock, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.mocks[id]
	if !ok {
		return nil, ErrNotFound
	}
	return m, nil
}

// Create 校验并新增一条 mock，返回新增对象。
func (s *Store) Create(m *Mock) (*Mock, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}
	m.FillDefaults()

	s.mu.Lock()
	s.mocks[m.ID] = m
	s.mu.Unlock()

	if err := s.save(); err != nil {
		return nil, err
	}
	return m, nil
}

// Update 用传入字段覆盖已有 mock；ID、CreatedAt 保持不变。
func (s *Store) Update(id string, m *Mock) (*Mock, error) {
	if err := m.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	existing, ok := s.mocks[id]
	if !ok {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	m.ID = existing.ID
	m.CreatedAt = existing.CreatedAt
	m.NormalizeBeforeUpdate()
	s.mocks[id] = m
	s.mu.Unlock()

	if err := s.save(); err != nil {
		return nil, err
	}
	return m, nil
}

// Delete 按 ID 删除一条 mock。
func (s *Store) Delete(id string) error {
	s.mu.Lock()
	_, ok := s.mocks[id]
	if !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	delete(s.mocks, id)
	s.mu.Unlock()

	return s.save()
}

// AllEnabled 返回所有已启用的 mock，供分发器匹配使用。
func (s *Store) AllEnabled() []*Mock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	list := make([]*Mock, 0, len(s.mocks))
	for _, m := range s.mocks {
		if m.Enabled {
			list = append(list, m)
		}
	}
	return list
}
