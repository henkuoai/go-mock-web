package internal

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

// ErrNotFound 表示按 ID 未找到对应 mock 或项目。
var ErrNotFound = errors.New("not found")

// 旧数据文件名，用于自动迁移到新的信封格式。
const legacyFile = "mocks.json"

// storeData 是磁盘上的信封格式。
type storeData struct {
	Projects []*Project `json:"projects"`
	Mocks    []*Mock    `json:"mocks"`
}

// Store 负责项目与 mock 数据的持久化加载与增删改查，并发安全。
type Store struct {
	mu       sync.RWMutex
	filePath string
	projects map[string]*Project
	mocks    map[string]*Mock
}

// NewStore 创建并初始化存储；优先加载信封文件，否则尝试迁移旧文件。
func NewStore(filePath string) (*Store, error) {
	s := &Store{
		filePath: filePath,
		projects: map[string]*Project{},
		mocks:    map[string]*Mock{},
	}
	if err := s.loadOrMigrate(); err != nil {
		return nil, err
	}
	return s, nil
}

// loadOrMigrate 加载信封文件；若不存在则尝试从旧 mocks.json 迁移。
func (s *Store) loadOrMigrate() error {
	b, err := os.ReadFile(s.filePath)
	if err == nil {
		return s.unmarshal(b)
	}
	if !os.IsNotExist(err) {
		return err
	}
	// 信封文件不存在，尝试迁移旧文件
	dir := filepath.Dir(s.filePath)
	legacy := filepath.Join(dir, legacyFile)
	lb, lerr := os.ReadFile(legacy)
	if lerr != nil {
		return nil // 旧文件也不存在，视为空存储
	}
	var oldMocks []*Mock
	if err := json.Unmarshal(lb, &oldMocks); err != nil {
		return err
	}
	// 创建默认项目并归入所有旧 mock
	now := time.Now()
	project := &Project{
		ID:          newID(),
		Name:        "默认项目",
		Description: "由旧数据自动迁移",
		CreatedAt:   now,
		UpdatedAt:   now,
	}
	s.projects[project.ID] = project
	for _, m := range oldMocks {
		m.ProjectID = project.ID
		s.mocks[m.ID] = m
	}
	return s.save()
}

// unmarshal 将信封字节解析进内存 map。
func (s *Store) unmarshal(b []byte) error {
	var data storeData
	if err := json.Unmarshal(b, &data); err != nil {
		return err
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	for _, p := range data.Projects {
		s.projects[p.ID] = p
	}
	for _, m := range data.Mocks {
		s.mocks[m.ID] = m
	}
	return nil
}

// save 将内存持久化到磁盘，临时文件 + rename 原子写入。
func (s *Store) save() error {
	if err := os.MkdirAll(filepath.Dir(s.filePath), 0o755); err != nil {
		return err
	}
	data := storeData{Projects: s.snapshotProjects(), Mocks: s.snapshotMocks()}
	b, err := json.MarshalIndent(data, "", "  ")
	if err != nil {
		return err
	}
	tmp, err := os.CreateTemp(filepath.Dir(s.filePath), "store-*.tmp")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)
	if _, err := tmp.Write(b); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, s.filePath)
}

