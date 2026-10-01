// Package server 提供 REST API 与内嵌前端资源。
package server

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"inspirationer/internal/ai"
	"inspirationer/internal/model"
	"inspirationer/internal/store"
	"inspirationer/internal/webdav"
)

// Server 聚合存储与 HTTP 处理。
type Server struct {
	Store   *store.Store
	Version string
	WebFS   fs.FS
	Log     *log.Logger

	backupMu sync.Mutex
}

// New 创建服务器。
func New(st *store.Store, webFS fs.FS, version string, logger *log.Logger) *Server {
	return &Server{Store: st, Version: version, WebFS: webFS, Log: logger}
}

// Routes 组装路由。
func (s *Server) Routes() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("/api/", s.handleAPI)
	mux.Handle("/", s.handleStatic())
	return s.withLogging(mux)
}

func (s *Server) withLogging(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		next.ServeHTTP(w, r)
		if strings.HasPrefix(r.URL.Path, "/api/") && s.Log != nil {
			s.Log.Printf("%s %s (%s)", r.Method, r.URL.RequestURI(), time.Since(start).Round(time.Millisecond))
		}
	})
}

func (s *Server) handleStatic() http.Handler {
	fileServer := http.FileServer(http.FS(s.WebFS))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		p := strings.TrimPrefix(r.URL.Path, "/")
		if p == "" {
			p = "index.html"
		}
		if _, err := fs.Stat(s.WebFS, p); err != nil {
			// 前端为单页应用，未知路径回落到 index.html
			r = r.Clone(r.Context())
			r.URL.Path = "/"
			w.Header().Set("Cache-Control", "no-store")
			fileServer.ServeHTTP(w, r)
			return
		}
		if strings.HasSuffix(p, ".html") {
			w.Header().Set("Cache-Control", "no-store")
		}
		fileServer.ServeHTTP(w, r)
	})
}

// ---------------------------------------------------------------- helpers

func writeJSON(w http.ResponseWriter, code int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	_ = enc.Encode(v)
}

type apiError struct {
	Error string `json:"error"`
}

func writeErr(w http.ResponseWriter, code int, format string, args ...interface{}) {
	writeJSON(w, code, apiError{Error: fmt.Sprintf(format, args...)})
}

func decodeJSON(r *http.Request, dst interface{}) error {
	defer r.Body.Close()
	dec := json.NewDecoder(io.LimitReader(r.Body, 64<<20))
	return dec.Decode(dst)
}

// ---------------------------------------------------------------- router

func (s *Server) handleAPI(w http.ResponseWriter, r *http.Request) {
	parts := strings.Split(strings.Trim(strings.TrimPrefix(r.URL.Path, "/api/"), "/"), "/")
	if len(parts) == 0 || parts[0] == "" {
		writeErr(w, http.StatusNotFound, "未知接口")
		return
	}
	head := parts[0]
	rest := parts[1:]

	switch head {
	case "bootstrap":
		s.handleBootstrap(w, r)
	case "stats":
		writeJSON(w, http.StatusOK, s.Store.Stats())
	case "snippets":
		s.handleSnippets(w, r, rest)
	case "tags":
		s.handleTags(w, r, rest)
	case "categories":
		s.handleCategories(w, r, rest)
	case "settings":
		s.handleSettings(w, r)
	case "ai":
		s.handleAI(w, r, rest)
	case "backup":
		s.handleBackup(w, r, rest)
	case "webdav":
		s.handleWebDAV(w, r, rest)
	default:
		writeErr(w, http.StatusNotFound, "未知接口: %s", head)
	}
}

// ---------------------------------------------------------------- bootstrap

type bootstrapResponse struct {
	Version    string           `json:"version"`
	DataDir    string           `json:"dataDir"`
	Settings   model.Settings   `json:"settings"`
	Tags       []model.Tag      `json:"tags"`
	Categories []model.Category `json:"categories"`
	Stats      store.Stats      `json:"stats"`
	AIReady    bool             `json:"aiReady"`
}

func (s *Server) handleBootstrap(w http.ResponseWriter, r *http.Request) {
	st := s.Store.Settings()
	writeJSON(w, http.StatusOK, bootstrapResponse{
		Version:    s.Version,
		DataDir:    s.Store.Dir(),
		Settings:   st,
		Tags:       s.Store.Tags(),
		Categories: s.Store.Categories(),
		Stats:      s.Store.Stats(),
		AIReady:    st.AI.Enabled && strings.TrimSpace(st.AI.APIKey) != "",
	})
}

