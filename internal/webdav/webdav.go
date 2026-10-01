// Package webdav 实现最小可用的 WebDAV 客户端（PUT/GET/PROPFIND/MKCOL/DELETE）。
package webdav

import (
	"bytes"
	"context"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"path"
	"sort"
	"strings"
	"time"

	"inspirationer/internal/i18n"
)

// Client 是 WebDAV 连接配置。
type Client struct {
	BaseURL  string
	Username string
	Password string
	HTTP     *http.Client
}

// New 创建客户端。
func New(baseURL, username, password string) *Client {
	return &Client{
		BaseURL:  strings.TrimSpace(baseURL),
		Username: username,
		Password: password,
		HTTP:     &http.Client{Timeout: 60 * time.Second},
	}
}

// RemoteFile 描述远端文件。
type RemoteFile struct {
	Name    string    `json:"name"`
	Href    string    `json:"href"`
	Size    int64     `json:"size"`
	ModTime time.Time `json:"modTime"`
	IsDir   bool      `json:"isDir"`
}

// resolve 拼接基础地址与相对路径。
func (c *Client) resolve(rel string) (string, error) {
	base := strings.TrimSpace(c.BaseURL)
	if base == "" {
		return "", i18n.Errorf("err.webdavNoUrl")
	}
	if !strings.HasPrefix(base, "http://") && !strings.HasPrefix(base, "https://") {
		base = "https://" + base
	}
	u, err := url.Parse(base)
	if err != nil {
		return "", i18n.Errorf("err.webdavBadUrl", err)
	}
	rel = strings.Trim(strings.ReplaceAll(rel, "\\", "/"), "/")
	if rel == "" {
		u.Path = strings.TrimRight(u.Path, "/") + "/"
		return u.String(), nil
	}
	segs := []string{}
	for _, s := range strings.Split(strings.Trim(u.Path, "/"), "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	for _, s := range strings.Split(rel, "/") {
		if s != "" {
			segs = append(segs, s)
		}
	}
	u.Path = "/" + path.Join(segs...)
	u.RawQuery = ""
	return u.String(), nil
}

