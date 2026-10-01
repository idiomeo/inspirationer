// Command inspirationer 启动本地灵感管理服务。
//
// Windows 下编译为 GUI 子系统程序：不显示黑色终端窗口，只在系统托盘出现一个
// 小灯泡图标（左键打开界面、右键菜单），启动成功后自动打开浏览器。
package main

import (
	"context"
	"embed"
	"encoding/json"
	"flag"
	"fmt"
	"io"
	"io/fs"
	"log"
	"net"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"syscall"
	"time"

	"inspirationer/internal/platform"
	"inspirationer/internal/server"
	"inspirationer/internal/store"
	"inspirationer/internal/tray"
)

//go:embed web
var embeddedWeb embed.FS

const (
	version   = "1.1.0"
	appName   = "灵感管理器"
	mutexName = `Local\Inspirationer-Singleton`
	logMaxLen = 4 << 20 // 单个日志文件上限 4MB
)

func main() {
	addr := flag.String("addr", "127.0.0.1:8420", "监听地址")
	dataDir := flag.String("data", "", "数据目录（默认 ./data）")
	webDir := flag.String("dev-web", "", "从磁盘目录加载前端资源（开发调试用，默认用内嵌资源）")
	openUI := flag.Bool("open", true, "启动成功后自动用默认浏览器打开界面")
	useTray := flag.Bool("tray", true, "在系统托盘显示图标（Windows）")
	console := flag.Bool("console", false, "额外显示控制台窗口以查看实时日志")
	single := flag.Bool("single-instance", true, "只允许运行一个实例（重复启动会打开已有实例）")
	showVersion := flag.Bool("version", false, "打印版本号后退出")
	flag.Parse()

	// 控制台（可选）：GUI 子系统默认没有控制台，调试时用 -console 打开
	if *console {
		if !platform.AllocConsole() {
			platform.AttachParentConsole()
		}
	}
	platform.SetDPIAware()

	if *showVersion {
		text := appName + " v" + version
		if platform.HasConsole() {
			fmt.Println(text)
		} else {
			platform.MessageBox(appName, text)
		}
		return
	}

	// ---------------------------------------------------------- 数据目录与日志
	absData, err := resolveDataDir(*dataDir)
	if err != nil {
		platform.ErrorBox(appName+" 启动失败", "无法创建数据目录：\n\n"+err.Error())
		return
	}
	logger, logPath, closeLog := newLogger(absData)
	defer closeLog()

	// ---------------------------------------------------------- 单实例
	if *single {
		release, already := platform.SingleInstance(mutexName)
		if already {
			handleExistingInstance(logger, absData, *openUI)
			return
		}
		defer release()
	}

	// ---------------------------------------------------------- 存储
	st, err := store.New(absData)
	if err != nil {
		fatal(logger, logPath, "初始化数据目录失败：%v", err)
		return
	}

	webFS, err := fs.Sub(embeddedWeb, "web")
	if err != nil {
		fatal(logger, logPath, "加载内嵌前端资源失败：%v", err)
		return
	}
	if strings.TrimSpace(*webDir) != "" {
		absWeb, err := filepath.Abs(*webDir)
		if err != nil {
			fatal(logger, logPath, "前端目录无效：%v", err)
			return
		}
		if _, err := os.Stat(filepath.Join(absWeb, "index.html")); err != nil {
			fatal(logger, logPath, "前端目录里找不到 index.html：%s", absWeb)
			return
		}
		webFS = os.DirFS(absWeb)
		logger.Printf("开发模式：前端资源来自磁盘 %s", absWeb)
	}

	srv := server.New(st, webFS, version, logger)

	// ---------------------------------------------------------- 监听
	ln, finalAddr, err := listen(*addr)
	if err != nil {
		fatal(logger, logPath,
			"无法监听 %s：\n%v\n\n提示：端口可能已被占用，可用 -addr 127.0.0.1:其他端口 指定其它端口。",
			*addr, err)
		return
	}

	httpSrv := &http.Server{
		Handler:     srv.Routes(),
		IdleTimeout: 120 * time.Second,
	}

	url := "http://" + hostForBrowser(finalAddr) + "/"
	writeRuntimeFile(absData, url)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	shutdownCh := make(chan struct{})
	var shutdownOnce sync.Once
	requestShutdown := func(reason string) {
		shutdownOnce.Do(func() {
			logger.Printf("收到退出请求（%s），正在关闭…", reason)
			close(shutdownCh)
		})
	}

	// ---------------------------------------------------------- 先让服务跑起来
	// 顺序很重要：HTTP 服务先启动，这样即使托盘/弹窗出问题，界面依然可用。
	go func() {
		if err := httpSrv.Serve(ln); err != nil && err != http.ErrServerClosed {
			logger.Printf("服务异常退出：%v", err)
			requestShutdown("服务异常")
		}
	}()

	// ---------------------------------------------------------- 托盘
	var tr *tray.Tray
	if *useTray {
		tr = tray.New(appName+" · "+url, func() { openBrowser(logger, url) })
		tr.SetDebugLogger(logger.Printf)
		tr.AddItem("打开灵感管理器", func() { openBrowser(logger, url) })
		tr.AddItem("打开数据目录", func() { openPath(logger, absData) })
		tr.AddItem("立即备份到 WebDAV", func() { backupNow(logger, srv, tr) })
		tr.AddSeparator()
		tr.AddItem("查看日志文件", func() { openPath(logger, logPath) })
		tr.AddSeparator()
		tr.AddItem("退出", func() { requestShutdown("托盘菜单") })
		tr.SetBalloon(appName+" 已启动", url)
		if err := tr.Start(); err != nil {
			// 托盘不可用时也要保证程序"可被看见、可被退出"：
			// 打开日志控制台（可 Ctrl+C），并在后台弹一次说明（不阻塞服务）。
			logger.Printf("托盘图标注册失败（服务照常运行）：%v", err)
			consoleOpened := false
			if !platform.HasConsole() {
				consoleOpened = platform.AllocConsole()
				if consoleOpened {
					logger.Println("已打开日志控制台窗口，按 Ctrl+C 可退出程序")
				}
			}
			hint := ""
			if consoleOpened {
				hint = "\n已打开日志控制台窗口：可按 Ctrl+C 退出。"
			}
			go platform.MessageBox(appName,
				"托盘图标注册失败，但服务已经正常启动。\n\n原因："+err.Error()+
					"\n\n访问地址："+url+"\n数据目录："+absData+hint)
		} else {
			logger.Printf("托盘图标已就绪：左键单击打开界面，右键打开菜单")
		}
	}

	// ---------------------------------------------------------- 启动
	srv.StartAutoBackup(ctx)
	maybeSnapshot(st, logger)

	logger.Printf("%s v%s 已启动", appName, version)
	logger.Printf("  访问地址: %s", url)
	logger.Printf("  数据目录: %s", absData)
	if logPath != "" {
		logger.Printf("  日志文件: %s", logPath)
	}
	if tr != nil && tr.Ready() {
		logger.Printf("  退出方式: 托盘图标右键 → 退出")
	} else {
		logger.Printf("  退出方式: 按 Ctrl+C（或结束进程）")
	}

	if *openUI {
		go func() {
			time.Sleep(400 * time.Millisecond)
			openBrowser(logger, url)
		}()
	}

	select {
	case <-ctx.Done():
		logger.Println("收到系统信号，正在关闭…")
	case <-shutdownCh:
	}

	if tr != nil {
		tr.Stop()
	}
	shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(shutdownCtx)
	removeRuntimeFile(absData)
	logger.Println("已退出，数据已保存在本地。")
}