// ---------------------------------------------------------------- snippets

func (s *Server) handleSnippets(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			f := store.Filter{
				Query:      r.URL.Query().Get("query"),
				Mode:       r.URL.Query().Get("mode"),
				CategoryID: r.URL.Query().Get("categoryId"),
				Archive:    r.URL.Query().Get("archive"),
				Sort:       r.URL.Query().Get("sort"),
			}
			if v := r.URL.Query().Get("tagIds"); v != "" {
				f.TagIDs = splitCSV(v)
			}
			if v := r.URL.Query().Get("limit"); v != "" {
				f.Limit, _ = strconv.Atoi(v)
			}
			list := s.Store.ListSnippets(f)
			writeJSON(w, http.StatusOK, map[string]interface{}{
				"items": list,
				"total": len(list),
			})
		case http.MethodPost:
			s.createSnippet(w, r)
		default:
			writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
		}
		return
	}

	if rest[0] == "bulk" {
		s.bulkSnippets(w, r)
		return
	}

	id := rest[0]
	switch r.Method {
	case http.MethodGet:
		sn, err := s.Store.GetSnippet(id)
		if err != nil {
			writeErr(w, http.StatusNotFound, "灵感不存在")
			return
		}
		writeJSON(w, http.StatusOK, sn)
	case http.MethodPut:
		s.updateSnippet(w, r, id)
	case http.MethodDelete:
		if err := s.Store.DeleteSnippet(id); err != nil {
			writeErr(w, http.StatusNotFound, "灵感不存在")
			return
		}
		writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
	case http.MethodPatch:
		s.patchSnippet(w, r, id)
	default:
		writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
	}
}

type snippetInput struct {
	Title       string   `json:"title"`
	Content     string   `json:"content"`
	Tags        []string `json:"tags"`
	CategoryID  string   `json:"categoryId"`
	Pinned      bool     `json:"pinned"`
	Archived    bool     `json:"archived"`
	TitleSource string   `json:"titleSource"`
	AutoTitle   bool     `json:"autoTitle"`
}

func (s *Server) createSnippet(w http.ResponseWriter, r *http.Request) {
	var in snippetInput
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
		return
	}
	if strings.TrimSpace(in.Content) == "" && strings.TrimSpace(in.Title) == "" {
		writeErr(w, http.StatusBadRequest, "内容不能为空")
		return
	}

	warnings := []string{}
	title := strings.TrimSpace(in.Title)
	source := model.TitleSourceUser
	if title == "" {
		title, source, warnings = s.autoTitle(r.Context(), in.Content)
	}

	sn, err := s.Store.CreateSnippet(model.Snippet{
		Title:       title,
		Content:     in.Content,
		Tags:        in.Tags,
		CategoryID:  in.CategoryID,
		Pinned:      in.Pinned,
		TitleSource: source,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "保存失败: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"snippet":  sn,
		"warnings": warnings,
		"stats":    s.Store.Stats(),
	})
}

// autoTitle 未填标题时的兜底策略：优先 AI，其次截取正文。
func (s *Server) autoTitle(ctx context.Context, content string) (string, string, []string) {
	settings := s.Store.Settings()
	warnings := []string{}
	maxRunes := settings.UI.TitleMaxRunes
	if maxRunes <= 0 {
		maxRunes = 10
	}
	if settings.AI.Enabled {
		cctx, cancel := context.WithTimeout(ctx, time.Duration(settings.AI.TimeoutSeconds+5)*time.Second)
		defer cancel()
		t, err := ai.New(settings.AI).GenerateTitle(cctx, content, 0)
		if err == nil && strings.TrimSpace(t) != "" {
			return strings.TrimSpace(t), model.TitleSourceAI, warnings
		}
		if err != nil {
			warnings = append(warnings, "AI 生成标题失败，已改用正文截取："+err.Error())
		}
	} else {
		warnings = append(warnings, "未配置 AI API，标题取正文前 "+strconv.Itoa(maxRunes)+" 个字")
	}
	return TruncateTitle(content, maxRunes), model.TitleSourceTruncate, warnings
}