// snapshotProjects 返回按更新时间倒序的项目列表副本。
func (s *Store) snapshotProjects() []*Project {
	list := make([]*Project, 0, len(s.projects))
	for _, p := range s.projects {
		list = append(list, p)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list
}

// snapshotMocks 返回按更新时间倒序的 mock 列表副本。
func (s *Store) snapshotMocks() []*Mock {
	list := make([]*Mock, 0, len(s.mocks))
	for _, m := range s.mocks {
		list = append(list, m)
	}
	sort.Slice(list, func(i, j int) bool {
		return list[i].UpdatedAt.After(list[j].UpdatedAt)
	})
	return list
}

// ---- 项目 CRUD ----

// ListProjects 返回所有项目（按更新时间倒序）。
func (s *Store) ListProjects() []*Project {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.snapshotProjects()
}

// GetProject 按 ID 返回单个项目。
func (s *Store) GetProject(id string) (*Project, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	p, ok := s.projects[id]
	if !ok {
		return nil, ErrNotFound
	}
	return p, nil
}

// CreateProject 校验并新增一个项目，返回新增对象。
func (s *Store) CreateProject(p *Project) (*Project, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	p.FillDefaults()

	s.mu.Lock()
	s.projects[p.ID] = p
	s.mu.Unlock()

	if err := s.save(); err != nil {
		return nil, err
	}
	return p, nil
}

// UpdateProject 用传入字段覆盖已有项目；ID、CreatedAt 保持不变。
func (s *Store) UpdateProject(id string, p *Project) (*Project, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}

	s.mu.Lock()
	existing, ok := s.projects[id]
	if !ok {
		s.mu.Unlock()
		return nil, ErrNotFound
	}
	p.ID = existing.ID
	p.CreatedAt = existing.CreatedAt
	p.NormalizeBeforeUpdate()
	s.projects[id] = p
	s.mu.Unlock()

	if err := s.save(); err != nil {
		return nil, err
	}
	return p, nil
}

// DeleteProject 删除项目及其下所有 mock（级联）。
func (s *Store) DeleteProject(id string) error {
	s.mu.Lock()
	if _, ok := s.projects[id]; !ok {
		s.mu.Unlock()
		return ErrNotFound
	}
	delete(s.projects, id)
	// 级联删除该项目下的 mock
	for mid, m := range s.mocks {
		if m.ProjectID == id {
			delete(s.mocks, mid)
		}
	}
	s.mu.Unlock()

	return s.save()
}

// MockCount 返回指定项目下的 mock 数量。
func (s *Store) MockCount(projectID string) int {
	s.mu.RLock()
	defer s.mu.RUnlock()
	n := 0
	for _, m := range s.mocks {
		if m.ProjectID == projectID {
			n++
		}
	}
	return n
}

// ---- Mock CRUD ----

// ListMocks 返回 mock 列表；projectID 为空时返回全部，否则按项目过滤。
// 可选按关键字 query 与方法 method 进一步过滤。
func (s *Store) ListMocks(projectID, query, method string) []*Mock {
	s.mu.RLock()
	defer s.mu.RUnlock()
	q := strings.ToLower(strings.TrimSpace(query))
	method = strings.ToUpper(strings.TrimSpace(method))

	list := s.snapshotMocks()
	if projectID == "" && q == "" && method == "" {
		return list
	}

	filtered := list[:0]
	for _, m := range list {
		if projectID != "" && m.ProjectID != projectID {
			continue
		}
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

// GetMock 按 ID 返回单个 mock。
func (s *Store) GetMock(id string) (*Mock, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	m, ok := s.mocks[id]
	if !ok {
		return nil, ErrNotFound
	}
	return m, nil
}

// CreateMock 校验并新增一条 mock，返回新增对象。
func (s *Store) CreateMock(m *Mock) (*Mock, error) {
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

// CreateMocks 批量新增 mock，用于 swagger 导入。任一校验失败则跳过该条。
func (s *Store) CreateMocks(mocks []*Mock) []*Mock {
	created := make([]*Mock, 0, len(mocks))
	s.mu.Lock()
	for _, m := range mocks {
		m.FillDefaults()
		s.mocks[m.ID] = m
		created = append(created, m)
	}
	s.mu.Unlock()
	_ = s.save()
	return created
}

// UpdateMock 用传入字段覆盖已有 mock；ID、CreatedAt、ProjectID 保持不变。
func (s *Store) UpdateMock(id string, m *Mock) (*Mock, error) {
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
	m.ProjectID = existing.ProjectID
	m.CreatedAt = existing.CreatedAt
	m.NormalizeBeforeUpdate()
	s.mocks[id] = m
	s.mu.Unlock()

	if err := s.save(); err != nil {
		return nil, err
	}
	return m, nil
}

// DeleteMock 按 ID 删除一条 mock。
func (s *Store) DeleteMock(id string) error {
	s.mu.Lock()
	if _, ok := s.mocks[id]; !ok {
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