/* ------------------------------------------------------------------ 日志 */

// resolveDataDir 决定数据目录：
//  1. 显式指定 -data 时严格使用它；
//  2. 否则用「当前工作目录/data」（双击 exe 或 .bat 启动时就是 exe 同级目录）；
//  3. 若当前目录不可写（例如从系统目录被拉起），退回到 exe 所在目录/data。
func resolveDataDir(flagValue string) (string, error) {
	explicit := strings.TrimSpace(flagValue) != ""
	root := strings.TrimSpace(flagValue)
	if root == "" {
		root = "data"
	}
	cand, err := filepath.Abs(root)
	if err != nil {
		cand = root
	}
	if mkErr := os.MkdirAll(cand, 0o755); mkErr == nil {
		if probeErr := writeProbe(cand); probeErr == nil {
			return cand, nil
		} else if explicit {
			return "", fmt.Errorf("%s 不可写：%v", cand, probeErr)
		}
	} else if explicit {
		return "", mkErr
	}

	exe, exeErr := os.Executable()
	if exeErr != nil {
		return "", fmt.Errorf("无法确定程序所在目录：%v", exeErr)
	}
	alt := filepath.Join(filepath.Dir(exe), "data")
	if err := os.MkdirAll(alt, 0o755); err != nil {
		return "", err
	}
	return alt, nil
}