// TruncateTitle 截取正文开头若干字作为标题。
func TruncateTitle(content string, maxRunes int) string {
	if maxRunes <= 0 {
		maxRunes = 10
	}
	clean := strings.ReplaceAll(content, "\r\n", "\n")
	lines := []string{}
	for _, ln := range strings.Split(clean, "\n") {
		ln = strings.TrimSpace(ln)
		ln = strings.TrimLeft(ln, "#>-*+ \t")
		ln = strings.Trim(ln, "`~")
		if ln != "" {
			lines = append(lines, ln)
		}
	}
	joined := strings.Join(lines, " ")
	joined = strings.TrimSpace(joined)
	r := []rune(joined)
	if len(r) == 0 {
		return "未命名灵感"
	}
	if len(r) > maxRunes {
		return strings.TrimSpace(string(r[:maxRunes]))
	}
	return joined
}

func (s *Server) updateSnippet(w http.ResponseWriter, r *http.Request, id string) {
	var in snippetInput
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
		return
	}
	cur, err := s.Store.GetSnippet(id)
	if err != nil {
		writeErr(w, http.StatusNotFound, "灵感不存在")
		return
	}
	title := strings.TrimSpace(in.Title)
	source := in.TitleSource
	if title == "" {
		settings := s.Store.Settings()
		maxRunes := settings.UI.TitleMaxRunes
		title = TruncateTitle(in.Content, maxRunes)
		source = model.TitleSourceTruncate
	}
	if source == "" {
		source = cur.TitleSource
		if source == "" {
			source = model.TitleSourceUser
		}
	}
	updated, err := s.Store.UpdateSnippet(id, model.Snippet{
		Title:       title,
		Content:     in.Content,
		Tags:        in.Tags,
		CategoryID:  in.CategoryID,
		Pinned:      in.Pinned,
		Archived:    in.Archived,
		TitleSource: source,
	})
	if err != nil {
		writeErr(w, http.StatusInternalServerError, "保存失败: %v", err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"snippet": updated,
		"stats":   s.Store.Stats(),
	})
}

type patchInput struct {
	Title       *string   `json:"title"`
	Content     *string   `json:"content"`
	Tags        *[]string `json:"tags"`
	AddTags     []string  `json:"addTags"`
	CategoryID  *string   `json:"categoryId"`
	Pinned      *bool     `json:"pinned"`
	Archived    *bool     `json:"archived"`
	TitleSource *string   `json:"titleSource"`
}

func (s *Server) patchSnippet(w http.ResponseWriter, r *http.Request, id string) {
	var in patchInput
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
		return
	}
	addIDs := []string{}
	for _, name := range in.AddTags {
		t, _, err := s.Store.EnsureTag(name)
		if err == nil {
			addIDs = append(addIDs, t.ID)
		}
	}
	sn, err := s.Store.MutateSnippet(id, func(cur *model.Snippet) {
		if in.Title != nil {
			cur.Title = strings.TrimSpace(*in.Title)
			if cur.Title == "" {
				cur.Title = TruncateTitle(cur.Content, s.Store.Settings().UI.TitleMaxRunes)
				cur.TitleSource = model.TitleSourceTruncate
			} else if in.TitleSource == nil {
				cur.TitleSource = model.TitleSourceUser
			}
		}
		if in.Content != nil {
			cur.Content = *in.Content
		}
		if in.Tags != nil {
			cur.Tags = *in.Tags
		}
		cur.Tags = append(cur.Tags, addIDs...)
		if in.CategoryID != nil {
			cur.CategoryID = *in.CategoryID
		}
		if in.Pinned != nil {
			cur.Pinned = *in.Pinned
		}
		if in.Archived != nil {
			cur.Archived = *in.Archived
		}
		if in.TitleSource != nil {
			cur.TitleSource = *in.TitleSource
		}
	})
	if err != nil {
		writeErr(w, http.StatusNotFound, "灵感不存在")
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"snippet": sn,
		"tags":    s.Store.Tags(),
		"stats":   s.Store.Stats(),
	})
}

type bulkInput struct {
	IDs        []string `json:"ids"`
	Action     string   `json:"action"` // delete | assign | archive | unarchive | pin | unpin | categorize
	CategoryID *string  `json:"categoryId"`
	AddTags    []string `json:"addTags"`
	RemoveTags []string `json:"removeTags"`
}

