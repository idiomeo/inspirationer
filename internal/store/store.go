// Package store 负责把灵感数据以 JSON 形式持久化到本地磁盘。
package store

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"

	"inspirationer/internal/i18n"
	"inspirationer/internal/model"
)

// ErrNotFound 表示目标不存在。
var ErrNotFound = errors.New("not found")

// Filter 是灵感列表的查询条件。
type Filter struct {
	Query      string   `json:"query"`
	Mode       string   `json:"mode"` // title | content | all
	CategoryID string   `json:"categoryId"`
	TagIDs     []string `json:"tagIds"`
	Archive    string   `json:"archive"` // "" = 未归档 | only | all
	Sort       string   `json:"sort"`    // updated | created | title
	Limit      int      `json:"limit"`
}

// Store 是带读写锁的内存数据 + 磁盘持久化。
type Store struct {
	mu         sync.RWMutex
	dir        string
	snippets   []model.Snippet
	tags       []model.Tag
	categories []model.Category
	settings   model.Settings

	// tag/category 自增取色游标
	paletteCursor int
}

// New 打开（或初始化）数据目录。
func New(dir string) (*Store, error) {
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return nil, i18n.Errorf("err.dataDir", err)
	}
	s := &Store{dir: dir, settings: model.DefaultSettings()}

	if err := s.loadJSON("snippets.json", &s.snippets); err != nil {
		return nil, err
	}
	if err := s.loadJSON("tags.json", &s.tags); err != nil {
		return nil, err
	}
	if err := s.loadJSON("categories.json", &s.categories); err != nil {
		return nil, err
	}
	if err := s.loadJSON("settings.json", &s.settings); err != nil {
		return nil, err
	}
	s.settings.Normalize()
	s.paletteCursor = len(s.tags) + len(s.categories)

	if s.snippets == nil {
		s.snippets = []model.Snippet{}
	}
	if s.tags == nil {
		s.tags = []model.Tag{}
	}
	if s.categories == nil {
		s.categories = []model.Category{}
	}
	// 首次运行时落盘，确保文件存在
	if err := s.saveAll(); err != nil {
		return nil, err
	}
	return s, nil
}

// Dir 返回数据目录。
func (s *Store) Dir() string { return s.dir }

func (s *Store) path(name string) string { return filepath.Join(s.dir, name) }

func (s *Store) loadJSON(name string, dst interface{}) error {
	b, err := os.ReadFile(s.path(name))
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return i18n.Errorf("err.readFile", name, err)
	}
	if len(strings.TrimSpace(string(b))) == 0 {
		return nil
	}
	if err := json.Unmarshal(b, dst); err != nil {
		// 尝试从本地快照恢复
		if rec := s.recoverFromSnapshot(name, dst); rec {
			return nil
		}
		return i18n.Errorf("err.parseFile", name, err)
	}
	return nil
}

// recoverFromSnapshot 在文件损坏时尝试用最近的本地快照恢复。
func (s *Store) recoverFromSnapshot(name string, dst interface{}) bool {
	entries, err := os.ReadDir(filepath.Join(s.dir, "backups"))
	if err != nil {
		return false
	}
	names := []string{}
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
			names = append(names, e.Name())
		}
	}
	sort.Sort(sort.Reverse(sort.StringSlice(names)))
	for _, n := range names {
		b, err := os.ReadFile(filepath.Join(s.dir, "backups", n))
		if err != nil {
			continue
		}
		var bk model.Backup
		if json.Unmarshal(b, &bk) != nil {
			continue
		}
		switch name {
		case "snippets.json":
			*(dst.(*[]model.Snippet)) = bk.Snippets
		case "tags.json":
			*(dst.(*[]model.Tag)) = bk.Tags
		case "categories.json":
			*(dst.(*[]model.Category)) = bk.Categories
		case "settings.json":
			if bk.Settings != nil {
				*(dst.(*model.Settings)) = *bk.Settings
			}
		default:
			return false
		}
		return true
	}
	return false
}

func writeFileAtomic(path string, data []byte) error {
	tmp := path + ".tmp"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

func (s *Store) saveJSON(name string, v interface{}) error {
	b, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		return err
	}
	return writeFileAtomic(s.path(name), b)
}