// writeProbe 真正写一个临时文件确认目录可写（只读目录上 MkdirAll 也可能“成功”）。
func writeProbe(dir string) error {
	f, err := os.CreateTemp(dir, ".write-test-*")
	if err != nil {
		return err
	}
	name := f.Name()
	_ = f.Close()
	return os.Remove(name)
}

// newLogger 把日志写到 <data>/logs/inspirationer.log，若存在控制台则同时输出。
func newLogger(dataDir string) (*log.Logger, string, func()) {
	logDir := filepath.Join(dataDir, "logs")
	if err := os.MkdirAll(logDir, 0o755); err != nil {
		return log.New(os.Stderr, "", log.LstdFlags), "", func() {}
	}
	path := filepath.Join(logDir, "inspirationer.log")
	rotateLog(path, logMaxLen)
	f, err := os.OpenFile(path, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0o644)
	if err != nil {
		return log.New(os.Stderr, "", log.LstdFlags), "", func() {}
	}
	return log.New(fileAndConsoleWriter{file: f}, "", log.LstdFlags), path, func() { f.Close() }
}

// fileAndConsoleWriter 始终写日志文件；如果进程有控制台（例如 -console 或降级打开），
// 同时输出到控制台，这样运行期后开的控制台也能看到日志。
type fileAndConsoleWriter struct{ file *os.File }

func (w fileAndConsoleWriter) Write(b []byte) (int, error) {
	n, err := w.file.Write(b)
	if platform.HasConsole() {
		_, _ = os.Stdout.Write(b)
	}
	return n, err
}

// rotateLog 超过阈值时把日志滚动为 .1（只保留一代，够用且简单）。
func rotateLog(path string, max int64) {
	info, err := os.Stat(path)
	if err != nil || info.Size() < max {
		return
	}
	_ = os.Remove(path + ".1")
	_ = os.Rename(path, path+".1")
}

func fatal(logger *log.Logger, logPath, format string, args ...interface{}) {
	msg := fmt.Sprintf(format, args...)
	logger.Printf("启动失败：%s", msg)
	text := msg
	if logPath != "" {
		text += "\n\n日志文件：" + logPath
	}
	platform.ErrorBox(appName+" - 启动失败", text)
}

/* ------------------------------------------------------------ 运行期状态 */

// runtimeInfo 写在数据目录里，便于第二个实例找到已在运行的地址。
type runtimeInfo struct {
	PID       int    `json:"pid"`
	URL       string `json:"url"`
	StartedAt string `json:"startedAt"`
}

func runtimeFilePath(dataDir string) string { return filepath.Join(dataDir, "runtime.json") }

func writeRuntimeFile(dataDir, url string) {
	info := runtimeInfo{PID: os.Getpid(), URL: url, StartedAt: time.Now().Format(time.RFC3339)}
	b, err := json.MarshalIndent(info, "", "  ")
	if err != nil {
		return
	}
	_ = os.WriteFile(runtimeFilePath(dataDir), b, 0o644)
}

func readRuntimeURL(dataDir string) string {
	b, err := os.ReadFile(runtimeFilePath(dataDir))
	if err != nil {
		return ""
	}
	var info runtimeInfo
	if json.Unmarshal(b, &info) != nil {
		return ""
	}
	return strings.TrimSpace(info.URL)
}