func (s *Server) bulkSnippets(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
		return
	}
	var in bulkInput
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
		return
	}
	if len(in.IDs) == 0 {
		writeErr(w, http.StatusBadRequest, "未选择任何灵感")
		return
	}
	affected := 0
	switch in.Action {
	case "delete":
		n, err := s.Store.DeleteSnippets(in.IDs)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "删除失败: %v", err)
			return
		}
		affected = n
	case "archive", "unarchive", "pin", "unpin":
		want := in.Action == "archive" || in.Action == "pin"
		isPin := in.Action == "pin" || in.Action == "unpin"
		for _, id := range in.IDs {
			_, err := s.Store.MutateSnippet(id, func(cur *model.Snippet) {
				if isPin {
					cur.Pinned = want
				} else {
					cur.Archived = want
				}
			})
			if err == nil {
				affected++
			}
		}
	case "assign", "categorize", "tag":
		addIDs := []string{}
		for _, name := range in.AddTags {
			t, _, err := s.Store.EnsureTag(name)
			if err == nil {
				addIDs = append(addIDs, t.ID)
			}
		}
		removeIDs := []string{}
		for _, id := range in.RemoveTags {
			removeIDs = append(removeIDs, id)
		}
		n, err := s.Store.BulkAssign(in.IDs, in.CategoryID, addIDs, removeIDs)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "批量修改失败: %v", err)
			return
		}
		affected = n
	default:
		writeErr(w, http.StatusBadRequest, "未知操作: %s", in.Action)
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"affected": affected,
		"stats":    s.Store.Stats(),
		"tags":     s.Store.Tags(),
	})
}

// ---------------------------------------------------------------- tags

func (s *Server) handleTags(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]interface{}{"items": s.Store.Tags(), "stats": s.Store.Stats()})
		case http.MethodPost:
			var in struct {
				Name  string `json:"name"`
				Color string `json:"color"`
			}
			if err := decodeJSON(r, &in); err != nil {
				writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
				return
			}
			t, err := s.Store.CreateTag(in.Name, in.Color)
			if err != nil {
				writeErr(w, http.StatusBadRequest, "%v", err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"tag": t, "items": s.Store.Tags(), "stats": s.Store.Stats()})
		default:
			writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
		}
		return
	}
	if len(rest) >= 2 && rest[1] == "merge" {
		s.mergeTag(w, r, rest[0])
		return
	}
	id := rest[0]
	switch r.Method {
	case http.MethodPut:
		var in struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if err := decodeJSON(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
			return
		}
		t, err := s.Store.UpdateTag(id, in.Name, in.Color)
		if err != nil {
			writeErr(w, http.StatusNotFound, "标签不存在")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"tag": t, "items": s.Store.Tags()})
	case http.MethodDelete:
		if err := s.Store.DeleteTag(id); err != nil {
			writeErr(w, http.StatusNotFound, "标签不存在")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": s.Store.Tags(), "stats": s.Store.Stats()})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
	}
}