// saveAll 调用方必须已持有写锁（或处于初始化阶段）。
func (s *Store) saveAll() error {
	if err := s.saveJSON("snippets.json", s.snippets); err != nil {
		return err
	}
	if err := s.saveJSON("tags.json", s.tags); err != nil {
		return err
	}
	if err := s.saveJSON("categories.json", s.categories); err != nil {
		return err
	}
	return s.saveJSON("settings.json", s.settings)
}

func newID(prefix string) string {
	var b [5]byte
	_, _ = rand.Read(b[:])
	return fmt.Sprintf("%s%s%s", prefix, time.Now().UTC().Format("20060102150405"), hex.EncodeToString(b[:]))
}

func nextColor(cursor int) string {
	return model.Palette[((cursor%len(model.Palette))+len(model.Palette))%len(model.Palette)]
}

// ---------------------------------------------------------------- snippets

// ListSnippets 按条件查询灵感。
func (s *Store) ListSnippets(f Filter) []model.Snippet {
	s.mu.RLock()
	defer s.mu.RUnlock()

	terms := strings.Fields(strings.ToLower(f.Query))
	out := make([]model.Snippet, 0, len(s.snippets))
	for _, sn := range s.snippets {
		if !f.matchArchive(sn) {
			continue
		}
		if f.CategoryID != "" {
			if f.CategoryID == "__none__" {
				if sn.CategoryID != "" {
					continue
				}
			} else if sn.CategoryID != f.CategoryID {
				continue
			}
		}
		if len(f.TagIDs) > 0 && !hasAllTags(sn, f.TagIDs) {
			continue
		}
		if len(terms) > 0 && !s.matchTerms(sn, terms, f.Mode) {
			continue
		}
		out = append(out, sn)
	}
	sortSnippets(out, f.Sort)
	if f.Limit > 0 && len(out) > f.Limit {
		out = out[:f.Limit]
	}
	return out
}

func (f Filter) matchArchive(sn model.Snippet) bool {
	switch f.Archive {
	case "all":
		return true
	case "only":
		return sn.Archived
	default:
		return !sn.Archived
	}
}

func hasAllTags(sn model.Snippet, ids []string) bool {
	for _, want := range ids {
		found := false
		for _, have := range sn.Tags {
			if have == want {
				found = true
				break
			}
		}
		if !found {
			return false
		}
	}
	return true
}

// matchTerms 调用方需持有锁；把 query 按空格切分后要求全部命中。
func (s *Store) matchTerms(sn model.Snippet, terms []string, mode string) bool {
	title := strings.ToLower(sn.Title)
	content := strings.ToLower(sn.Content)
	extra := ""
	if mode != "title" && mode != "content" {
		extra = strings.ToLower(s.tagNamesLocked(sn.Tags) + " " + s.categoryNameLocked(sn.CategoryID))
	}
	for _, t := range terms {
		ok := false
		switch mode {
		case "title":
			ok = strings.Contains(title, t)
		case "content":
			ok = strings.Contains(content, t)
		default:
			ok = strings.Contains(title, t) || strings.Contains(content, t) || strings.Contains(extra, t)
		}
		if !ok {
			return false
		}
	}
	return true
}

func (s *Store) tagNamesLocked(ids []string) string {
	parts := make([]string, 0, len(ids))
	for _, id := range ids {
		for _, t := range s.tags {
			if t.ID == id {
				parts = append(parts, t.Name)
				break
			}
		}
	}
	return strings.Join(parts, " ")
}

func (s *Store) categoryNameLocked(id string) string {
	for _, c := range s.categories {
		if c.ID == id {
			return c.Name
		}
	}
	return ""
}

func sortSnippets(list []model.Snippet, mode string) {
	sort.SliceStable(list, func(i, j int) bool {
		a, b := list[i], list[j]
		if a.Pinned != b.Pinned {
			return a.Pinned
		}
		switch mode {
		case "created":
			return a.CreatedAt.After(b.CreatedAt)
		case "title":
			return strings.ToLower(a.Title) < strings.ToLower(b.Title)
		default:
			return a.UpdatedAt.After(b.UpdatedAt)
		}
	})
}

// GetSnippet 取单条灵感。
func (s *Store) GetSnippet(id string) (model.Snippet, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	for _, sn := range s.snippets {
		if sn.ID == id {
			return sn, nil
		}
	}
	return model.Snippet{}, ErrNotFound
}

