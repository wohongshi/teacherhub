package main

// teacherhub —— 教师工作台（重写版）
// 云端（teacherhub serve）：零账号、统一访问 key，只管两件事——
//   ① 每位教师的 ima 知识库贡献统计；② 模型工作台产物库（相似检索 / 下发链接 / 统计页）。
// 本地（teacherhub ui/config/ping/contrib/search/push/stats）：
//   纯本地干活，配置走自带 WebUI（云端地址 / 教师姓名 / 访问 key），产物生成后自动推送云端。

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"sort"
	"strings"
	"time"
)

const version = "1.1.0"

func main() {
	args := os.Args[1:]
	cmd := "help"
	if len(args) > 0 && !strings.HasPrefix(args[0], "-") {
		cmd = args[0]
		args = args[1:]
	}
	switch cmd {
	case "serve":
		cmdServe(args)
	case "ui":
		cmdUI(args)
	case "config":
		cmdConfig(args)
	case "ping":
		cmdPing(args)
	case "contrib":
		cmdContrib(args)
	case "search":
		cmdSearch(args)
	case "push":
		cmdPush(args)
	case "stats":
		cmdStats(args)
	case "scan":
		cmdScan(args)
	default:
		usage()
	}
}

func usage() {
	fmt.Println(`teacherhub v` + version + ` —— 教师工作台（本地插件 + 轻云端）

云端（部署到云服务器）：
  serve     启动云端服务 --addr :8096 --data ./data --key 访问key（留空自动生成）

本地（每位老师的电脑 / WorkBuddy 插件调用）：
  ui        打开配置网页（云端地址 / 教师姓名 / 访问 key），保存即用
  config    命令行配置：--cloud 地址 --name 姓名 --key key（不带参数=查看）
  ping      测云端连通
  contrib   报贡献：contrib "添加了《钠与水》到化学实验库" [--kb 知识库] [--item 条目]
  search    检索云端有无类似工作台：search "钠与水 实验"
  push      推送产物：push 文件.html --title 标题 --tags 标签1,标签2 --desc 简介
  stats     看大家最近的贡献统计
  scan      定时扫描 ImaHub 近期创建/上传，自动把文件名报成贡献（配 cron/插件定时跑）

说明：全程零账号；key 全员统一一把（不外传给学生）；工作台产物=单文件 HTML。`)
}

// ---------- 本地配置（~/.teacherhub/config.json） ----------

type localConfig struct {
	Cloud string `json:"cloud"` // 云端地址，如 https://hub.example.com
	Name  string `json:"name"`  // 教师姓名（署名用）
	Key   string `json:"key"`   // 统一访问 key
	Imahub    string `json:"imahub,omitempty"`    // ImaHub 地址（scan 数据源，默认 http://127.0.0.1:8090）
	ImaToken  string `json:"ima_token,omitempty"` // ImaHub MCP token（Bearer 鉴权）
	LastScan  int64  `json:"last_scan,omitempty"` // scan 增量水位（unix 秒）
}

func configPath() string {
	home, err := os.UserHomeDir()
	if err != nil {
		return ".teacherhub/config.json"
	}
	return filepath.Join(home, ".teacherhub", "config.json")
}

func loadConfig() localConfig {
	var c localConfig
	b, err := os.ReadFile(configPath())
	if err == nil {
		_ = json.Unmarshal(b, &c)
	}
	return c
}

func saveConfig(c localConfig) error {
	p := configPath()
	if err := os.MkdirAll(filepath.Dir(p), 0o755); err != nil {
		return err
	}
	b, _ := json.MarshalIndent(c, "", "  ")
	return os.WriteFile(p, b, 0o600)
}

// ---------- 云端调用 ----------

func cloudURL() string {
	c := loadConfig()
	if c.Cloud == "" {
		fmt.Println("还没配置云端地址。先跑：teacherhub ui")
		os.Exit(1)
	}
	return strings.TrimRight(c.Cloud, "/")
}