func (s *Server) mergeTag(w http.ResponseWriter, r *http.Request, fromID string) {
	var in struct {
		Into string `json:"into"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
		return
	}
	if in.Into == "" || in.Into == fromID {
		writeErr(w, http.StatusBadRequest, "目标标签无效")
		return
	}
	filter := store.Filter{TagIDs: []string{fromID}, Archive: "all"}
	list := s.Store.ListSnippets(filter)
	for _, sn := range list {
		_, _ = s.Store.MutateSnippet(sn.ID, func(cur *model.Snippet) {
			kept := cur.Tags[:0:0]
			hasInto := false
			for _, t := range cur.Tags {
				if t == fromID {
					continue
				}
				if t == in.Into {
					hasInto = true
				}
				kept = append(kept, t)
			}
			if !hasInto {
				kept = append(kept, in.Into)
			}
			cur.Tags = kept
		})
	}
	_ = s.Store.DeleteTag(fromID)
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": s.Store.Tags(), "stats": s.Store.Stats()})
}

// ---------------------------------------------------------------- categories

func (s *Server) handleCategories(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) == 0 {
		switch r.Method {
		case http.MethodGet:
			writeJSON(w, http.StatusOK, map[string]interface{}{"items": s.Store.Categories(), "stats": s.Store.Stats()})
		case http.MethodPost:
			var in struct {
				Name  string `json:"name"`
				Color string `json:"color"`
			}
			if err := decodeJSON(r, &in); err != nil {
				writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
				return
			}
			c, err := s.Store.CreateCategory(in.Name, in.Color)
			if err != nil {
				writeErr(w, http.StatusBadRequest, "%v", err)
				return
			}
			writeJSON(w, http.StatusOK, map[string]interface{}{"category": c, "items": s.Store.Categories(), "stats": s.Store.Stats()})
		default:
			writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
		}
		return
	}
	id := rest[0]
	switch r.Method {
	case http.MethodPut:
		var in struct {
			Name  string `json:"name"`
			Color string `json:"color"`
		}
		if err := decodeJSON(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
			return
		}
		c, err := s.Store.UpdateCategory(id, in.Name, in.Color)
		if err != nil {
			writeErr(w, http.StatusNotFound, "分类不存在")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"category": c, "items": s.Store.Categories()})
	case http.MethodDelete:
		if err := s.Store.DeleteCategory(id); err != nil {
			writeErr(w, http.StatusNotFound, "分类不存在")
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": s.Store.Categories(), "stats": s.Store.Stats()})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
	}
}

// ---------------------------------------------------------------- settings

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	switch r.Method {
	case http.MethodGet:
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"settings": s.Store.Settings(),
			"dataDir":  s.Store.Dir(),
		})
	case http.MethodPut:
		var in model.Settings
		if err := decodeJSON(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
			return
		}
		cur := s.Store.Settings()
		// 允许前端提交掩码值表示“不修改密码”
		if in.AI.APIKey == "__KEEP__" {
			in.AI.APIKey = cur.AI.APIKey
		}
		if in.WebDAV.Password == "__KEEP__" {
			in.WebDAV.Password = cur.WebDAV.Password
		}
		in.WebDAV.LastBackup = cur.WebDAV.LastBackup
		in.WebDAV.LastStatus = cur.WebDAV.LastStatus
		out, err := s.Store.UpdateSettings(in)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "保存设置失败: %v", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"settings": out})
	default:
		writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
	}
}

// ---------------------------------------------------------------- AI

func (s *Server) handleAI(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) == 0 {
		writeErr(w, http.StatusNotFound, "未知接口")
		return
	}
	switch rest[0] {
	case "test":
		s.aiTest(w, r)
	case "title":
		s.aiTitle(w, r)
	case "suggest":
		s.aiSuggest(w, r)
	case "apply":
		s.aiApply(w, r)
	default:
		writeErr(w, http.StatusNotFound, "未知接口: ai/%s", rest[0])
	}
}

func (s *Server) aiTest(w http.ResponseWriter, r *http.Request) {
	cfg := s.Store.Settings().AI
	if r.Method == http.MethodPost {
		// 允许用未保存的配置做“测试连接”
		var in model.AISettings
		if err := decodeJSON(r, &in); err == nil && strings.TrimSpace(in.BaseURL) != "" {
			if in.APIKey == "__KEEP__" {
				in.APIKey = cfg.APIKey
			}
			in.Enabled = true
			cfg = in
		}
	}
	if !cfg.Enabled {
		writeErr(w, http.StatusBadRequest, "AI 功能未启用")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(cfg.TimeoutSeconds+10)*time.Second)
	defer cancel()
	out, err := ai.New(cfg).TestConnection(ctx)
	if err != nil {
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "message": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": "连接成功，模型回复：" + out})
}

func (s *Server) aiTitle(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
		return
	}
	var in struct {
		ID      string `json:"id"`
		Content string `json:"content"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
		return
	}
	content := in.Content
	if content == "" && in.ID != "" {
		if sn, err := s.Store.GetSnippet(in.ID); err == nil {
			content = sn.Content
		}
	}
	if strings.TrimSpace(content) == "" {
		writeErr(w, http.StatusBadRequest, "内容为空")
		return
	}
	settings := s.Store.Settings()
	if !settings.AI.Enabled {
		writeErr(w, http.StatusBadRequest, "AI 功能未启用")
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), time.Duration(settings.AI.TimeoutSeconds+5)*time.Second)
	defer cancel()
	title, err := ai.New(settings.AI).GenerateTitle(ctx, content, 0)
	if err != nil {
		writeErr(w, http.StatusBadGateway, "%v", err)
		return
	}
	if in.ID != "" {
		_, _ = s.Store.MutateSnippet(in.ID, func(cur *model.Snippet) {
			cur.Title = title
			cur.TitleSource = model.TitleSourceAI
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"title": title})
}

type suggestItem struct {
	ID       string   `json:"id"`
	Title    string   `json:"title"`
	Tags     []string `json:"tags"`
	Category string   `json:"category"`
	Summary  string   `json:"summary"`
	Error    string   `json:"error,omitempty"`
}