func (c *Client) do(ctx context.Context, method, target string, body []byte, headers map[string]string) (*http.Response, error) {
	var rdr io.Reader
	if body != nil {
		rdr = bytes.NewReader(body)
	}
	req, err := http.NewRequestWithContext(ctx, method, target, rdr)
	if err != nil {
		return nil, err
	}
	if c.Username != "" || c.Password != "" {
		req.SetBasicAuth(c.Username, c.Password)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := c.HTTP.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func readBody(resp *http.Response) string {
	b, _ := io.ReadAll(io.LimitReader(resp.Body, 2048))
	return strings.TrimSpace(string(b))
}

// EnsureDir 逐级创建远端目录（已存在时忽略 405）。
func (c *Client) EnsureDir(ctx context.Context, dir string) error {
	dir = strings.Trim(strings.ReplaceAll(dir, "\\", "/"), "/")
	if dir == "" {
		return nil
	}
	segs := strings.Split(dir, "/")
	for i := range segs {
		sub := strings.Join(segs[:i+1], "/")
		target, err := c.resolve(sub)
		if err != nil {
			return err
		}
		resp, err := c.do(ctx, "MKCOL", target, nil, nil)
		if err != nil {
			return i18n.Errorf("err.webdavMkcol", err)
		}
		code := resp.StatusCode
		body := readBody(resp)
		resp.Body.Close()
		switch code {
		case 200, 201, 204, 405: // OK / Created / No Content / 已存在
		default:
			return i18n.Errorf("err.webdavMkcolStatus", sub, code, body)
		}
	}
	return nil
}

// Put 上传文件。
func (c *Client) Put(ctx context.Context, rel string, data []byte) error {
	target, err := c.resolve(rel)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodPut, target, data, map[string]string{
		"Content-Type": "application/json; charset=utf-8",
	})
	if err != nil {
		return i18n.Errorf("err.webdavUpload", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return i18n.Errorf("err.webdavUploadStatus", resp.StatusCode, readBody(resp))
	}
	return nil
}

// Get 下载文件。
func (c *Client) Get(ctx context.Context, rel string) ([]byte, error) {
	target, err := c.resolve(rel)
	if err != nil {
		return nil, err
	}
	resp, err := c.do(ctx, http.MethodGet, target, nil, nil)
	if err != nil {
		return nil, i18n.Errorf("err.webdavDownload", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, i18n.Errorf("err.webdavDownloadStatus", resp.StatusCode, readBody(resp))
	}
	return io.ReadAll(io.LimitReader(resp.Body, 64<<20))
}

// Delete 删除远端文件。
func (c *Client) Delete(ctx context.Context, rel string) error {
	target, err := c.resolve(rel)
	if err != nil {
		return err
	}
	resp, err := c.do(ctx, http.MethodDelete, target, nil, nil)
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return i18n.Errorf("err.webdavDelete", resp.StatusCode, readBody(resp))
	}
	return nil
}

type multistatus struct {
	Responses []struct {
		Href     string `xml:"href"`
		Propstat []struct {
			Status string `xml:"status"`
			Prop   struct {
				DisplayName   string `xml:"displayname"`
				ContentLength string `xml:"getcontentlength"`
				LastModified  string `xml:"getlastmodified"`
				ResourceType  struct {
					Collection *struct{} `xml:"collection"`
				} `xml:"resourcetype"`
			} `xml:"prop"`
		} `xml:"propstat"`
	} `xml:"response"`
}

// List 列出目录下的文件（Depth: 1），按修改时间倒序。
func (c *Client) List(ctx context.Context, dir string) ([]RemoteFile, error) {
	target, err := c.resolve(dir)
	if err != nil {
		return nil, err
	}
	body := []byte(`<?xml version="1.0" encoding="utf-8" ?>
<d:propfind xmlns:d="DAV:">
  <d:prop>
    <d:displayname/>
    <d:getcontentlength/>
    <d:getlastmodified/>
    <d:resourcetype/>
  </d:prop>
</d:propfind>`)
	resp, err := c.do(ctx, "PROPFIND", target, body, map[string]string{
		"Depth":        "1",
		"Content-Type": "application/xml; charset=utf-8",
	})
	if err != nil {
		return nil, i18n.Errorf("err.webdavList", err)
	}
	defer resp.Body.Close()
	raw, _ := io.ReadAll(io.LimitReader(resp.Body, 4<<20))
	if resp.StatusCode == 404 {
		return []RemoteFile{}, nil
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return nil, i18n.Errorf("err.webdavListStatus", resp.StatusCode, strings.TrimSpace(string(raw)))
	}

	var ms multistatus
	if err := xml.Unmarshal(raw, &ms); err != nil {
		return nil, i18n.Errorf("err.webdavParseList", err)
	}
	basePath := ""
	if u, err := url.Parse(target); err == nil {
		basePath = strings.TrimRight(u.Path, "/")
	}
	out := []RemoteFile{}
	for _, r := range ms.Responses {
		href := r.Href
		decoded := href
		if u, err := url.Parse(href); err == nil {
			decoded = u.Path
		}
		if strings.TrimRight(decoded, "/") == basePath {
			continue // 目录自身
		}
		f := RemoteFile{Href: href}
		if len(r.Propstat) > 0 {
			p := r.Propstat[0].Prop
			f.Name = p.DisplayName
			if f.Name == "" {
				f.Name = path.Base(strings.TrimRight(decoded, "/"))
			}
			fmt.Sscanf(p.ContentLength, "%d", &f.Size)
			f.ModTime = parseHTTPDate(p.LastModified)
			f.IsDir = p.ResourceType.Collection != nil
		}
		if f.Name == "" {
			continue
		}
		out = append(out, f)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].ModTime.After(out[j].ModTime) })
	return out, nil
}

var dateLayouts = []string{
	time.RFC1123, time.RFC1123Z, time.RFC850, time.ANSIC,
	"Mon, 2 Jan 2006 15:04:05 MST",
	"2006-01-02T15:04:05Z07:00",
	"2006-01-02T15:04:05Z",
}

func parseHTTPDate(s string) time.Time {
	s = strings.TrimSpace(s)
	if s == "" {
		return time.Time{}
	}
	for _, l := range dateLayouts {
		if t, err := time.Parse(l, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// Test 验证连通性（尝试列根目录），lang 决定返回文案的语言。
func (c *Client) Test(ctx context.Context, lang string) (string, error) {
	files, err := c.List(ctx, "")
	if err != nil {
		return "", err
	}
	return i18n.T(lang, "webdav.testOk", len(files)), nil
}