// apiCall 云端 API 请求（统一走 X-Access-Key），返回解好的 JSON。
func apiCall(method, path string, body any, query string) (map[string]any, error) {
	c := loadConfig()
	var rd *strings.Reader
	if body != nil {
		b, _ := json.Marshal(body)
		rd = strings.NewReader(string(b))
	} else {
		rd = strings.NewReader("")
	}
	url := strings.TrimRight(c.Cloud, "/") + path
	if query != "" {
		url += "?" + query
	}
	req, err := http.NewRequest(method, url, rd)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Access-Key", c.Key)
	cli := &http.Client{Timeout: 20 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		return nil, fmt.Errorf("连不上云端：%w", err)
	}
	defer resp.Body.Close()
	var out map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&out); err != nil {
		return nil, fmt.Errorf("云端响应异常（HTTP %d）", resp.StatusCode)
	}
	if resp.StatusCode >= 400 || out["ok"] == false {
		msg, _ := out["error"].(string)
		return out, fmt.Errorf("云端拒绝：%s", msg)
	}
	return out, nil
}

// ---------- 本地命令 ----------

// reorderArgs 把「位置参数在前、旗子在后」的自然写法排成 flag 包能吃的顺序（旗子带值一起前移）。
func reorderArgs(args []string) []string {
	flags := []string{}
	pos := []string{}
	for i := 0; i < len(args); i++ {
		a := args[i]
		if strings.HasPrefix(a, "-") {
			flags = append(flags, a)
			if i+1 < len(args) && !strings.HasPrefix(args[i+1], "-") {
				flags = append(flags, args[i+1])
				i++
			}
		} else {
			pos = append(pos, a)
		}
	}
	return append(flags, pos...)
}

func cmdConfig(args []string) {
	fl := flag.NewFlagSet("config", flag.ExitOnError)
	cloud := fl.String("cloud", "", "云端地址")
	name := fl.String("name", "", "教师姓名")
	key := fl.String("key", "", "统一访问 key")
	imahub := fl.String("imahub", "", "ImaHub 地址（scan 用）")
	imatoken := fl.String("imatoken", "", "ImaHub MCP token（scan 用）")
	fl.Parse(reorderArgs(args))
	c := loadConfig()
	changed := false
	if *cloud != "" {
		c.Cloud = strings.TrimSpace(*cloud)
		changed = true
	}
	if *name != "" {
		c.Name = strings.TrimSpace(*name)
		changed = true
	}
	if *key != "" {
		c.Key = strings.TrimSpace(*key)
		changed = true
	}
	if *imahub != "" {
		c.Imahub = strings.TrimSpace(*imahub)
		changed = true
	}
	if *imatoken != "" {
		c.ImaToken = strings.TrimSpace(*imatoken)
		changed = true
	}
	if changed {
		if err := saveConfig(c); err != nil {
			fmt.Println("保存失败:", err)
			os.Exit(1)
		}
		fmt.Println("已保存:", configPath())
	}
	masked := c.Key
	if rr := []rune(masked); len(rr) > 8 {
		masked = string(rr[:4]) + "…" + string(rr[len(rr)-2:])
	}
	fmt.Printf("云端地址: %s\n教师姓名: %s\n访问 key: %s\nImaHub: %s\n", orDash(c.Cloud), orDash(c.Name), orDash(masked), orDash(c.Imahub))
}

func orDash(s string) string {
	if s == "" {
		return "（未设置）"
	}
	return s
}

func cmdPing(args []string) {
	out, err := apiCall("GET", "/api/ping", nil, "")
	if err != nil {
		fmt.Println("✗", err)
		os.Exit(1)
	}
	fmt.Printf("✓ 云端在线（v%v）\n", out["version"])
}