func (s *Server) aiSuggest(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
		return
	}
	var in struct {
		IDs []string `json:"ids"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
		return
	}
	settings := s.Store.Settings()
	if !settings.AI.Enabled {
		writeErr(w, http.StatusBadRequest, "AI 功能未启用，请先在设置中配置 AI API")
		return
	}
	if len(in.IDs) == 0 {
		writeErr(w, http.StatusBadRequest, "未选择任何灵感")
		return
	}

	tagNames := []string{}
	for _, t := range s.Store.Tags() {
		tagNames = append(tagNames, t.Name)
	}
	catNames := []string{}
	for _, c := range s.Store.Categories() {
		catNames = append(catNames, c.Name)
	}

	type job struct {
		idx int
		sn  model.Snippet
	}
	jobs := make([]job, 0, len(in.IDs))
	for _, id := range in.IDs {
		if sn, err := s.Store.GetSnippet(id); err == nil {
			jobs = append(jobs, job{idx: len(jobs), sn: sn})
		}
	}
	results := make([]suggestItem, len(jobs))

	sem := make(chan struct{}, 3)
	var wg sync.WaitGroup
	for i := range jobs {
		wg.Add(1)
		go func(j job) {
			defer wg.Done()
			sem <- struct{}{}
			defer func() { <-sem }()
			ctx, cancel := context.WithTimeout(r.Context(), time.Duration(settings.AI.TimeoutSeconds+5)*time.Second)
			defer cancel()
			sug, err := ai.New(settings.AI).Suggest(ctx, j.sn.Content, tagNames, catNames)
			item := suggestItem{ID: j.sn.ID, Title: j.sn.Title}
			if err != nil {
				item.Error = err.Error()
			} else {
				item.Tags = sug.Tags
				item.Category = sug.Category
				item.Summary = sug.Summary
			}
			results[j.idx] = item
		}(jobs[i])
	}
	wg.Wait()

	out := make([]suggestItem, 0, len(results))
	for _, it := range results {
		if it.ID != "" {
			out = append(out, it)
		}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"items": out})
}

type applyItem struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	ApplyTitle bool     `json:"applyTitle"`
	Tags       []string `json:"tags"`
	Category   string   `json:"category"`
	Mode       string   `json:"mode"` // merge | replace
}

func (s *Server) aiApply(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
		return
	}
	var in struct {
		Items []applyItem `json:"items"`
	}
	if err := decodeJSON(r, &in); err != nil {
		writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
		return
	}
	applied := 0
	tagsCreated := 0
	catsCreated := 0
	errors := []string{}
	for _, it := range in.Items {
		if _, err := s.Store.GetSnippet(it.ID); err != nil {
			errors = append(errors, "灵感不存在: "+it.ID)
			continue
		}
		tagIDs := []string{}
		for _, name := range it.Tags {
			t, created, err := s.Store.EnsureTag(name)
			if err != nil {
				continue
			}
			if created {
				tagsCreated++
			}
			tagIDs = append(tagIDs, t.ID)
		}
		catID := ""
		if strings.TrimSpace(it.Category) != "" {
			c, created, err := s.Store.EnsureCategory(it.Category)
			if err == nil {
				catID = c.ID
				if created {
					catsCreated++
				}
			}
		}
		_, err := s.Store.MutateSnippet(it.ID, func(cur *model.Snippet) {
			if it.Mode == "replace" {
				cur.Tags = tagIDs
			} else {
				cur.Tags = append(cur.Tags, tagIDs...)
			}
			if catID != "" {
				cur.CategoryID = catID
			}
			if it.ApplyTitle && strings.TrimSpace(it.Title) != "" {
				cur.Title = strings.TrimSpace(it.Title)
				cur.TitleSource = model.TitleSourceAI
			}
		})
		if err != nil {
			errors = append(errors, err.Error())
			continue
		}
		applied++
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"applied":     applied,
		"tagsCreated": tagsCreated,
		"catsCreated": catsCreated,
		"errors":      errors,
		"tags":        s.Store.Tags(),
		"categories":  s.Store.Categories(),
		"stats":       s.Store.Stats(),
	})
}

// ---------------------------------------------------------------- backup

func (s *Server) handleBackup(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) == 0 {
		writeErr(w, http.StatusNotFound, "未知接口")
		return
	}
	switch rest[0] {
	case "export":
		bk := s.Store.Export(true)
		name := "inspirationer-" + time.Now().Format("20060102-150405") + ".json"
		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.Header().Set("Content-Disposition", "attachment; filename=\""+name+"\"")
		enc := json.NewEncoder(w)
		enc.SetEscapeHTML(false)
		enc.SetIndent("", "  ")
		_ = enc.Encode(bk)
	case "local":
		path, err := s.Store.Snapshot(30)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "本地快照失败: %v", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "path": path})
	case "import":
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
			return
		}
		var in struct {
			Mode string       `json:"mode"`
			Data model.Backup `json:"data"`
		}
		if err := decodeJSON(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
			return
		}
		if in.Data.App == "" && len(in.Data.Snippets) == 0 && len(in.Data.Tags) == 0 {
			writeErr(w, http.StatusBadRequest, "备份数据为空或格式不正确")
			return
		}
		mode := store.ImportMerge
		if in.Mode == string(store.ImportReplace) {
			mode = store.ImportReplace
		}
		res, err := s.Store.Import(in.Data, mode)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "导入失败: %v", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok": true, "result": res,
			"stats": s.Store.Stats(), "tags": s.Store.Tags(), "categories": s.Store.Categories(),
		})
	default:
		writeErr(w, http.StatusNotFound, "未知接口: backup/%s", rest[0])
	}
}

// ---------------------------------------------------------------- webdav

func (s *Server) handleWebDAV(w http.ResponseWriter, r *http.Request, rest []string) {
	if len(rest) == 0 {
		writeErr(w, http.StatusNotFound, "未知接口")
		return
	}
	switch rest[0] {
	case "test":
		cfg, err := s.webdavConfigFromRequest(r)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "message": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 30*time.Second)
		defer cancel()
		msg, err := webdav.New(cfg.URL, cfg.Username, cfg.Password).Test(ctx)
		if err != nil {
			writeJSON(w, http.StatusOK, map[string]interface{}{"ok": false, "message": err.Error()})
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "message": msg})
	case "backup":
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
			return
		}
		info, err := s.BackupToWebDAV(r.Context())
		if err != nil {
			writeErr(w, http.StatusBadGateway, "%v", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "info": info})
	case "list":
		cfg := s.Store.Settings().WebDAV
		ctx, cancel := context.WithTimeout(r.Context(), 45*time.Second)
		defer cancel()
		files, err := webdav.New(cfg.URL, cfg.Username, cfg.Password).List(ctx, cfg.RemoteDir)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "%v", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{"items": files, "dir": cfg.RemoteDir})
	case "restore":
		if r.Method != http.MethodPost {
			writeErr(w, http.StatusMethodNotAllowed, "方法不支持")
			return
		}
		var in struct {
			Name string `json:"name"`
			Mode string `json:"mode"`
		}
		if err := decodeJSON(r, &in); err != nil {
			writeErr(w, http.StatusBadRequest, "请求体解析失败: %v", err)
			return
		}
		if strings.TrimSpace(in.Name) == "" {
			writeErr(w, http.StatusBadRequest, "未指定文件")
			return
		}
		cfg := s.Store.Settings().WebDAV
		ctx, cancel := context.WithTimeout(r.Context(), 120*time.Second)
		defer cancel()
		rel := strings.Trim(strings.ReplaceAll(cfg.RemoteDir, "\\", "/"), "/")
		if rel != "" {
			rel += "/"
		}
		data, err := webdav.New(cfg.URL, cfg.Username, cfg.Password).Get(ctx, rel+in.Name)
		if err != nil {
			writeErr(w, http.StatusBadGateway, "%v", err)
			return
		}
		var bk model.Backup
		if err := json.Unmarshal(data, &bk); err != nil {
			writeErr(w, http.StatusBadRequest, "远端文件不是有效的备份: %v", err)
			return
		}
		mode := store.ImportMerge
		if in.Mode == string(store.ImportReplace) {
			mode = store.ImportReplace
		}
		// 恢复前先本地快照，避免误操作
		_, _ = s.Store.Snapshot(30)
		res, err := s.Store.Import(bk, mode)
		if err != nil {
			writeErr(w, http.StatusInternalServerError, "恢复失败: %v", err)
			return
		}
		writeJSON(w, http.StatusOK, map[string]interface{}{
			"ok": true, "result": res, "stats": s.Store.Stats(),
			"tags": s.Store.Tags(), "categories": s.Store.Categories(),
		})
	default:
		writeErr(w, http.StatusNotFound, "未知接口: webdav/%s", rest[0])
	}
}

func (s *Server) webdavConfigFromRequest(r *http.Request) (model.WebDAVSettings, error) {
	cfg := s.Store.Settings().WebDAV
	if r.Method == http.MethodPost {
		var in model.WebDAVSettings
		if err := decodeJSON(r, &in); err == nil && strings.TrimSpace(in.URL) != "" {
			if in.Password == "__KEEP__" {
				in.Password = cfg.Password
			}
			if in.RemoteDir == "" {
				in.RemoteDir = cfg.RemoteDir
			}
			cfg = in
		}
	}
	if strings.TrimSpace(cfg.URL) == "" {
		return cfg, fmt.Errorf("未配置 WebDAV 地址")
	}
	return cfg, nil
}

// BackupInfo 描述一次备份结果。
type BackupInfo struct {
	File    string    `json:"file"`
	Bytes   int       `json:"bytes"`
	When    time.Time `json:"when"`
	Pruned  int       `json:"pruned"`
	Message string    `json:"message"`
}

// BackupToWebDAV 立即执行一次 WebDAV 备份。
func (s *Server) BackupToWebDAV(ctx context.Context) (BackupInfo, error) {
	s.backupMu.Lock()
	defer s.backupMu.Unlock()

	cfg := s.Store.Settings().WebDAV
	if !cfg.Enabled && strings.TrimSpace(cfg.URL) == "" {
		return BackupInfo{}, fmt.Errorf("WebDAV 未启用或未配置地址")
	}
	client := webdav.New(cfg.URL, cfg.Username, cfg.Password)
	dir := strings.Trim(strings.ReplaceAll(cfg.RemoteDir, "\\", "/"), "/")

	ctx, cancel := context.WithTimeout(ctx, 120*time.Second)
	defer cancel()
	if err := client.EnsureDir(ctx, dir); err != nil {
		_ = s.Store.PatchWebDAVStatus(time.Time{}, "失败: "+err.Error())
		return BackupInfo{}, err
	}

	bk := s.Store.Export(true)
	payload, err := json.MarshalIndent(bk, "", "  ")
	if err != nil {
		return BackupInfo{}, err
	}
	name := "inspirationer-backup-" + time.Now().Format("20060102-150405") + ".json"
	rel := name
	if dir != "" {
		rel = dir + "/" + name
	}
	if err := client.Put(ctx, rel, payload); err != nil {
		_ = s.Store.PatchWebDAVStatus(time.Time{}, "失败: "+err.Error())
		return BackupInfo{}, err
	}
	// 额外维护一份 latest.json，方便一键恢复
	latestRel := "latest.json"
	if dir != "" {
		latestRel = dir + "/latest.json"
	}
	_ = client.Put(ctx, latestRel, payload)

	info := BackupInfo{File: name, Bytes: len(payload), When: time.Now(), Message: "备份成功"}

	// 清理旧备份
	if cfg.KeepRemote > 0 {
		if files, err := client.List(ctx, dir); err == nil {
			kept := 0
			for _, f := range files {
				if f.IsDir || f.Name == "latest.json" || !strings.HasSuffix(strings.ToLower(f.Name), ".json") {
					continue
				}
				kept++
				if kept > cfg.KeepRemote {
					delRel := f.Name
					if dir != "" {
						delRel = dir + "/" + f.Name
					}
					if err := client.Delete(ctx, delRel); err == nil {
						info.Pruned++
					}
				}
			}
		}
	}

	_ = s.Store.PatchWebDAVStatus(info.When, fmt.Sprintf("成功：%s（%d 字节）", name, len(payload)))
	return info, nil
}

// StartAutoBackup 启动定时自动备份循环。
func (s *Server) StartAutoBackup(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(time.Minute)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				cfg := s.Store.Settings().WebDAV
				if !cfg.Enabled || strings.TrimSpace(cfg.URL) == "" {
					continue
				}
				interval := time.Duration(cfg.IntervalMinutes) * time.Minute
				if interval <= 0 {
					interval = time.Hour
				}
				if time.Since(cfg.LastBackup) < interval {
					continue
				}
				info, err := s.BackupToWebDAV(context.Background())
				if s.Log != nil {
					if err != nil {
						s.Log.Printf("自动备份失败: %v", err)
					} else {
						s.Log.Printf("自动备份成功: %s (%d 字节)", info.File, info.Bytes)
					}
				}
			}
		}
	}()
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p != "" {
			out = append(out, p)
		}
	}
	return out
}