// CreateSnippet 新建灵感（ID/时间戳由存储层生成）。
func (s *Store) CreateSnippet(sn model.Snippet) (model.Snippet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now()
	sn.ID = newID("sn_")
	sn.CreatedAt = now
	sn.UpdatedAt = now
	if sn.TitleSource == "" {
		sn.TitleSource = model.TitleSourceUser
	}
	sn.Tags = s.dedupeTagIDsLocked(sn.Tags)
	s.snippets = append(s.snippets, sn)
	if err := s.saveJSON("snippets.json", s.snippets); err != nil {
		return sn, err
	}
	return sn, nil
}

// UpdateSnippet 整体替换一条灵感的可编辑字段。
func (s *Store) UpdateSnippet(id string, in model.Snippet) (model.Snippet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.snippets {
		if s.snippets[i].ID != id {
			continue
		}
		cur := &s.snippets[i]
		cur.Title = in.Title
		cur.Content = in.Content
		cur.Tags = s.dedupeTagIDsLocked(in.Tags)
		cur.CategoryID = in.CategoryID
		cur.Pinned = in.Pinned
		cur.Archived = in.Archived
		if in.TitleSource != "" {
			cur.TitleSource = in.TitleSource
		}
		cur.UpdatedAt = time.Now()
		if err := s.saveJSON("snippets.json", s.snippets); err != nil {
			return *cur, err
		}
		return *cur, nil
	}
	return model.Snippet{}, ErrNotFound
}

// MutateSnippet 对单条灵感做局部修改。
func (s *Store) MutateSnippet(id string, fn func(*model.Snippet)) (model.Snippet, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.snippets {
		if s.snippets[i].ID != id {
			continue
		}
		cur := &s.snippets[i]
		fn(cur)
		cur.Tags = s.dedupeTagIDsLocked(cur.Tags)
		cur.UpdatedAt = time.Now()
		if err := s.saveJSON("snippets.json", s.snippets); err != nil {
			return *cur, err
		}
		return *cur, nil
	}
	return model.Snippet{}, ErrNotFound
}

// DeleteSnippet 删除一条灵感。
func (s *Store) DeleteSnippet(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.snippets {
		if s.snippets[i].ID == id {
			s.snippets = append(s.snippets[:i], s.snippets[i+1:]...)
			return s.saveJSON("snippets.json", s.snippets)
		}
	}
	return ErrNotFound
}

// DeleteSnippets 批量删除。
func (s *Store) DeleteSnippets(ids []string) (int, error) {
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	kept := s.snippets[:0:0]
	removed := 0
	for _, sn := range s.snippets {
		if set[sn.ID] {
			removed++
			continue
		}
		kept = append(kept, sn)
	}
	s.snippets = kept
	return removed, s.saveJSON("snippets.json", s.snippets)
}

// BulkAssign 批量设置分类 / 增删标签。
func (s *Store) BulkAssign(ids []string, categoryID *string, addTags, removeTags []string) (int, error) {
	set := map[string]bool{}
	for _, id := range ids {
		set[id] = true
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	n := 0
	for i := range s.snippets {
		if !set[s.snippets[i].ID] {
			continue
		}
		cur := &s.snippets[i]
		if categoryID != nil {
			cur.CategoryID = *categoryID
		}
		for _, t := range addTags {
			cur.Tags = append(cur.Tags, t)
		}
		if len(removeTags) > 0 {
			rm := map[string]bool{}
			for _, t := range removeTags {
				rm[t] = true
			}
			kept := cur.Tags[:0:0]
			for _, t := range cur.Tags {
				if !rm[t] {
					kept = append(kept, t)
				}
			}
			cur.Tags = kept
		}
		cur.Tags = s.dedupeTagIDsLocked(cur.Tags)
		cur.UpdatedAt = time.Now()
		n++
	}
	if n == 0 {
		return 0, nil
	}
	return n, s.saveJSON("snippets.json", s.snippets)
}

// dedupeTagIDsLocked 去重；调用方需持有锁。
func (s *Store) dedupeTagIDsLocked(ids []string) []string {
	seen := map[string]bool{}
	out := make([]string, 0, len(ids))
	for _, id := range ids {
		if id == "" || seen[id] {
			continue
		}
		seen[id] = true
		out = append(out, id)
	}
	if out == nil {
		out = []string{}
	}
	return out
}

// ---------------------------------------------------------------- tags

// Tags 返回全部标签（按名称排序）。
func (s *Store) Tags() []model.Tag {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]model.Tag{}, s.tags...)
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

