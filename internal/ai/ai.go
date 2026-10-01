// Package ai 提供兼容 OpenAI Chat Completions 协议的极简客户端。
// 支持 OpenAI / DeepSeek / Moonshot / 通义 / Ollama / One-API 等任何兼容网关。
package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"regexp"
	"strings"
	"time"

	"inspirationer/internal/i18n"
	"inspirationer/internal/model"
)

// Client 是 AI 调用封装。
type Client struct {
	cfg model.AISettings
}

// New 依据设置创建客户端。
func New(cfg model.AISettings) *Client { return &Client{cfg: cfg} }

func (c *Client) endpoint() string {
	base := strings.TrimRight(strings.TrimSpace(c.cfg.BaseURL), "/")
	if base == "" {
		base = "https://api.openai.com/v1"
	}
	if strings.HasSuffix(base, "/chat/completions") {
		return base
	}
	return base + "/chat/completions"
}

type chatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatRequest struct {
	Model       string        `json:"model"`
	Messages    []chatMessage `json:"messages"`
	Temperature float64       `json:"temperature"`
	MaxTokens   int           `json:"max_tokens,omitempty"`
	Stream      bool          `json:"stream"`
}

type chatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
		Text string `json:"text"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

// Chat 发起一次对话补全请求。
func (c *Client) Chat(ctx context.Context, system, user string) (string, error) {
	if !c.cfg.Enabled {
		return "", i18n.Errorf("err.aiDisabledHint")
	}
	if strings.TrimSpace(c.cfg.APIKey) == "" && !strings.Contains(c.cfg.BaseURL, "localhost") && !strings.Contains(c.cfg.BaseURL, "127.0.0.1") {
		return "", i18n.Errorf("err.aiNoKey")
	}

	timeout := time.Duration(c.cfg.TimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 45 * time.Second
	}
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	msgs := []chatMessage{}
	if strings.TrimSpace(system) != "" {
		msgs = append(msgs, chatMessage{Role: "system", Content: system})
	}
	msgs = append(msgs, chatMessage{Role: "user", Content: user})

	body, err := json.Marshal(chatRequest{
		Model:       c.cfg.Model,
		Messages:    msgs,
		Temperature: c.cfg.Temperature,
		MaxTokens:   c.cfg.MaxTokens,
		Stream:      false,
	})
	if err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.endpoint(), bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/json")
	if strings.TrimSpace(c.cfg.APIKey) != "" {
		req.Header.Set("Authorization", "Bearer "+strings.TrimSpace(c.cfg.APIKey))
	}

	client := &http.Client{Timeout: timeout + 5*time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", i18n.Errorf("err.aiRequestFailed", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return "", i18n.Errorf("err.aiBadStatus", resp.StatusCode, truncate(string(raw), 400))
	}

	var cr chatResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return "", i18n.Errorf("err.aiBadResponse", err)
	}
	if cr.Error != nil && cr.Error.Message != "" {
		return "", i18n.Errorf("err.aiApiError", cr.Error.Message)
	}
	if len(cr.Choices) == 0 {
		return "", i18n.Errorf("err.aiEmptyResponse")
	}
	out := cr.Choices[0].Message.Content
	if strings.TrimSpace(out) == "" {
		out = cr.Choices[0].Text
	}
	return strings.TrimSpace(out), nil
}

const titleSystemPrompt = `你是一个灵感管理助手。请为用户的灵感片段生成一个简洁、准确、有辨识度的中文标题。
要求：
1. 只输出标题本身，不要引号、不要句号、不要任何解释或前缀；
2. 长度不超过 16 个汉字（英文不超过 40 字符）；
3. 能概括核心内容，避免“灵感”“想法”“无标题”这类空泛词。`

// GenerateTitle 依据正文生成标题。
func (c *Client) GenerateTitle(ctx context.Context, content string, maxRunes int) (string, error) {
	body := truncate(content, 6000)
	out, err := c.Chat(ctx, titleSystemPrompt, "灵感内容如下：\n\n"+body)
	if err != nil {
		return "", err
	}
	return cleanTitle(out, maxRunes), nil
}

var (
	codeFenceRe = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)```")
	jsonObjRe   = regexp.MustCompile(`(?s)\{.*\}`)
	leadTrimRe  = regexp.MustCompile(`^[\s"'“”‘’《》【】#*\-\d.、:：]+`)
	tailTrimRe  = regexp.MustCompile(`[\s"'“”‘’。.!！?？#*]+$`)
)