func removeRuntimeFile(dataDir string) {
	_ = os.Remove(runtimeFilePath(dataDir))
}

// handleExistingInstance 处理「已经有实例在跑」的情况：直接打开它的界面。
func handleExistingInstance(logger *log.Logger, dataDir string, openIt bool) {
	url := readRuntimeURL(dataDir)
	if url != "" && probeServer(url) {
		logger.Printf("检测到已有实例正在运行：%s", url)
		if openIt {
			openBrowser(logger, url)
		}
		return
	}
	logger.Println("检测到已有实例正在运行，但无法确认其地址")
	platform.MessageBox(appName,
		"已经有一个灵感管理器在运行了。\n\n请查看屏幕右下角系统托盘里的 💡 图标（可能需要点开“显示隐藏的图标”）。")
}

// probeServer 探测某个地址上是否真的跑着本程序。
func probeServer(base string) bool {
	client := &http.Client{Timeout: 1500 * time.Millisecond}
	resp, err := client.Get(strings.TrimRight(base, "/") + "/api/bootstrap")
	if err != nil {
		return false
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, io.LimitReader(resp.Body, 4096))
	return resp.StatusCode == http.StatusOK
}

/* ------------------------------------------------------------ 小工具 */

func openBrowser(logger *log.Logger, url string) {
	if err := platform.OpenURL(url); err != nil {
		logger.Printf("打开浏览器失败：%v（请手动访问 %s）", err, url)
		return
	}
	logger.Printf("已请求系统默认浏览器打开：%s", url)
}

func openPath(logger *log.Logger, path string) {
	if path == "" {
		return
	}
	if err := platform.OpenURL(path); err != nil {
		logger.Printf("打开 %s 失败：%v", path, err)
	}
}

func backupNow(logger *log.Logger, srv *server.Server, tr *tray.Tray) {
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Minute)
	defer cancel()
	info, err := srv.BackupToWebDAV(ctx)
	if err != nil {
		logger.Printf("手动备份失败：%v", err)
		if tr != nil {
			tr.Notify(appName+"：备份失败", err.Error())
		}
		return
	}
	msg := fmt.Sprintf("%s（%.1f KB）", info.File, float64(info.Bytes)/1024)
	logger.Printf("手动备份成功：%s", msg)
	if tr != nil {
		tr.Notify(appName+"：备份成功", msg)
	}
}

// listen 在端口被占用时自动向后尝试若干端口。
func listen(addr string) (net.Listener, string, error) {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		host, port = "127.0.0.1", "8420"
	}
	startPort := 0
	fmt.Sscanf(port, "%d", &startPort)
	if startPort == 0 {
		startPort = 8420
	}
	var lastErr error
	for i := 0; i < 25; i++ {
		try := net.JoinHostPort(host, fmt.Sprintf("%d", startPort+i))
		ln, err := net.Listen("tcp", try)
		if err == nil {
			return ln, try, nil
		}
		lastErr = err
	}
	return nil, "", lastErr
}

func hostForBrowser(addr string) string {
	host, port, err := net.SplitHostPort(addr)
	if err != nil {
		return addr
	}
	if host == "" || host == "0.0.0.0" || host == "::" {
		host = "127.0.0.1"
	}
	return net.JoinHostPort(host, port)
}

// maybeSnapshot 启动时写一份本地快照（距上次快照 6 小时以上才写）。
func maybeSnapshot(st *store.Store, logger *log.Logger) {
	dir := filepath.Join(st.Dir(), "backups")
	entries, err := os.ReadDir(dir)
	if err == nil {
		names := []string{}
		for _, e := range entries {
			if !e.IsDir() && strings.HasSuffix(e.Name(), ".json") {
				names = append(names, e.Name())
			}
		}
		if len(names) > 0 {
			sort.Strings(names)
			if info, err := os.Stat(filepath.Join(dir, names[len(names)-1])); err == nil {
				if time.Since(info.ModTime()) < 6*time.Hour {
					return
				}
			}
		}
	}
	if path, err := st.Snapshot(30); err == nil {
		logger.Printf("已创建本地快照：%s", path)
	}
}