func cmdScan(args []string) {
	fl := flag.NewFlagSet("scan", flag.ExitOnError)
	days := fl.Int("days", 7, "首跑回看天数")
	fl.Parse(reorderArgs(args))
	c := loadConfig()
	if c.Cloud == "" || c.Name == "" {
		fmt.Println("还没配置云端/姓名。先跑：teacherhub ui")
		os.Exit(1)
	}
	im := c.Imahub
	if im == "" {
		im = "http://127.0.0.1:8090"
	}
	req, err := http.NewRequest("GET", strings.TrimRight(im, "/")+"/api/dashboard", nil)
	if err != nil {
		fmt.Println("✗", err)
		os.Exit(1)
	}
	if c.ImaToken != "" {
		req.Header.Set("Authorization", "Bearer "+c.ImaToken)
	}
	cli := &http.Client{Timeout: 20 * time.Second}
	resp, err := cli.Do(req)
	if err != nil {
		fmt.Println("✗ 连不上 ImaHub:", err)
		os.Exit(1)
	}
	defer resp.Body.Close()
	var out map[string]any
	_ = json.NewDecoder(resp.Body).Decode(&out)
	if resp.StatusCode != 200 {
		fmt.Println("✗ ImaHub 返回:", resp.StatusCode, out["msg"])
		os.Exit(1)
	}
	data, _ := out["data"].(map[string]any)
	rec, _ := data["recent"].([]any)
	watermark := c.LastScan
	if watermark == 0 {
		watermark = time.Now().AddDate(0, 0, -*days).Unix()
	}
	type scanItem struct {
		kind, title, kb string
		at              int64
	}
	var fresh []scanItem
	for _, r := range rec {
		m, _ := r.(map[string]any)
		if m == nil {
			continue
		}
		at := int64(0)
		if v, ok := m["at"].(float64); ok {
			at = int64(v)
		}
		title, _ := m["title"].(string)
		if at <= watermark || title == "" {
			continue
		}
		kind, _ := m["kind"].(string)
		kb, _ := m["kb_name"].(string)
		fresh = append(fresh, scanItem{kind, title, kb, at})
	}
	if len(fresh) == 0 {
		fmt.Println("没有新条目")
		return
	}
	sort.Slice(fresh, func(i, j int) bool { return fresh[i].at < fresh[j].at })
	maxAt := watermark
	n := 0
	for _, it := range fresh {
		text := "创建了《" + it.title + "》"
		if it.kind == "upload" {
			text = "上传了《" + it.title + "》"
		}
		if _, err := apiCall("POST", "/api/contrib", map[string]any{
			"teacher": c.Name, "text": text, "kb": it.kb, "item": it.title,
		}, ""); err != nil {
			fmt.Println("✗ 上报失败:", it.title, err)
			continue
		}
		n++
		if it.at > maxAt {
			maxAt = it.at
		}
	}
	c.LastScan = maxAt
	_ = saveConfig(c)
	fmt.Printf("✓ 已上报 %d 条新资料动态\n", n)
}

func cmdContrib(args []string) {
	fl := flag.NewFlagSet("contrib", flag.ExitOnError)
	kb := fl.String("kb", "", "知识库名（选填）")
	item := fl.String("item", "", "条目名（选填）")
	fl.Parse(reorderArgs(args))
	if fl.NArg() < 1 {
		fmt.Println(`用法：teacherhub contrib "添加了《钠与水》到化学实验库" [--kb 化学实验库] [--item 钠与水]`)
		os.Exit(1)
	}
	c := loadConfig()
	if c.Name == "" {
		fmt.Println("还没配置教师姓名。先跑：teacherhub ui")
		os.Exit(1)
	}
	out, err := apiCall("POST", "/api/contrib", map[string]any{
		"teacher": c.Name, "text": fl.Arg(0), "kb": *kb, "item": *item,
	}, "")
	if err != nil {
		fmt.Println("✗", err)
		os.Exit(1)
	}
	fmt.Println("✓ 已记录贡献", out["id"])
}

func cmdSearch(args []string) {
	if len(args) < 1 {
		fmt.Println(`用法：teacherhub search "钠与水 实验"`)
		os.Exit(1)
	}
	q := strings.Join(args, " ")
	out, err := apiCall("GET", "/api/workbenches/search", nil, "q="+url.QueryEscape(q)+"&limit=5")
	if err != nil {
		fmt.Println("✗", err)
		os.Exit(1)
	}
	matches, _ := out["matches"].([]any)
	if len(matches) == 0 {
		fmt.Println("云端没有类似产物（可以生成新的）")
		return
	}
	fmt.Printf("云端有 %d 个类似产物：\n", len(matches))
	for i, m := range matches {
		mm, _ := m.(map[string]any)
		wb, _ := mm["workbench"].(map[string]any)
		fmt.Printf("%d. %v（相似度 %.2f）\n   %v\n", i+1, wb["title"], mm["score"], wb["link"])
	}
}

func cmdPush(args []string) {
	fl := flag.NewFlagSet("push", flag.ExitOnError)
	title := fl.String("title", "", "标题")
	desc := fl.String("desc", "", "简介")
	tags := fl.String("tags", "", "标签（逗号分隔）")
	keywords := fl.String("keywords", "", "检索关键词（逗号分隔）")
	fl.Parse(reorderArgs(args))
	if fl.NArg() < 1 || *title == "" {
		fmt.Println(`用法：teacherhub push 文件.html --title 标题 [--desc 简介] [--tags 化学,实验] [--keywords 钠,水,反应]`)
		os.Exit(1)
	}
	content, err := os.ReadFile(fl.Arg(0))
	if err != nil {
		fmt.Println("✗ 读文件失败:", err)
		os.Exit(1)
	}
	c := loadConfig()
	out, err := apiCall("POST", "/api/workbenches", map[string]any{
		"title": *title, "desc": *desc,
		"tags": splitCSV(*tags), "keywords": splitCSV(*keywords),
		"teacher": c.Name, "content": string(content),
	}, "")
	if err != nil {
		fmt.Println("✗", err)
		os.Exit(1)
	}
	fmt.Println("✓ 已推送云端")
	fmt.Println("链接:", out["link"])
}