func cleanTitle(s string, maxRunes int) string {
	s = strings.TrimSpace(s)
	s = strings.TrimPrefix(s, "标题：")
	s = strings.TrimPrefix(s, "标题:")
	if i := strings.IndexAny(s, "\n\r"); i >= 0 {
		s = s[:i]
	}
	s = strings.TrimSpace(leadTrimRe.ReplaceAllString(s, ""))
	s = strings.TrimSpace(tailTrimRe.ReplaceAllString(s, ""))
	if maxRunes > 0 {
		r := []rune(s)
		if len(r) > maxRunes {
			s = string(r[:maxRunes])
		}
	}
	return s
}

// Suggestion 是 AI 对一条灵感给出的结构化建议。
type Suggestion struct {
	Title    string   `json:"title"`
	Tags     []string `json:"tags"`
	Category string   `json:"category"`
	Summary  string   `json:"summary"`
}

const suggestSystemPrompt = `你是一个灵感整理助手。请阅读用户的灵感片段，输出严格合法的 JSON（不要 markdown 代码块、不要多余文字），格式：
{"title":"不超过16字的标题","tags":["标签1","标签2","标签3"],"category":"单个分类名","summary":"一句话摘要"}

规则：
1. tags：2~4 个，中文优先，每个不超过 8 个字，尽量复用「已有标签」；避免空泛词（如“灵感”“想法”“其他”）。
2. category：只能给出 1 个，优先复用「已有分类」，否则给出一个更上位的新分类名（2~6 字），例如“产品设计”“技术方案”“写作素材”。
3. 如果内容过短或语义不明，也要尽量给出最合理的猜测。`

// Suggest 让 AI 对一条灵感给出标签与分类建议。
func (c *Client) Suggest(ctx context.Context, content string, existingTags, existingCategories []string) (Suggestion, error) {
	user := strings.Builder{}
	user.WriteString("已有标签（可复用，也可新增）：")
	if len(existingTags) == 0 {
		user.WriteString("（无）")
	} else {
		user.WriteString(strings.Join(existingTags, "、"))
	}
	user.WriteString("\n已有分类（可复用，也可新增）：")
	if len(existingCategories) == 0 {
		user.WriteString("（无）")
	} else {
		user.WriteString(strings.Join(existingCategories, "、"))
	}
	user.WriteString("\n\n灵感内容：\n")
	user.WriteString(truncate(content, 6000))

	out, err := c.Chat(ctx, suggestSystemPrompt, user.String())
	if err != nil {
		return Suggestion{}, err
	}
	return parseSuggestion(out), nil
}

func parseSuggestion(raw string) Suggestion {
	text := strings.TrimSpace(raw)
	if m := codeFenceRe.FindStringSubmatch(text); len(m) > 1 {
		text = strings.TrimSpace(m[1])
	}
	if m := jsonObjRe.FindString(text); m != "" {
		text = m
	}
	var s Suggestion
	if err := json.Unmarshal([]byte(text), &s); err != nil {
		// 兜底：把整段文本当作一行标题 + 关键词
		return Suggestion{Title: cleanTitle(raw, 16)}
	}
	s.Title = cleanTitle(s.Title, 16)
	clean := make([]string, 0, len(s.Tags))
	seen := map[string]bool{}
	for _, t := range s.Tags {
		t = strings.TrimSpace(strings.Trim(t, "#\"'“”"))
		if t == "" || len([]rune(t)) > 12 || seen[strings.ToLower(t)] {
			continue
		}
		seen[strings.ToLower(t)] = true
		clean = append(clean, t)
		if len(clean) >= 5 {
			break
		}
	}
	s.Tags = clean
	s.Category = strings.TrimSpace(strings.Trim(s.Category, "#\"'“”"))
	if len([]rune(s.Category)) > 12 {
		s.Category = string([]rune(s.Category)[:12])
	}
	s.Summary = strings.TrimSpace(s.Summary)
	return s
}

func truncate(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}

// TestConnection 用一次极小的请求验证配置是否可用。
func (c *Client) TestConnection(ctx context.Context) (string, error) {
	out, err := c.Chat(ctx, "你是一个测试助手，只回答被要求的内容。", "请只回复两个字：可用")
	if err != nil {
		return "", err
	}
	return out, nil
}
