// Package model 定义灵感管理器的核心数据结构。
package model

import "time"

// 标题来源
const (
	TitleSourceUser     = "user"     // 用户自己输入
	TitleSourceAI       = "ai"       // AI 总结生成
	TitleSourceTruncate = "truncate" // 截取正文前 N 个字
)

// Snippet 是一条灵感片段。
type Snippet struct {
	ID          string    `json:"id"`
	Title       string    `json:"title"`
	Content     string    `json:"content"`
	Tags        []string  `json:"tags"` // 存放 Tag.ID
	CategoryID  string    `json:"categoryId"`
	TitleSource string    `json:"titleSource"`
	Pinned      bool      `json:"pinned"`
	Archived    bool      `json:"archived"`
	CreatedAt   time.Time `json:"createdAt"`
	UpdatedAt   time.Time `json:"updatedAt"`
}

// Tag 标签。
type Tag struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// Category 分类。
type Category struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Color string `json:"color"`
}

// AISettings 兼容 OpenAI Chat Completions 协议的接口配置。
type AISettings struct {
	Enabled        bool    `json:"enabled"`
	BaseURL        string  `json:"baseUrl"`
	APIKey         string  `json:"apiKey"`
	Model          string  `json:"model"`
	Temperature    float64 `json:"temperature"`
	MaxTokens      int     `json:"maxTokens"`
	TimeoutSeconds int     `json:"timeoutSeconds"`
}

// WebDAVSettings WebDAV 备份配置。
type WebDAVSettings struct {
	Enabled         bool      `json:"enabled"`
	URL             string    `json:"url"`
	Username        string    `json:"username"`
	Password        string    `json:"password"`
	RemoteDir       string    `json:"remoteDir"`
	IntervalMinutes int       `json:"intervalMinutes"`
	KeepRemote      int       `json:"keepRemote"`
	LastBackup      time.Time `json:"lastBackup"`
	LastStatus      string    `json:"lastStatus"`
}

// Shortcuts 可自定义的快捷键，使用 "Alt+N" / "Ctrl+Shift+K" / "Escape" 形式。
type Shortcuts struct {
	NewSnippet    string `json:"newSnippet"`
	SaveSnippet   string `json:"saveSnippet"`
	FocusSearch   string `json:"focusSearch"`
	SelectAll     string `json:"selectAll"`
	PipelineNext  string `json:"pipelineNext"`
	TogglePreview string `json:"togglePreview"`
	OpenSettings  string `json:"openSettings"`
	CloseModal    string `json:"closeModal"`
}

// UISettings 界面偏好。
type UISettings struct {
	Theme          string `json:"theme"` // dark | light
	CardPreview    bool   `json:"cardPreview"`
	TitleMaxRunes  int    `json:"titleMaxRunes"`
	ConfirmDelete  bool   `json:"confirmDelete"`
	AutoBackupHint bool   `json:"autoBackupHint"`
}

// Settings 全部设置。
type Settings struct {
	AI        AISettings     `json:"ai"`
	WebDAV    WebDAVSettings `json:"webdav"`
	Shortcuts Shortcuts      `json:"shortcuts"`
	UI        UISettings     `json:"ui"`
}

// Backup 是导出/备份文件的结构。
type Backup struct {
	Version    int        `json:"version"`
	App        string     `json:"app"`
	ExportedAt time.Time  `json:"exportedAt"`
	Snippets   []Snippet  `json:"snippets"`
	Tags       []Tag      `json:"tags"`
	Categories []Category `json:"categories"`
	Settings   *Settings  `json:"settings,omitempty"`
}

// DefaultSettings 返回带默认值的设置。
func DefaultSettings() Settings {
	return Settings{
		AI: AISettings{
			Enabled:        false,
			BaseURL:        "https://api.openai.com/v1",
			APIKey:         "",
			Model:          "gpt-4o-mini",
			Temperature:    0.3,
			MaxTokens:      512,
			TimeoutSeconds: 45,
		},
		WebDAV: WebDAVSettings{
			Enabled:         false,
			URL:             "",
			Username:        "",
			Password:        "",
			RemoteDir:       "inspirationer",
			IntervalMinutes: 60,
			KeepRemote:      10,
		},
		Shortcuts: Shortcuts{
			NewSnippet:    "Alt+N",
			SaveSnippet:   "Alt+S",
			FocusSearch:   "Alt+K",
			SelectAll:     "Alt+A",
			PipelineNext:  "Alt+J",
			TogglePreview: "Alt+P",
			OpenSettings:  "Alt+O",
			CloseModal:    "Escape",
		},
		UI: UISettings{
			Theme:         "dark",
			CardPreview:   true,
			TitleMaxRunes: 10,
			ConfirmDelete: true,
		},
	}
}

// Normalize 补齐缺失/非法的设置项。
func (s *Settings) Normalize() {
	d := DefaultSettings()
	if s.AI.BaseURL == "" {
		s.AI.BaseURL = d.AI.BaseURL
	}
	if s.AI.Model == "" {
		s.AI.Model = d.AI.Model
	}
	if s.AI.MaxTokens <= 0 {
		s.AI.MaxTokens = d.AI.MaxTokens
	}
	if s.AI.TimeoutSeconds <= 0 || s.AI.TimeoutSeconds > 600 {
		s.AI.TimeoutSeconds = d.AI.TimeoutSeconds
	}
	if s.WebDAV.RemoteDir == "" {
		s.WebDAV.RemoteDir = d.WebDAV.RemoteDir
	}
	if s.WebDAV.IntervalMinutes <= 0 {
		s.WebDAV.IntervalMinutes = d.WebDAV.IntervalMinutes
	}
	if s.WebDAV.KeepRemote <= 0 {
		s.WebDAV.KeepRemote = d.WebDAV.KeepRemote
	}
	if s.Shortcuts.NewSnippet == "" {
		s.Shortcuts.NewSnippet = d.Shortcuts.NewSnippet
	}
	if s.Shortcuts.SaveSnippet == "" {
		s.Shortcuts.SaveSnippet = d.Shortcuts.SaveSnippet
	}
	if s.Shortcuts.FocusSearch == "" {
		s.Shortcuts.FocusSearch = d.Shortcuts.FocusSearch
	}
	if s.Shortcuts.SelectAll == "" {
		s.Shortcuts.SelectAll = d.Shortcuts.SelectAll
	}
	if s.Shortcuts.PipelineNext == "" {
		s.Shortcuts.PipelineNext = d.Shortcuts.PipelineNext
	}
	if s.Shortcuts.TogglePreview == "" {
		s.Shortcuts.TogglePreview = d.Shortcuts.TogglePreview
	}
	if s.Shortcuts.OpenSettings == "" {
		s.Shortcuts.OpenSettings = d.Shortcuts.OpenSettings
	}
	if s.Shortcuts.CloseModal == "" {
		s.Shortcuts.CloseModal = d.Shortcuts.CloseModal
	}
	if s.UI.Theme != "light" && s.UI.Theme != "dark" {
		s.UI.Theme = d.UI.Theme
	}
	if s.UI.TitleMaxRunes <= 0 || s.UI.TitleMaxRunes > 200 {
		s.UI.TitleMaxRunes = d.UI.TitleMaxRunes
	}
}

// Palette 是新标签/分类自动取色用的调色板。
var Palette = []string{
	"#ef4444", "#f97316", "#f59e0b", "#eab308",
	"#84cc16", "#22c55e", "#10b981", "#14b8a6",
	"#06b6d4", "#0ea5e9", "#3b82f6", "#6366f1",
	"#8b5cf6", "#a855f7", "#d946ef", "#ec4899",
}