func cmdStats(args []string) {
	out, err := apiCall("GET", "/api/stats", nil, "")
	if err != nil {
		fmt.Println("✗", err)
		os.Exit(1)
	}
	byT, _ := out["by_teacher"].([]any)
	fmt.Printf("工作台产物 %v 个 · 贡献记录 %v 条\n\n", out["total_workbenches"], out["total_contribs"])
	fmt.Println("按教师统计：")
	for _, t := range byT {
		tt, _ := t.(map[string]any)
		fmt.Printf("  %v：贡献 %v 条 · 工作台 %v 个\n", tt["teacher"], tt["contribs"], tt["workbenches"])
	}
	rec, _ := out["recent"].([]any)
	if len(rec) > 0 {
		fmt.Println("\n最近贡献：")
		for _, r := range rec {
			rr, _ := r.(map[string]any)
			fmt.Printf("  [%v] %v：%v\n", formatTS(rr["at"]), rr["teacher"], rr["text"])
		}
	}
}

// ---------- 配置 WebUI ----------

func cmdUI(args []string) {
	fl := flag.NewFlagSet("ui", flag.ExitOnError)
	port := fl.String("port", "8791", "本地端口")
	fl.Parse(args)
	addr := "127.0.0.1:" + *port
	mux := http.NewServeMux()
	mux.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(configPageHTML(loadConfig())))
	})
	mux.HandleFunc("/save", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "POST only", 405)
			return
		}
		var c localConfig
		if err := json.NewDecoder(r.Body).Decode(&c); err != nil {
			http.Error(w, "bad json", 400)
			return
		}
		c.Cloud = strings.TrimSpace(c.Cloud)
		c.Name = strings.TrimSpace(c.Name)
		c.Key = strings.TrimSpace(c.Key)
		if err := saveConfig(c); err != nil {
			http.Error(w, err.Error(), 500)
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	mux.HandleFunc("/test", func(w http.ResponseWriter, r *http.Request) {
		var c localConfig
		_ = json.NewDecoder(r.Body).Decode(&c)
		req, _ := http.NewRequest("GET", strings.TrimRight(c.Cloud, "/")+"/api/ping", nil)
		req.Header.Set("X-Access-Key", c.Key)
		cli := &http.Client{Timeout: 8 * time.Second}
		resp, err := cli.Do(req)
		if err != nil {
			_, _ = w.Write([]byte(`{"ok":false,"error":"连不上云端"}`))
			return
		}
		defer resp.Body.Close()
		var out map[string]any
		_ = json.NewDecoder(resp.Body).Decode(&out)
		if resp.StatusCode != 200 || out["ok"] != true {
			_, _ = w.Write([]byte(`{"ok":false,"error":"key 不对或服务异常（HTTP ` + fmt.Sprint(resp.StatusCode) + `）"}`))
			return
		}
		_, _ = w.Write([]byte(`{"ok":true}`))
	})
	fmt.Println("配置页已打开:", "http://"+addr)
	openBrowser("http://" + addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		fmt.Println("启动失败:", err)
		os.Exit(1)
	}
}

func openBrowser(url string) {
	var cmd *exec.Cmd
	switch runtime.GOOS {
	case "windows":
		cmd = exec.Command("cmd", "/c", "start", "", url)
	case "darwin":
		cmd = exec.Command("open", url)
	default:
		cmd = exec.Command("xdg-open", url)
	}
	_ = cmd.Start()
}

// ---------- 小工具 ----------

func splitCSV(s string) []string {
	out := []string{}
	for _, p := range strings.Split(s, ",") {
		if t := strings.TrimSpace(p); t != "" {
			out = append(out, t)
		}
	}
	return out
}

func formatTS(v any) string {
	switch t := v.(type) {
	case float64:
		return time.Unix(int64(t), 0).Format("01-02 15:04")
	case int64:
		return time.Unix(t, 0).Format("01-02 15:04")
	}
	return "—"
}