// FindTagByName 忽略大小写按名称查找。
func (s *Store) FindTagByName(name string) (model.Tag, bool) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.findTagByNameLocked(name)
}

func (s *Store) findTagByNameLocked(name string) (model.Tag, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, t := range s.tags {
		if strings.ToLower(t.Name) == n {
			return t, true
		}
	}
	return model.Tag{}, false
}

// CreateTag 新建标签。
func (s *Store) CreateTag(name, color string) (model.Tag, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Tag{}, i18n.Errorf("err.emptyTagName")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.findTagByNameLocked(name); ok {
		return t, nil
	}
	if color == "" {
		color = nextColor(s.paletteCursor)
		s.paletteCursor++
	}
	t := model.Tag{ID: newID("tag_"), Name: name, Color: color}
	s.tags = append(s.tags, t)
	return t, s.saveJSON("tags.json", s.tags)
}

// EnsureTag 按名称取或建标签。
func (s *Store) EnsureTag(name string) (model.Tag, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Tag{}, false, i18n.Errorf("err.emptyTagName")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if t, ok := s.findTagByNameLocked(name); ok {
		return t, false, nil
	}
	color := nextColor(s.paletteCursor)
	s.paletteCursor++
	t := model.Tag{ID: newID("tag_"), Name: name, Color: color}
	s.tags = append(s.tags, t)
	return t, true, s.saveJSON("tags.json", s.tags)
}

// UpdateTag 改名/改色。
func (s *Store) UpdateTag(id, name, color string) (model.Tag, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.tags {
		if s.tags[i].ID != id {
			continue
		}
		if strings.TrimSpace(name) != "" {
			s.tags[i].Name = strings.TrimSpace(name)
		}
		if color != "" {
			s.tags[i].Color = color
		}
		return s.tags[i], s.saveJSON("tags.json", s.tags)
	}
	return model.Tag{}, ErrNotFound
}

// DeleteTag 删除标签并从所有灵感上摘除。
func (s *Store) DeleteTag(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for i := range s.tags {
		if s.tags[i].ID == id {
			s.tags = append(s.tags[:i], s.tags[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	for i := range s.snippets {
		kept := s.snippets[i].Tags[:0:0]
		for _, t := range s.snippets[i].Tags {
			if t != id {
				kept = append(kept, t)
			}
		}
		s.snippets[i].Tags = kept
	}
	return s.saveAll()
}

// ---------------------------------------------------------------- categories

// Categories 返回全部分类（按名称排序）。
func (s *Store) Categories() []model.Category {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := append([]model.Category{}, s.categories...)
	sort.Slice(out, func(i, j int) bool {
		return strings.ToLower(out[i].Name) < strings.ToLower(out[j].Name)
	})
	return out
}

func (s *Store) findCategoryByNameLocked(name string) (model.Category, bool) {
	n := strings.ToLower(strings.TrimSpace(name))
	for _, c := range s.categories {
		if strings.ToLower(c.Name) == n {
			return c, true
		}
	}
	return model.Category{}, false
}

// CreateCategory 新建分类。
func (s *Store) CreateCategory(name, color string) (model.Category, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Category{}, i18n.Errorf("err.emptyCategoryName")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.findCategoryByNameLocked(name); ok {
		return c, nil
	}
	if color == "" {
		color = nextColor(s.paletteCursor)
		s.paletteCursor++
	}
	c := model.Category{ID: newID("cat_"), Name: name, Color: color}
	s.categories = append(s.categories, c)
	return c, s.saveJSON("categories.json", s.categories)
}

// EnsureCategory 按名称取或建分类。
func (s *Store) EnsureCategory(name string) (model.Category, bool, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return model.Category{}, false, i18n.Errorf("err.emptyCategoryName")
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	if c, ok := s.findCategoryByNameLocked(name); ok {
		return c, false, nil
	}
	color := nextColor(s.paletteCursor)
	s.paletteCursor++
	c := model.Category{ID: newID("cat_"), Name: name, Color: color}
	s.categories = append(s.categories, c)
	return c, true, s.saveJSON("categories.json", s.categories)
}

// UpdateCategory 改名/改色。
func (s *Store) UpdateCategory(id, name, color string) (model.Category, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for i := range s.categories {
		if s.categories[i].ID != id {
			continue
		}
		if strings.TrimSpace(name) != "" {
			s.categories[i].Name = strings.TrimSpace(name)
		}
		if color != "" {
			s.categories[i].Color = color
		}
		return s.categories[i], s.saveJSON("categories.json", s.categories)
	}
	return model.Category{}, ErrNotFound
}

// DeleteCategory 删除分类，相关灵感的分类置空。
func (s *Store) DeleteCategory(id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	found := false
	for i := range s.categories {
		if s.categories[i].ID == id {
			s.categories = append(s.categories[:i], s.categories[i+1:]...)
			found = true
			break
		}
	}
	if !found {
		return ErrNotFound
	}
	for i := range s.snippets {
		if s.snippets[i].CategoryID == id {
			s.snippets[i].CategoryID = ""
		}
	}
	return s.saveAll()
}

// ---------------------------------------------------------------- settings

// Settings 返回设置副本。
func (s *Store) Settings() model.Settings {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.settings
}

// UpdateSettings 覆盖设置。
func (s *Store) UpdateSettings(in model.Settings) (model.Settings, error) {
	in.Normalize()
	s.mu.Lock()
	defer s.mu.Unlock()
	s.settings = in
	return s.settings, s.saveJSON("settings.json", s.settings)
}

// PatchWebDAVStatus 记录备份结果。
func (s *Store) PatchWebDAVStatus(when time.Time, status string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if !when.IsZero() {
		s.settings.WebDAV.LastBackup = when
	}
	s.settings.WebDAV.LastStatus = status
	return s.saveJSON("settings.json", s.settings)
}

// ---------------------------------------------------------------- stats

// Stats 用于侧栏计数。
type Stats struct {
	Snippets      int            `json:"snippets"`
	Archived      int            `json:"archived"`
	Pinned        int            `json:"pinned"`
	Tags          int            `json:"tags"`
	Categories    int            `json:"categories"`
	ByCategory    map[string]int `json:"byCategory"`
	ByTag         map[string]int `json:"byTag"`
	Uncategorized int            `json:"uncategorized"`
}

// Stats 统计各分类/标签下的灵感数量。
func (s *Store) Stats() Stats {
	s.mu.RLock()
	defer s.mu.RUnlock()
	st := Stats{
		Tags:       len(s.tags),
		Categories: len(s.categories),
		ByCategory: map[string]int{},
		ByTag:      map[string]int{},
	}
	for _, sn := range s.snippets {
		if sn.Archived {
			st.Archived++
			continue
		}
		st.Snippets++
		if sn.Pinned {
			st.Pinned++
		}
		if sn.CategoryID == "" {
			st.Uncategorized++
		} else {
			st.ByCategory[sn.CategoryID]++
		}
		for _, t := range sn.Tags {
			st.ByTag[t]++
		}
	}
	return st
}

// ---------------------------------------------------------------- backup

// Export 生成备份数据。
func (s *Store) Export(includeSettings bool) model.Backup {
	s.mu.RLock()
	defer s.mu.RUnlock()
	bk := model.Backup{
		Version:    1,
		App:        "inspirationer",
		ExportedAt: time.Now(),
		Snippets:   append([]model.Snippet{}, s.snippets...),
		Tags:       append([]model.Tag{}, s.tags...),
		Categories: append([]model.Category{}, s.categories...),
	}
	if includeSettings {
		st := s.settings
		bk.Settings = &st
	}
	return bk
}

// ImportMode 导入策略。
type ImportMode string

const (
	ImportReplace ImportMode = "replace"
	ImportMerge   ImportMode = "merge"
)

// Import 导入备份。
func (s *Store) Import(bk model.Backup, mode ImportMode) (map[string]int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	res := map[string]int{"snippets": 0, "tags": 0, "categories": 0}

	if mode == ImportReplace {
		s.snippets = []model.Snippet{}
		s.tags = []model.Tag{}
		s.categories = []model.Category{}
	}

	// 标签：按名称合并
	nameToTag := map[string]string{}
	for _, t := range s.tags {
		nameToTag[strings.ToLower(t.Name)] = t.ID
	}
	for _, t := range bk.Tags {
		key := strings.ToLower(strings.TrimSpace(t.Name))
		if key == "" {
			continue
		}
		if _, ok := nameToTag[key]; ok {
			continue
		}
		if t.ID == "" || s.hasIDLocked(s.tags, t.ID) {
			t.ID = newID("tag_")
		}
		if t.Color == "" {
			t.Color = nextColor(s.paletteCursor)
			s.paletteCursor++
		}
		s.tags = append(s.tags, t)
		nameToTag[key] = t.ID
		res["tags"]++
	}

	// 分类
	nameToCat := map[string]string{}
	idMap := map[string]string{}
	for _, c := range s.categories {
		nameToCat[strings.ToLower(c.Name)] = c.ID
	}
	for _, c := range bk.Categories {
		key := strings.ToLower(strings.TrimSpace(c.Name))
		if key == "" {
			continue
		}
		if exist, ok := nameToCat[key]; ok {
			idMap[c.ID] = exist
			continue
		}
		old := c.ID
		if c.ID == "" || s.hasIDLocked(s.categories, c.ID) {
			c.ID = newID("cat_")
		}
		if c.Color == "" {
			c.Color = nextColor(s.paletteCursor)
			s.paletteCursor++
		}
		s.categories = append(s.categories, c)
		nameToCat[key] = c.ID
		idMap[old] = c.ID
		res["categories"]++
	}

	existing := map[string]int{}
	for i, sn := range s.snippets {
		existing[sn.ID] = i
	}
	for _, sn := range bk.Snippets {
		// 重映射标签/分类 ID
		tags := []string{}
		for _, tid := range sn.Tags {
			if mapped, ok := nameToTag[strings.ToLower(s.tagNameByID(bk.Tags, tid))]; ok {
				tags = append(tags, mapped)
			} else if s.hasIDLocked(s.tags, tid) {
				tags = append(tags, tid)
			}
		}
		sn.Tags = tags
		if mapped, ok := idMap[sn.CategoryID]; ok {
			sn.CategoryID = mapped
		} else if sn.CategoryID != "" && !s.hasIDLocked(s.categories, sn.CategoryID) {
			sn.CategoryID = ""
		}

		if idx, ok := existing[sn.ID]; ok {
			s.snippets[idx] = sn
			continue
		}
		if sn.ID == "" {
			sn.ID = newID("sn_")
		}
		if sn.CreatedAt.IsZero() {
			sn.CreatedAt = time.Now()
		}
		if sn.UpdatedAt.IsZero() {
			sn.UpdatedAt = sn.CreatedAt
		}
		s.snippets = append(s.snippets, sn)
		res["snippets"]++
	}

	if bk.Settings != nil {
		st := *bk.Settings
		st.Normalize()
		// 备份里可能没有的本地状态一并更新
		s.settings = st
	}
	return res, s.saveAll()
}

func (s *Store) hasIDLocked(list interface{}, id string) bool {
	switch v := list.(type) {
	case []model.Tag:
		for _, x := range v {
			if x.ID == id {
				return true
			}
		}
	case []model.Category:
		for _, x := range v {
			if x.ID == id {
				return true
			}
		}
	}
	return false
}

func (s *Store) tagNameByID(list []model.Tag, id string) string {
	for _, t := range list {
		if t.ID == id {
			return t.Name
		}
	}
	return ""
}

// Snapshot 在 data/backups 下写一份本地快照，保留最近 keep 份。
func (s *Store) Snapshot(keep int) (string, error) {
	bk := s.Export(true)
	b, err := json.MarshalIndent(bk, "", "  ")
	if err != nil {
		return "", err
	}
	dir := filepath.Join(s.dir, "backups")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", err
	}
	name := fmt.Sprintf("snapshot-%s.json", time.Now().Format("20060102-150405"))
	if err := writeFileAtomic(filepath.Join(dir, name), b); err != nil {
		return "", err
	}
	entries, err := os.ReadDir(dir)
	if err == nil {
		names := []string{}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				names = append(names, e.Name())
			}
		}
		sort.Sort(sort.Reverse(sort.StringSlice(names)))
		if keep <= 0 {
			keep = 20
		}
		for i := keep; i < len(names); i++ {
			_ = os.Remove(filepath.Join(dir, names[i]))
		}
	}
	return filepath.Join(dir, name), nil
}
