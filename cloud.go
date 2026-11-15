package main

// cloud.go —— 云端服务（teacherhub serve）：零账号、统一访问 key。
// 职责两件：①ima 贡献统计 ②工作台产物库（相似检索 / 下发链接 / 统计页）。
// 存储：data/store.json（元数据）+ data/artifacts/{id}.html（产物）。

import (
	"archive/zip"
	"compress/gzip"
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io"
	"io/fs"
	"log"
	"math"
	"mime"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Contrib struct {
	ID      string `json:"id"`
	Teacher string `json:"teacher"`
	Text    string `json:"text"`
	KB      string `json:"kb,omitempty"`
	Item    string `json:"item,omitempty"`
	At      int64  `json:"at"`
}

type Workbench struct {
	ID         string   `json:"id"`
	Title      string   `json:"title"`
	Desc       string   `json:"desc,omitempty"`
	Tags       []string `json:"tags,omitempty"`
	Keywords   []string `json:"keywords,omitempty"`
	Teacher    string   `json:"teacher,omitempty"`
	Size       int64    `json:"size"`
	Downloads  int      `json:"downloads"`
	Views      int      `json:"views"`
	HasPreview bool     `json:"has_preview"`
	Hidden     bool     `json:"hidden"`
	RateSum    int      `json:"rate_sum"`
	RateNum    int      `json:"rate_num"`
	CreatedAt  int64    `json:"created_at"`
}

// ScoreExam 一次考试成绩（原始分数组，仅进统计不公开原分）。
type ScoreExam struct {
	ID        string         `json:"id"`
	Name      string         `json:"name"`
	At        int64          `json:"at"`
	Scores    []float64      `json:"scores"`
	Questions []QuestionStat `json:"questions,omitempty"`
}

// FileRec 校本资源文件元数据（磁盘名用 id，展示名净化后存元数据）。
type FileRec struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Size      int64  `json:"size"`
	Teacher   string `json:"teacher"`
	At        int64  `json:"at"`
	Downloads int    `json:"downloads"`
}

// sanitizeFileName 文件名净化：拦路径分隔符与控制字符（点号是合法字符不拦）。
func sanitizeFileName(s string) string {
	s = strings.TrimSpace(s)
	s = strings.ReplaceAll(s, "\\", "_")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.Map(func(r rune) rune {
		if r < 32 || r == 127 {
			return -1
		}
		return r
	}, s)
	s = strings.TrimSpace(s)
	if s == "" || s == "." || s == ".." {
		s = "file"
	}
	if rs := []rune(s); len(rs) > 120 {
		s = string(rs[:120])
	}
	return s
}

// LogEntry 管理操作日志。
type LogEntry struct {
	At     int64  `json:"at"`
	Op     string `json:"op"`
	Detail string `json:"detail"`
}

// KBClaim 课标模块认领（谁认领了哪个模块的映射）。
type KBClaim struct {
	By string `json:"by"`
	At int64  `json:"at"`
}

// QuestionStat 单题统计（得分率 = Avg/Max，用于失分排行）。
type QuestionStat struct {
	Name string  `json:"name"`
	Max  float64 `json:"max"`
	Avg  float64 `json:"avg"`
}

// scoreSummary 成绩统计视图（不含原始分）。
func scoreSummary(e ScoreExam) map[string]any {
	n := len(e.Scores)
	out := map[string]any{"id": e.ID, "name": e.Name, "at": e.At, "count": n}
	if n == 0 {
		return out
	}
	sum, mx, mn, pass := 0.0, e.Scores[0], e.Scores[0], 0
	for _, v := range e.Scores {
		sum += v
		if v > mx {
			mx = v
		}
		if v < mn {
			mn = v
		}
		if v >= 60 {
			pass++
		}
	}
	out["avg"] = math.Round(sum/float64(n)*10) / 10
	out["max"] = mx
	out["min"] = mn
	out["pass_rate"] = math.Round(float64(pass)/float64(n)*1000) / 10
	return out
}

// scoreSegments 分数段分布（10 分一桶）。
func scoreSegments(scores []float64) []map[string]any {
	segs := []map[string]any{}
	for lo := 0; lo <= 140; lo += 10 {
		n := 0
		for _, v := range scores {
			if v >= float64(lo) && v < float64(lo+10) {
				n++
			}
		}
		segs = append(segs, map[string]any{"lo": lo, "hi": lo + 9, "n": n})
	}
	n := 0
	for _, v := range scores {
		if v >= 150 {
			n++
		}
	}
	segs = append(segs, map[string]any{"lo": 150, "hi": 150, "n": n})
	return segs
}

// BoardMsg 留言墙条目（开放留言，限速防刷）。
type BoardMsg struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Text  string `json:"text"`
	Likes int    `json:"likes"`
	At    int64  `json:"at"`
}

type cloudStore struct {
	mu   sync.Mutex
	dir  string
	data struct {
		Key         string             `json:"key"`
		Contribs    []Contrib          `json:"contribs"`
		Workbenches []Workbench        `json:"workbenches"`
		Board       []BoardMsg         `json:"board,omitempty"`
		Notice      string             `json:"notice,omitempty"`
		DailyViews  map[string]int     `json:"daily_views,omitempty"`
		Scores      []ScoreExam        `json:"scores,omitempty"`
		KBClaims    map[string]KBClaim `json:"kb_claims,omitempty"`
		Logs        []LogEntry         `json:"logs,omitempty"`
		Files       []FileRec          `json:"files,omitempty"`
	}
	boardLast  map[string]int64   // IP → 上次留言时间（内存限速）
	boardLikes map[string]bool    // ip|msgID → 已赞
	fileQuota  map[string][]int64 // IP → 上传时间戳列表（每小时 10 个限速）
}

func openStore(dir, key string) (*cloudStore, error) {
	if err := os.MkdirAll(filepath.Join(dir, "files"), 0o755); err != nil {
		return nil, err
	}
	if err := os.MkdirAll(filepath.Join(dir, "artifacts"), 0o755); err != nil {
		return nil, err
	}
	s := &cloudStore{dir: dir, boardLast: map[string]int64{}, boardLikes: map[string]bool{}, fileQuota: map[string][]int64{}}
	b, err := os.ReadFile(filepath.Join(dir, "store.json"))
	if err == nil {
		_ = json.Unmarshal(b, &s.data)
	}
	if s.data.Key == "" {
		s.data.Key = key
	}
	if s.data.Key == "" {
		s.data.Key = randToken()
	}
	if s.data.Contribs == nil {
		s.data.Contribs = []Contrib{}
	}
	if s.data.Workbenches == nil {
		s.data.Workbenches = []Workbench{}
	}
	for i := range s.data.Board { // 迁移：ID 字段加入前的旧留言补 ID
		if s.data.Board[i].ID == "" {
			s.data.Board[i].ID = randToken()[:8]
		}
	}
	return s, s.save()
}

func (s *cloudStore) save() error {
	b, _ := json.MarshalIndent(&s.data, "", "  ")
	return os.WriteFile(filepath.Join(s.dir, "store.json"), b, 0o600)
}

// downloadDirty 下载计数脏标记（攒批后随下次 save 持久化，不再每次 GET 全量写）。
var downloadDirty bool

// flushDownloads 检查并持久化下载计数（由 autoBackup 定时调用）。
func flushDownloads(st *cloudStore) {
	st.mu.Lock()
	if downloadDirty {
		_ = st.save()
		downloadDirty = false
	}
	st.mu.Unlock()
}

// keyMatches 锁内比较 key（rotate-key 并发写时防撕裂读）。
func (s *cloudStore) keyMatches(got string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return got != "" && got == s.data.Key
}

// log 记录管理操作日志（须在 st.mu 锁内调用，随外层 save 落盘；滚动 500 条）。
func (s *cloudStore) log(op, detail string) {
	s.data.Logs = append(s.data.Logs, LogEntry{At: time.Now().Unix(), Op: op, Detail: detail})
	if len(s.data.Logs) > 500 {
		s.data.Logs = s.data.Logs[len(s.data.Logs)-500:]
	}
}

func randToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// gzipRW gzip 响应包装。
type gzipRW struct {
	http.ResponseWriter
	gz *gzip.Writer
}

func (w *gzipRW) Write(b []byte) (int, error) { return w.gz.Write(b) }

// etagRW ETag 包装：缓冲响应体，算 hash 后设置 ETag/304。
type etagRW struct {
	http.ResponseWriter
	buf   strings.Builder
	code  int
	wrote bool
}

func (w *etagRW) WriteHeader(code int)        { w.code = code }
func (w *etagRW) Write(b []byte) (int, error) { w.wrote = true; return w.buf.Write(b) }
func (w *etagRW) Flush()                      {}

// withETag 对 GET 响应加 ETag + 304 缓存（gzip 路径由 withGzip 处理）。
func withETag(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet || strings.Contains(r.Header.Get("Accept-Encoding"), "gzip") {
			next.ServeHTTP(w, r)
			return
		}
		etag := etagRW{ResponseWriter: w}
		next.ServeHTTP(&etag, r)
		if !etag.wrote || etag.buf.Len() == 0 {
			return
		}
		body := []byte(etag.buf.String())
		sum := fmt.Sprintf("W/\"%x-%x\"", len(body), fnvHash(body))
		w.Header().Set("ETag", sum)
		if match := r.Header.Get("If-None-Match"); match != "" && strings.Contains(match, sum) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		if etag.code != 0 {
			w.WriteHeader(etag.code)
		}
		_, _ = w.Write(body)
	})
}

// fnvHash 简易 FNV-1a hash 用于 ETag。
func fnvHash(b []byte) uint32 {
	h := uint32(2166136261)
	for _, c := range b {
		h ^= uint32(c)
		h *= 16777619
	}
	return h
}

// wantGzip 是否值得压缩：页面/JSON/XML 压缩，大文件下载与 zip 不压。
func wantGzip(p string) bool {
	if strings.HasPrefix(p, "/api/files/") ||
		strings.HasPrefix(p, "/api/admin/export") ||
		strings.HasPrefix(p, "/api/admin/backups") ||
		strings.HasSuffix(p, "/file") ||
		strings.HasSuffix(p, "/preview") {
		return false
	}
	return true
}

// acceptsGzip 解析 Accept-Encoding：token 精确匹配且 q>0 才算支持（gzip;q=0 表示拒绝）。
func acceptsGzip(ae string) bool {
	for _, part := range strings.Split(ae, ",") {
		part = strings.TrimSpace(part)
		name := part
		q := 1.0
		if i := strings.Index(part, ";"); i >= 0 {
			name = strings.TrimSpace(part[:i])
			for _, p := range strings.Split(part[i+1:], ";") {
				p = strings.TrimSpace(p)
				if strings.HasPrefix(p, "q=") {
					if v, err := strconv.ParseFloat(strings.TrimPrefix(p, "q="), 64); err == nil {
						q = v
					}
				}
			}
		}
		if (name == "gzip" || name == "*") && q > 0 {
			return true
		}
	}
	return false
}

// withGzip 对响应做 gzip（浏览器带 Accept-Encoding 时）。
func withGzip(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodHead ||
			!acceptsGzip(r.Header.Get("Accept-Encoding")) ||
			!wantGzip(r.URL.Path) {
			next.ServeHTTP(w, r)
			return
		}
		r.Header.Del("Range") // gzip 与 Range 语义冲突，去掉 Range 保证全量响应正确
		w.Header().Set("Content-Encoding", "gzip")
		w.Header().Add("Vary", "Accept-Encoding")
		gz := gzip.NewWriter(w)
		defer gz.Close()
		next.ServeHTTP(&gzipRW{ResponseWriter: w, gz: gz}, r)
	})
}

// doBackup 立即生成一份快照 zip（store.json + artifacts + files，轮转留 7 份）。
// backupMu 串行化手动备份与 24h ticker；返回 error 供调用方判断成败。
var backupMu sync.Mutex

func doBackup(st *cloudStore) error {
	backupMu.Lock()
	defer backupMu.Unlock()
	bdir := filepath.Join(st.dir, "backups")
	if err := os.MkdirAll(bdir, 0o755); err != nil {
		return err
	}
	// 原子写：先写临时文件再 rename，防同秒覆盖/半成品被下载
	tmp, err := os.CreateTemp(bdir, "backup-*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	zw := zip.NewWriter(tmp)
	add := func(n, p string) error {
		b, err := os.ReadFile(p)
		if err != nil {
			return err // 可读性失败是真实错误，不该静默跳过
		}
		fw, err := zw.Create(n)
		if err != nil {
			return err
		}
		_, err = fw.Write(b)
		return err
	}
	if err := add("store.json", filepath.Join(st.dir, "store.json")); err != nil {
		zw.Close()
		tmp.Close()
		return err
	}
	for _, sub := range []string{"artifacts", "files"} {
		_ = filepath.WalkDir(filepath.Join(st.dir, sub), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(st.dir, path)
			// 单个文件读失败不中断整体（可能被并发删除），但记录
			_ = add(filepath.ToSlash(rel), path)
			return nil
		})
	}
	if err := zw.Close(); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	finalName := filepath.Join(bdir, fmt.Sprintf("backup-%019d-%s.zip", time.Now().UnixNano(), randToken()[:4]))
	if err := os.Rename(tmp.Name(), finalName); err != nil {
		return err
	}
	// 轮转
	if entries, err := os.ReadDir(bdir); err == nil && len(entries) > 7 {
		sort.Slice(entries, func(a, b int) bool { return entries[a].Name() < entries[b].Name() })
		for _, e := range entries[:len(entries)-7] {
			_ = os.Remove(filepath.Join(bdir, e.Name()))
		}
	}
	return nil
}

// autoBackup 启动时与每 24h 自动快照。
func autoBackup(st *cloudStore) {
	_ = doBackup(st) // 启动快照失败不致命，下次 tick 重试
	go func() {
		for range time.Tick(1 * time.Hour) {
			flushDownloads(st) // 每小时持久化一次下载计数
		}
	}()
	go func() {
		for range time.Tick(24 * time.Hour) {
			_ = doBackup(st)
		}
	}()
}

// backupHistory 把当前产物内容备份到 artifacts/{id}.hist/{unix}.html（滚动保留 5 份）。
func backupHistory(st *cloudStore, id string) {
	old, err := os.ReadFile(filepath.Join(st.dir, "artifacts", id+".html"))
	if err != nil {
		return
	}
	hdir := filepath.Join(st.dir, "artifacts", id+".hist")
	_ = os.MkdirAll(hdir, 0o755)
	_ = os.WriteFile(filepath.Join(hdir, fmt.Sprintf("%d.html", time.Now().Unix())), old, 0o644)
	if entries, err := os.ReadDir(hdir); err == nil && len(entries) > 5 {
		sort.Slice(entries, func(a, b int) bool { return entries[a].Name() < entries[b].Name() })
		for _, e := range entries[:len(entries)-5] {
			_ = os.Remove(filepath.Join(hdir, e.Name()))
		}
	}
}

// savePreview 解析 dataURL（data:image/*;base64,...）落盘 artifacts/{id}.preview；成功返回 true。
func savePreview(st *cloudStore, id, dataURL string) bool {
	i := strings.Index(dataURL, ";base64,")
	if i < 0 || len(dataURL) > 600_000 {
		return false
	}
	b, err := base64.StdEncoding.DecodeString(dataURL[i+8:])
	if err != nil || len(b) < 100 || len(b) > 400_000 {
		return false
	}
	return os.WriteFile(filepath.Join(st.dir, "artifacts", id+".preview"), b, 0o644) == nil
}

// ---------- 云端 HTTP ----------

func cmdServe(args []string) {
	addr := ":8096"
	data := "./data"
	key := ""
	for i := 0; i < len(args); i++ {
		switch args[i] {
		case "--addr":
			i++
			if i < len(args) {
				addr = args[i]
			}
		case "--data":
			i++
			if i < len(args) {
				data = args[i]
			}
		case "--key":
			i++
			if i < len(args) {
				key = args[i]
			}
		}
	}
	st, err := openStore(data, key)
	if err != nil {
		log.Fatal(err)
	}
	autoBackup(st) // 启动快照 + 每 24h 自动备份（backups/ 轮转 7 份）
	mux := http.NewServeMux()
	// 访问政策：浏览（GET）全免 key——打开即看；写操作（POST/DELETE）必须带统一 key。
	api := func(w http.ResponseWriter, r *http.Request, h func(http.ResponseWriter, *http.Request)) {
		if r.Method == http.MethodGet || r.Method == http.MethodHead {
			h(w, r)
			return
		}
		got := r.Header.Get("X-Access-Key")
		if got == "" {
			got = r.URL.Query().Get("key")
		}
		if !st.keyMatches(got) {
			writeJSON(w, 401, map[string]any{"ok": false, "error": "写操作需要统一访问 key"})
			return
		}
		h(w, r)
	}

	mux.HandleFunc("/api/ping", func(w http.ResponseWriter, r *http.Request) {
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			writeJSON(w, 200, map[string]any{"ok": true, "version": version})
		})
	})

	// 贡献
	mux.HandleFunc("/api/contrib", func(w http.ResponseWriter, r *http.Request) {
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				var c Contrib
				if err := json.NewDecoder(r.Body).Decode(&c); err != nil || strings.TrimSpace(c.Text) == "" {
					writeJSON(w, 400, map[string]any{"ok": false, "error": "缺 text"})
					return
				}
				c.ID = "cg_" + randToken()[:12]
				c.Teacher = strings.TrimSpace(c.Teacher)
				if c.Teacher == "" {
					c.Teacher = "匿名"
				}
				c.At = time.Now().Unix()
				st.mu.Lock()
				st.data.Contribs = append(st.data.Contribs, c)
				st.log("贡献上报", c.Teacher+"："+c.Text)
				// 只留最近 2000 条
				if len(st.data.Contribs) > 2000 {
					st.data.Contribs = st.data.Contribs[len(st.data.Contribs)-2000:]
				}
				_ = st.save()
				st.mu.Unlock()
				writeJSON(w, 200, map[string]any{"ok": true, "id": c.ID})
				return
			}
			st.mu.Lock()
			list := make([]Contrib, len(st.data.Contribs))
			copy(list, st.data.Contribs)
			st.mu.Unlock()
			sort.Slice(list, func(i, j int) bool { return list[i].At > list[j].At })
			if len(list) > 50 {
				list = list[:50]
			}
			writeJSON(w, 200, map[string]any{"ok": true, "contribs": list})
		})
	})

	// 统计（聚合：按教师）
	mux.HandleFunc("/api/stats", func(w http.ResponseWriter, r *http.Request) {
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			st.mu.Lock()
			defer st.mu.Unlock()
			type agg struct {
				Teacher   string `json:"teacher"`
				Contribs  int    `json:"contribs"`
				Workbench int    `json:"workbenches"`
				LastAt    int64  `json:"last_at"`
			}
			m := map[string]*agg{}
			for _, c := range st.data.Contribs {
				a := m[c.Teacher]
				if a == nil {
					a = &agg{Teacher: c.Teacher}
					m[c.Teacher] = a
				}
				a.Contribs++
				if c.At > a.LastAt {
					a.LastAt = c.At
				}
			}
			for _, w2 := range st.data.Workbenches {
				if w2.Teacher == "" {
					continue
				}
				a := m[w2.Teacher]
				if a == nil {
					a = &agg{Teacher: w2.Teacher}
					m[w2.Teacher] = a
				}
				a.Workbench++
				if w2.CreatedAt > a.LastAt {
					a.LastAt = w2.CreatedAt
				}
			}
			list := []*agg{}
			for _, a := range m {
				list = append(list, a)
			}
			sort.Slice(list, func(i, j int) bool { return list[i].LastAt > list[j].LastAt })
			recent := make([]Contrib, 0, 15)
			for i := len(st.data.Contribs) - 1; i >= 0 && len(recent) < 15; i-- {
				recent = append(recent, st.data.Contribs[i])
			}
			views, downloads := 0, 0
			for _, w2 := range st.data.Workbenches {
				views += w2.Views
				downloads += w2.Downloads
			}
			daily := map[string]int{}
			for k, v := range st.data.DailyViews {
				daily[k] = v
			}
			writeJSON(w, 200, map[string]any{
				"ok": true, "total_workbenches": len(st.data.Workbenches),
				"total_contribs": len(st.data.Contribs),
				"by_teacher":     list, "recent": recent,
				"totals": map[string]any{
					"views": views, "downloads": downloads,
					"today_views": daily[time.Now().Format("2006-01-02")],
					"board_msgs":  len(st.data.Board), "daily": daily,
				},
			})
		})
	})

	// 产物库
	mux.HandleFunc("/api/workbenches", func(w http.ResponseWriter, r *http.Request) {
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodPost {
				var in struct {
					Title    string   `json:"title"`
					Desc     string   `json:"desc"`
					Tags     []string `json:"tags"`
					Keywords []string `json:"keywords"`
					Teacher  string   `json:"teacher"`
					Content  string   `json:"content"`
					Preview  string   `json:"preview"`
				}
				r.Body = http.MaxBytesReader(w, r.Body, 10<<20) // 10MB 上限防内存 DoS
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Title == "" || in.Content == "" {
					writeJSON(w, 400, map[string]any{"ok": false, "error": "缺 title/content 或 body 过大"})
					return
				}
				if len([]rune(in.Title)) > 200 || len([]rune(in.Desc)) > 500 || len(in.Content) > 5_000_000 {
					writeJSON(w, 400, map[string]any{"ok": false, "error": "字段过长（title≤200字/desc≤500字/content≤5MB）"})
					return
				}
				wb := Workbench{
					ID: "wb_" + randToken()[:12], Title: strings.TrimSpace(in.Title),
					Desc: strings.TrimSpace(in.Desc), Tags: in.Tags, Keywords: in.Keywords,
					Teacher: strings.TrimSpace(in.Teacher), Size: int64(len(in.Content)),
					CreatedAt: time.Now().Unix(),
				}
				if err := os.WriteFile(filepath.Join(st.dir, "artifacts", wb.ID+".html"), []byte(in.Content), 0o644); err != nil {
					writeJSON(w, 500, map[string]any{"ok": false, "error": "落盘失败"})
					return
				}
				if in.Preview != "" {
					wb.HasPreview = savePreview(st, wb.ID, in.Preview)
				}
				st.mu.Lock()
				st.data.Workbenches = append(st.data.Workbenches, wb)
				st.log("产物创建", wb.Title)
				_ = st.save()
				st.mu.Unlock()
				writeJSON(w, 200, map[string]any{"ok": true, "id": wb.ID, "link": workbenchLink(r, wb.ID)})
				return
			}
			showAll := r.URL.Query().Get("all") == "1"
			st.mu.Lock()
			list := make([]Workbench, 0, len(st.data.Workbenches))
			for _, w2 := range st.data.Workbenches {
				if !w2.Hidden || showAll {
					list = append(list, w2)
				}
			}
			st.mu.Unlock()
			sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt > list[j].CreatedAt })
			out := []map[string]any{}
			for _, w2 := range list {
				out = append(out, wbView(w2, r))
			}
			writeJSON(w, 200, map[string]any{"ok": true, "workbenches": out})
		})
	})

	// 相似检索
	mux.HandleFunc("/api/workbenches/search", func(w http.ResponseWriter, r *http.Request) {
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			q := strings.TrimSpace(r.URL.Query().Get("q"))
			if q == "" {
				writeJSON(w, 200, map[string]any{"ok": true, "matches": []any{}})
				return
			}
			st.mu.Lock()
			list := make([]Workbench, 0, len(st.data.Workbenches))
			for _, w2 := range st.data.Workbenches {
				if !w2.Hidden {
					list = append(list, w2)
				}
			}
			st.mu.Unlock()
			type match struct {
				Workbench map[string]any `json:"workbench"`
				Score     float64        `json:"score"`
			}
			matches := []match{}
			for _, w2 := range list {
				sc := similarity(q, w2)
				if sc >= 0.3 {
					matches = append(matches, match{Workbench: wbView(w2, r), Score: sc})
				}
			}
			sort.Slice(matches, func(i, j int) bool { return matches[i].Score > matches[j].Score })
			if len(matches) > 10 {
				matches = matches[:10]
			}
			writeJSON(w, 200, map[string]any{"ok": true, "matches": matches})
		})
	})

	mux.HandleFunc("/api/workbenches/", func(w http.ResponseWriter, r *http.Request) {
		id0 := strings.TrimPrefix(r.URL.Path, "/api/workbenches/")
		if r.Method == http.MethodPost && strings.HasSuffix(id0, "/rate") { // 评分免 key（在鉴权壳外）
			rateH(st, w, r, strings.TrimSuffix(id0, "/rate"))
			return
		}
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimPrefix(r.URL.Path, "/api/workbenches/")
			if r.Method == http.MethodDelete { // 删除产物（管理 key；历史备份 .hist 保留可恢复）
				st.mu.Lock()
				found := false
				for i := range st.data.Workbenches {
					if st.data.Workbenches[i].ID == id {
						st.data.Workbenches = append(st.data.Workbenches[:i], st.data.Workbenches[i+1:]...)
						st.log("产物删除", id)
						found = true
						break
					}
				}
				if found {
					_ = st.save()
				}
				st.mu.Unlock()
				if !found {
					writeJSON(w, 404, map[string]any{"ok": false, "error": "产物不存在"})
					return
				}
				_ = os.Remove(filepath.Join(st.dir, "artifacts", id+".html"))
				_ = os.Remove(filepath.Join(st.dir, "artifacts", id+".preview"))
				writeJSON(w, 200, map[string]any{"ok": true})
				return
			}
			if r.Method == http.MethodPost && strings.HasSuffix(id, "/hidden") { // 下架/上架（写操作须 key）
				id = strings.TrimSuffix(id, "/hidden")
				var in struct {
					Hidden bool `json:"hidden"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					writeJSON(w, 400, map[string]any{"ok": false, "error": "请求体解析失败"})
					return
				}
				st.mu.Lock()
				found := false
				for i := range st.data.Workbenches {
					if st.data.Workbenches[i].ID == id {
						st.data.Workbenches[i].Hidden = in.Hidden
						st.log("产物上下架", id+" hidden="+fmt.Sprintf("%v", in.Hidden))
						found = true
						break
					}
				}
				if found {
					_ = st.save()
				}
				st.mu.Unlock()
				writeJSON(w, 200, map[string]any{"ok": found})
				return
			}
			if r.Method == http.MethodPut { // 更新产物内容（覆盖式升级，写操作须 key）
				var in struct {
					Title   string   `json:"title"`
					Desc    string   `json:"desc"`
					Tags    []string `json:"tags"`
					Content string   `json:"content"`
					Preview string   `json:"preview"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					writeJSON(w, 400, map[string]any{"ok": false, "error": "请求体解析失败"})
					return
				}
				st.mu.Lock()
				found := false
				for i := range st.data.Workbenches {
					if st.data.Workbenches[i].ID == id {
						if in.Title != "" {
							if len([]rune(in.Title)) > 200 || len([]rune(in.Desc)) > 500 || len(in.Content) > 5_000_000 {
								writeJSON(w, 400, map[string]any{"ok": false, "error": "字段过长（title≤200字/desc≤500字/content≤5MB）"})
								return
							}
							st.data.Workbenches[i].Title = strings.TrimSpace(in.Title)
						}
						if in.Desc != "" {
							st.data.Workbenches[i].Desc = strings.TrimSpace(in.Desc)
						}
						if in.Tags != nil {
							st.data.Workbenches[i].Tags = in.Tags
						}
						if in.Content != "" {
							backupHistory(st, id) // 更新前先存旧版
							if err := os.WriteFile(filepath.Join(st.dir, "artifacts", id+".html"), []byte(in.Content), 0o644); err == nil {
								st.data.Workbenches[i].Size = int64(len(in.Content))
							}
						}
						if in.Preview != "" {
							st.data.Workbenches[i].HasPreview = savePreview(st, id, in.Preview)
						}
						found = true
						break
					}
				}
				if found {
					_ = st.save()
				}
				st.mu.Unlock()
				if !found {
					writeJSON(w, 404, map[string]any{"ok": false, "error": "产物不存在"})
					return
				}
				writeJSON(w, 200, map[string]any{"ok": true, "id": id})
				return
			}
			if strings.HasSuffix(id, "/history") { // 版本历史列表（免 key）
				id = strings.TrimSuffix(id, "/history")
				st.mu.Lock()
				exists := false
				for _, w2 := range st.data.Workbenches {
					if w2.ID == id {
						exists = true
						break
					}
				}
				st.mu.Unlock()
				if !exists {
					writeJSON(w, 404, map[string]any{"ok": false, "error": "产物不存在"})
					return
				}
				type hv struct {
					TS   int64 `json:"ts"`
					Size int64 `json:"size"`
				}
				list := []hv{}
				if entries, err := os.ReadDir(filepath.Join(st.dir, "artifacts", id+".hist")); err == nil {
					for _, e := range entries {
						if fi, err := e.Info(); err == nil {
							ts, _ := strconv.ParseInt(strings.TrimSuffix(e.Name(), ".html"), 10, 64)
							list = append(list, hv{TS: ts, Size: fi.Size()})
						}
					}
				}
				sort.Slice(list, func(a, b int) bool { return list[a].TS > list[b].TS })
				writeJSON(w, 200, map[string]any{"ok": true, "history": list})
				return
			}
			if strings.HasSuffix(id, "/restore") { // 恢复历史版本（写操作须 key）
				id = strings.TrimSuffix(id, "/restore")
				var in struct {
					TS int64 `json:"ts"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
					writeJSON(w, 400, map[string]any{"ok": false, "error": "请求体解析失败"})
					return
				}
				hfile := filepath.Join(st.dir, "artifacts", id+".hist", fmt.Sprintf("%d.html", in.TS))
				b, err := os.ReadFile(hfile)
				if err != nil {
					writeJSON(w, 404, map[string]any{"ok": false, "error": "历史版本不存在"})
					return
				}
				backupHistory(st, id) // 恢复前把当前版本也存档
				_ = os.WriteFile(filepath.Join(st.dir, "artifacts", id+".html"), b, 0o644)
				st.mu.Lock()
				for i := range st.data.Workbenches {
					if st.data.Workbenches[i].ID == id {
						st.data.Workbenches[i].Size = int64(len(b))
						break
					}
				}
				_ = st.save()
				st.mu.Unlock()
				writeJSON(w, 200, map[string]any{"ok": true})
				return
			}
			if strings.HasSuffix(id, "/preview") { // 预览图（免 key）
				id = strings.TrimSuffix(id, "/preview")
				if id == "" || strings.ContainsAny(id, "/\\.") {
					http.NotFound(w, r)
					return
				}
				w.Header().Set("Cache-Control", "public, max-age=3600")
				http.ServeFile(w, r, filepath.Join(st.dir, "artifacts", id+".preview"))
				return
			}
			if strings.HasSuffix(id, "/file") {
				id = strings.TrimSuffix(id, "/file")
				st.mu.Lock()
				for i := range st.data.Workbenches {
					if st.data.Workbenches[i].ID == id {
						st.data.Workbenches[i].Downloads++
						_ = st.save()
						break
					}
				}
				st.mu.Unlock()
				w.Header().Set("Content-Disposition", "attachment; filename=\""+id+".html\"")
				http.ServeFile(w, r, filepath.Join(st.dir, "artifacts", id+".html"))
				return
			}
			st.mu.Lock()
			defer st.mu.Unlock()
			for _, w2 := range st.data.Workbenches {
				if w2.ID == id {
					writeJSON(w, 200, map[string]any{"ok": true, "workbench": wbView(w2, r)})
					return
				}
			}
			writeJSON(w, 404, map[string]any{"ok": false, "error": "产物不存在"})
		})
	})

	// 产物查看：/w/{id} 顶栏包装（返回工作台），/w/{id}/raw 原文直看——浏览全免 key
	mux.HandleFunc("/w/", func(w http.ResponseWriter, r *http.Request) {
		rest := strings.TrimPrefix(r.URL.Path, "/w/")
		raw := strings.HasSuffix(rest, "/raw")
		id := strings.TrimSuffix(rest, "/raw")
		if id == "" || strings.ContainsAny(id, "/\\.") {
			http.NotFound(w, r)
			return
		}
		st.mu.Lock()
		var meta *Workbench
		for i := range st.data.Workbenches {
			if st.data.Workbenches[i].ID == id {
				meta = &st.data.Workbenches[i]
				break
			}
		}
		st.mu.Unlock()
		if meta == nil {
			http.NotFound(w, r)
			return
		}
		if !raw { // 查看页打开即计一次热度（raw 原文不算）
			st.mu.Lock()
			for i := range st.data.Workbenches {
				if st.data.Workbenches[i].ID == id {
					st.data.Workbenches[i].Views++
					if st.data.DailyViews == nil {
						st.data.DailyViews = map[string]int{}
					}
					today := time.Now().Format("2006-01-02")
					st.data.DailyViews[today]++
					if len(st.data.DailyViews) > 30 {
						for k := range st.data.DailyViews {
							if k != today {
								delete(st.data.DailyViews, k)
							}
						}
					}
					_ = st.save()
					break
				}
			}
			st.mu.Unlock()
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		if raw {
			http.ServeFile(w, r, filepath.Join(st.dir, "artifacts", id+".html"))
			return
		}
		_, _ = w.Write([]byte(viewerHTML(*meta)))
	})

	// 成绩分析：读免 key（只出统计）、写留 key
	mux.HandleFunc("/api/scores", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet: // 列表（不含原始分）
			st.mu.Lock()
			exams := make([]map[string]any, 0, len(st.data.Scores))
			for _, e := range st.data.Scores {
				exams = append(exams, scoreSummary(e))
			}
			st.mu.Unlock()
			writeJSON(w, 200, map[string]any{"ok": true, "exams": exams})
		case http.MethodPost: // 录入（key）
			api(w, r, func(w http.ResponseWriter, r *http.Request) {
				var in struct {
					Name      string    `json:"name"`
					Scores    []float64 `json:"scores"`
					Questions []struct {
						Name string  `json:"name"`
						Max  float64 `json:"max"`
						Avg  float64 `json:"avg"`
					} `json:"questions"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Name == "" || len(in.Scores) == 0 {
					writeJSON(w, 400, map[string]any{"ok": false, "error": "缺 name/scores"})
					return
				}
				if len([]rune(in.Name)) > 50 || len(in.Scores) > 500 {
					writeJSON(w, 400, map[string]any{"ok": false, "error": "name≤50字/scores≤500个"})
					return
				}
				for _, v := range in.Scores {
					if v < 0 || v > 150 {
						writeJSON(w, 400, map[string]any{"ok": false, "error": "分数须在 0-150"})
						return
					}
				}
				var qs []QuestionStat
				if len(in.Questions) > 50 {
					writeJSON(w, 400, map[string]any{"ok": false, "error": "questions ≤50 题"})
					return
				}
				for _, q := range in.Questions {
					q.Name = strings.TrimSpace(q.Name)
					if q.Name == "" || len([]rune(q.Name)) > 20 || q.Max <= 0 || q.Max > 200 || q.Avg < 0 || q.Avg > q.Max {
						writeJSON(w, 400, map[string]any{"ok": false, "error": "questions 格式不对（name≤20字, 0≤avg≤max≤200）"})
						return
					}
					qs = append(qs, QuestionStat{Name: q.Name, Max: q.Max, Avg: q.Avg})
				}
				e := ScoreExam{ID: "sx_" + randToken()[:12], Name: in.Name, At: time.Now().Unix(), Scores: in.Scores, Questions: qs}
				st.mu.Lock()
				st.data.Scores = append(st.data.Scores, e)
				st.log("成绩录入", fmt.Sprintf("%s（%d 人）", e.Name, len(e.Scores)))
				_ = st.save()
				st.mu.Unlock()
				writeJSON(w, 200, map[string]any{"ok": true, "id": e.ID})
			})
		default:
			writeJSON(w, 405, map[string]any{"ok": false, "error": "方法不允许"})
		}
	})
	mux.HandleFunc("/api/scores/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/scores/")
		if id == "" || strings.Contains(id, "/") {
			writeJSON(w, 404, map[string]any{"ok": false, "error": "not found"})
			return
		}
		switch r.Method {
		case http.MethodGet: // 详情（含分数段，不含原始分）
			st.mu.Lock()
			var found *ScoreExam
			for i := range st.data.Scores {
				if st.data.Scores[i].ID == id {
					found = &st.data.Scores[i]
					break
				}
			}
			var out map[string]any
			if found != nil {
				out = scoreSummary(*found)
				out["segments"] = scoreSegments(found.Scores)
				if len(found.Questions) > 0 {
					qs := make([]map[string]any, 0, len(found.Questions))
					for _, q := range found.Questions {
						rate := 0.0
						if q.Max > 0 {
							rate = math.Round(q.Avg/q.Max*1000) / 10
						}
						qs = append(qs, map[string]any{"name": q.Name, "max": q.Max, "avg": q.Avg, "rate": rate})
					}
					sort.Slice(qs, func(i, j int) bool { return qs[i]["rate"].(float64) < qs[j]["rate"].(float64) })
					out["questions"] = qs
				}
			}
			st.mu.Unlock()
			if found == nil {
				writeJSON(w, 404, map[string]any{"ok": false, "error": "考试不存在"})
				return
			}
			out["ok"] = true
			writeJSON(w, 200, out)
		case http.MethodDelete: // 删除（key）
			api(w, r, func(w http.ResponseWriter, r *http.Request) {
				st.mu.Lock()
				found := false
				ns := st.data.Scores[:0]
				for _, e := range st.data.Scores {
					if e.ID == id {
						found = true
						continue
					}
					ns = append(ns, e)
				}
				if found {
					st.data.Scores = ns
					st.log("成绩删除", id)
					_ = st.save()
				}
				st.mu.Unlock()
				if !found {
					writeJSON(w, 404, map[string]any{"ok": false, "error": "考试不存在"})
					return
				}
				writeJSON(w, 200, map[string]any{"ok": true})
			})
		default:
			writeJSON(w, 405, map[string]any{"ok": false, "error": "方法不允许"})
		}
	})
	// 成绩 CSV 导出（管理 key：含原始分，仅管理用途）
	mux.HandleFunc("/api/scores/csv/", func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Access-Key")
		if got == "" {
			got = r.URL.Query().Get("key")
		}
		if !st.keyMatches(got) {
			writeJSON(w, 401, map[string]any{"ok": false, "error": "key 不对"})
			return
		}
		id := strings.TrimPrefix(r.URL.Path, "/api/scores/csv/")
		st.mu.Lock()
		var found *ScoreExam
		for i := range st.data.Scores {
			if st.data.Scores[i].ID == id {
				found = &st.data.Scores[i]
				break
			}
		}
		var buf strings.Builder
		fname := ""
		if found != nil {
			fname = found.Name
			buf.WriteString("\xEF\xBB\xBF") // BOM 中文兼容
			buf.WriteString("成绩,人数,均分,最高,最低,及格率\r\n")
			su := scoreSummary(*found)
			buf.WriteString(fmt.Sprintf("%s,%d,%v,%v,%v,%v%%\r\n\r\n", found.Name, su["count"], su["avg"], su["max"], su["min"], su["pass_rate"]))
			buf.WriteString("序号,分数\r\n")
			for i, v := range found.Scores {
				buf.WriteString(fmt.Sprintf("%d,%g\r\n", i+1, v))
			}
			if len(found.Questions) > 0 {
				buf.WriteString("\r\n题号,满分,平均分,得分率\r\n")
				for _, q := range found.Questions {
					buf.WriteString(fmt.Sprintf("%s,%g,%g,%.1f%%\r\n", q.Name, q.Max, q.Avg, q.Avg/q.Max*100))
				}
			}
		}
		st.mu.Unlock()
		if found == nil {
			writeJSON(w, 404, map[string]any{"ok": false, "error": "考试不存在"})
			return
		}
		w.Header().Set("Content-Type", "text/csv; charset=utf-8")
		w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fname + ".csv"}))
		_, _ = w.Write([]byte(buf.String()))
	})

	// 课标认领：读免 key、写留 key
	mux.HandleFunc("/api/kb", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		claims := make(map[string]KBClaim, len(st.data.KBClaims))
		for k, v := range st.data.KBClaims {
			claims[k] = v
		}
		st.mu.Unlock()
		writeJSON(w, 200, map[string]any{"ok": true, "claims": claims})
	})
	mux.HandleFunc("/api/kb/claim", func(w http.ResponseWriter, r *http.Request) {
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			if r.Method == http.MethodDelete { // 清除认领
				var in struct {
					Module string `json:"module"`
				}
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Module == "" {
					writeJSON(w, 400, map[string]any{"ok": false, "error": "缺 module"})
					return
				}
				st.mu.Lock()
				_, existed := st.data.KBClaims[in.Module]
				if existed {
					delete(st.data.KBClaims, in.Module)
					st.log("清除认领", in.Module)
					_ = st.save()
				}
				st.mu.Unlock()
				if !existed {
					writeJSON(w, 404, map[string]any{"ok": false, "error": "该模块无认领记录"})
					return
				}
				writeJSON(w, 200, map[string]any{"ok": true})
				return
			}
			if r.Method != http.MethodPost {
				writeJSON(w, 405, map[string]any{"ok": false, "error": "方法不允许"})
				return
			}
			var in struct {
				Module string `json:"module"`
				By     string `json:"by"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Module == "" || in.By == "" {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "缺 module/by"})
				return
			}
			if len([]rune(in.Module)) > 30 || len([]rune(in.By)) > 20 {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "module≤30字/by≤20字"})
				return
			}
			st.mu.Lock()
			if st.data.KBClaims == nil {
				st.data.KBClaims = map[string]KBClaim{}
			}
			st.data.KBClaims[in.Module] = KBClaim{By: in.By, At: time.Now().Unix()}
			st.log("课标认领", in.Module+" ← "+in.By)
			_ = st.save()
			st.mu.Unlock()
			writeJSON(w, 200, map[string]any{"ok": true})
		})
	})

	// 留言墙：免 key 开放留言（60s/条限速 + 200 字限制，滚动存 200 条）
	mux.HandleFunc("/api/board", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete { // 删除留言（管理 key）
			got := r.Header.Get("X-Access-Key")
			if got == "" {
				got = r.URL.Query().Get("key")
			}
			if !st.keyMatches(got) {
				writeJSON(w, 401, map[string]any{"ok": false, "error": "写操作需要统一访问 key"})
				return
			}
			var in struct {
				ID string `json:"id"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ID == "" {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "缺 id"})
				return
			}
			st.mu.Lock()
			kept := st.data.Board[:0]
			for _, m := range st.data.Board {
				if m.ID != in.ID {
					kept = append(kept, m)
				}
			}
			st.data.Board = kept
			st.log("留言删除", in.ID)
			_ = st.save()
			st.mu.Unlock()
			writeJSON(w, 200, map[string]any{"ok": true})
			return
		}
		if r.Method == http.MethodPost {
			var in struct {
				Name string `json:"name"`
				Text string `json:"text"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "请求体解析失败"})
				return
			}
			in.Name = strings.TrimSpace(in.Name)
			in.Text = strings.TrimSpace(in.Text)
			if in.Name == "" || in.Text == "" {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "缺昵称或内容"})
				return
			}
			if len([]rune(in.Text)) > 200 {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "留言太长（200 字内）"})
				return
			}
			ip := clientIP(r)
			now := time.Now().Unix()
			st.mu.Lock()
			if st.boardLast == nil {
				st.boardLast = map[string]int64{}
			}
			if now-st.boardLast[ip] < 60 {
				st.mu.Unlock()
				writeJSON(w, 429, map[string]any{"ok": false, "error": "发言太快，歇一分钟再来"})
				return
			}
			st.boardLast[ip] = now
			if nr := []rune(in.Name); len(nr) > 16 {
				in.Name = string(nr[:16])
			}
			st.data.Board = append(st.data.Board, BoardMsg{ID: randToken()[:8], Name: in.Name, Text: in.Text, At: now})
			if len(st.data.Board) > 200 {
				st.data.Board = st.data.Board[len(st.data.Board)-200:]
			}
			_ = st.save()
			st.mu.Unlock()
			writeJSON(w, 200, map[string]any{"ok": true})
			return
		}
		st.mu.Lock()
		list := make([]BoardMsg, len(st.data.Board))
		copy(list, st.data.Board)
		st.mu.Unlock()
		for i := 1; i < len(list); i++ { // 时间倒序
			for j := i; j > 0 && list[j].At > list[j-1].At; j-- {
				list[j], list[j-1] = list[j-1], list[j]
			}
		}
		if len(list) > 30 {
			list = list[:30]
		}
		writeJSON(w, 200, map[string]any{"ok": true, "board": list})
	})

	// 公告：GET 免 key；POST/DELETE 须统一 key（text 空 = 撤下）
	mux.HandleFunc("/api/notice", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodPost {
			got := r.Header.Get("X-Access-Key")
			if got == "" {
				got = r.URL.Query().Get("key")
			}
			if !st.keyMatches(got) {
				writeJSON(w, 401, map[string]any{"ok": false, "error": "写操作需要统一访问 key"})
				return
			}
			var in struct {
				Text string `json:"text"`
			}
			if err := json.NewDecoder(r.Body).Decode(&in); err != nil {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "请求体解析失败"})
				return
			}
			st.mu.Lock()
			st.data.Notice = strings.TrimSpace(in.Text)
			st.log("公告更新", "公告内容已更新（"+fmt.Sprintf("%d", len(in.Text))+" 字）")
			_ = st.save()
			st.mu.Unlock()
			writeJSON(w, 200, map[string]any{"ok": true})
			return
		}
		st.mu.Lock()
		text := st.data.Notice
		st.mu.Unlock()
		writeJSON(w, 200, map[string]any{"ok": true, "notice": text})
	})

	// 留言点赞：免 key，每 IP 每条限一赞
	mux.HandleFunc("/api/board/like", func(w http.ResponseWriter, r *http.Request) {
		var in struct {
			ID string `json:"id"`
		}
		if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.ID == "" {
			writeJSON(w, 400, map[string]any{"ok": false, "error": "缺 id"})
			return
		}
		ip := clientIP(r)
		key := ip + "|" + in.ID
		st.mu.Lock()
		if st.boardLikes == nil {
			st.boardLikes = map[string]bool{}
		}
		if st.boardLikes[key] {
			st.mu.Unlock()
			writeJSON(w, 200, map[string]any{"ok": true, "already": true})
			return
		}
		if len(st.boardLikes) > 20000 {
			st.boardLikes = map[string]bool{}
		}
		st.boardLikes[key] = true
		liked := false
		for i := range st.data.Board {
			if st.data.Board[i].ID == in.ID {
				st.data.Board[i].Likes++
				liked = true
				break
			}
		}
		if liked {
			_ = st.save()
		}
		st.mu.Unlock()
		writeJSON(w, 200, map[string]any{"ok": true})
	})

	// RSS 订阅：新产物 feed（免 key）
	mux.HandleFunc("/feed.xml", func(w http.ResponseWriter, r *http.Request) {
		st.mu.Lock()
		list := make([]Workbench, len(st.data.Workbenches))
		copy(list, st.data.Workbenches)
		st.mu.Unlock()
		sort.Slice(list, func(i, j int) bool { return list[i].CreatedAt > list[j].CreatedAt })
		if len(list) > 20 {
			list = list[:20]
		}
		base := "http://" + r.Host
		if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
			base = "https://" + r.Host
		}
		w.Header().Set("Content-Type", "application/rss+xml; charset=utf-8")
		var b strings.Builder
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>化学学科园地 · 动态</title><link>` + base + `/</link><description>产物与共享文件更新订阅</description>`)
		for _, wb := range list {
			b.WriteString(`<item><title>` + html.EscapeString(wb.Title) + `</title><link>` + base + `/w/` + wb.ID + `</link><pubDate>` + time.Unix(wb.CreatedAt, 0).Format(time.RFC1123Z) + `</pubDate><description>` + html.EscapeString(wb.Desc) + `</description></item>`)
		}
		st.mu.Lock()
		files := make([]FileRec, len(st.data.Files))
		copy(files, st.data.Files)
		st.mu.Unlock()
		sort.Slice(files, func(i, j int) bool { return files[i].At > files[j].At })
		if len(files) > 20 {
			files = files[:20]
		}
		for _, fl := range files {
			b.WriteString(`<item><title>共享文件：` + html.EscapeString(fl.Name) + `</title><link>` + base + `/api/files/` + fl.ID + `</link><pubDate>` + time.Unix(fl.At, 0).Format(time.RFC1123Z) + `</pubDate><description>` + html.EscapeString(fl.Teacher) + ` 上传（点击下载）</description></item>`)
		}
		b.WriteString(`</channel></rss>`)
		_, _ = w.Write([]byte(b.String()))
	})

	// 备份快照管理：清单 + 单份下载（管理 key）
	backHandler := func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Access-Key")
		if got == "" {
			got = r.URL.Query().Get("key")
		}
		if !st.keyMatches(got) {
			writeJSON(w, 401, map[string]any{"ok": false, "error": "key 不对"})
			return
		}
		path := strings.TrimPrefix(r.URL.Path, "/api/admin/backups")
		bdir := filepath.Join(st.dir, "backups")
		if path != "" { // 单份下载
			name := strings.TrimPrefix(path, "/")
			if name == "" || strings.ContainsAny(name, "/\\") {
				http.NotFound(w, r)
				return
			}
			w.Header().Set("Content-Disposition", `attachment; filename="`+name+`"`)
			http.ServeFile(w, r, filepath.Join(bdir, name))
			return
		}
		writeJSON(w, 200, map[string]any{"ok": true, "backups": listBackups(st)})
	}
	mux.HandleFunc("/api/admin/backups/", backHandler)
	mux.HandleFunc("/api/admin/backups", backHandler)

	// 模板一键上架（管理 key，幂等）
	mux.HandleFunc("/api/admin/seed-templates", func(w http.ResponseWriter, r *http.Request) {
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				writeJSON(w, 405, map[string]any{"ok": false, "error": "方法不允许"})
				return
			}
			results := seedTemplates(st)
			writeJSON(w, 200, map[string]any{"ok": true, "results": results})
		})
	})

	// 立即备份（管理 key）
	mux.HandleFunc("/api/admin/backup", func(w http.ResponseWriter, r *http.Request) {
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				writeJSON(w, 405, map[string]any{"ok": false, "error": "方法不允许"})
				return
			}
			if err := doBackup(st); err != nil {
				writeJSON(w, 500, map[string]any{"ok": false, "error": "备份失败：" + err.Error()})
				return
			}
			st.mu.Lock()
			st.log("手动备份", "生成快照 zip")
			_ = st.save()
			st.mu.Unlock()
			writeJSON(w, 200, map[string]any{"ok": true, "backups": listBackups(st)})
		})
	})

	// 操作日志（管理 key）
	mux.HandleFunc("/api/admin/logs", func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Access-Key")
		if got == "" {
			got = r.URL.Query().Get("key")
		}
		if !st.keyMatches(got) {
			writeJSON(w, 401, map[string]any{"ok": false, "error": "key 不对"})
			return
		}
		st.mu.Lock()
		logs := make([]LogEntry, len(st.data.Logs))
		copy(logs, st.data.Logs)
		st.mu.Unlock()
		for i, j := 0, len(logs)-1; i < j; i, j = i+1, j-1 { // 倒序（新在前）
			logs[i], logs[j] = logs[j], logs[i]
		}
		writeJSON(w, 200, map[string]any{"ok": true, "logs": logs})
	})
	// key 轮换（管理 key，旧 key 立即作废）
	mux.HandleFunc("/api/admin/rotate-key", func(w http.ResponseWriter, r *http.Request) {
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodPost {
				writeJSON(w, 405, map[string]any{"ok": false, "error": "方法不允许"})
				return
			}
			nk := randToken()
			st.mu.Lock()
			st.data.Key = nk
			st.log("key 轮换", "旧 key 已作废，新 key 生效")
			_ = st.save()
			st.mu.Unlock()
			writeJSON(w, 200, map[string]any{"ok": true, "key": nk})
		})
	})
	// 校本资源文件：列表免 key、上传免 key 限速、下载免 key、删除留 key
	mux.HandleFunc("/api/files", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet: // 列表（倒序）
			st.mu.Lock()
			files := make([]FileRec, len(st.data.Files))
			copy(files, st.data.Files)
			st.mu.Unlock()
			for i, j := 0, len(files)-1; i < j; i, j = i+1, j-1 {
				files[i], files[j] = files[j], files[i]
			}
			writeJSON(w, 200, map[string]any{"ok": true, "files": files})
		case http.MethodPost: // 上传（免 key，署名 + 限速 + 大小限制）
			// 限速用 RemoteAddr 兜底（XFF 可伪造，只有已知反代才该被信任）
			ip := r.RemoteAddr
			if host, _, err := net.SplitHostPort(ip); err == nil {
				ip = host
			}
			now := time.Now().Unix()
			st.mu.Lock()
			if st.fileQuota == nil {
				st.fileQuota = map[string][]int64{}
			}
			q := append(st.fileQuota[ip], now)
			kept := q[:0]
			for _, t := range q {
				if now-t < 3600 {
					kept = append(kept, t)
				}
			}
			if len(kept) > 10 {
				st.fileQuota[ip] = kept
				st.mu.Unlock()
				writeJSON(w, 429, map[string]any{"ok": false, "error": "上传太频繁（每小时 10 个）"})
				return
			}
			st.fileQuota[ip] = kept
			if len(st.fileQuota) > 20000 {
				st.fileQuota = map[string][]int64{ip: kept}
			}
			st.mu.Unlock()

			r.Body = http.MaxBytesReader(w, r.Body, 50<<20) // 单文件 ≤50MB
			// 1MB 内存 + 磁盘临时文件，防并发上传内存膨胀
			if err := r.ParseMultipartForm(1 << 20); err != nil {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "文件过大或格式不对（≤50MB）"})
				return
			}
			fh, hdr, err := r.FormFile("file")
			_ = hdr
			if err != nil {
				writeJSON(w, 400, map[string]any{"ok": false, "error": "缺 file 字段"})
				return
			}
			defer fh.Close()
			teacherField := strings.TrimSpace(r.FormValue("teacher"))
			teacher := teacherField
			if teacher == "" {
				teacher = "匿名"
			}
			if len([]rune(teacher)) > 20 {
				teacher = string([]rune(teacher)[:20])
			}
			name := sanitizeFileName(hdr.Filename)
			id := "fx_" + randToken()[:12]
			dst, err := os.Create(filepath.Join(st.dir, "files", id))
			if err != nil {
				writeJSON(w, 500, map[string]any{"ok": false, "error": "存储失败"})
				return
			}
			n, err := io.Copy(dst, fh)
			dst.Close()
			if err != nil {
				_ = os.Remove(filepath.Join(st.dir, "files", id))
				writeJSON(w, 500, map[string]any{"ok": false, "error": "写入失败"})
				return
			}
			rec := FileRec{ID: id, Name: name, Size: n, Teacher: teacher, At: now}
			st.mu.Lock()
			st.data.Files = append(st.data.Files, rec)
			st.log("文件上传", name+"（"+teacher+"）")
			_ = st.save()
			st.mu.Unlock()
			writeJSON(w, 200, map[string]any{"ok": true, "id": id, "name": name, "size": n})
		default:
			writeJSON(w, 405, map[string]any{"ok": false, "error": "方法不允许"})
		}
	})
	mux.HandleFunc("/api/files/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/files/")
		if id == "" || strings.Contains(id, "/") {
			writeJSON(w, 404, map[string]any{"ok": false, "error": "not found"})
			return
		}
		switch r.Method {
		case http.MethodGet: // 下载（免 key；attachment + nosniff 防内容嗅探渲染）
			st.mu.Lock()
			var rec *FileRec
			for i := range st.data.Files {
				if st.data.Files[i].ID == id {
					rec = &st.data.Files[i]
					break
				}
			}
			if rec != nil {
				rec.Downloads++
				downloadDirty = true
			}
			var fname string
			if rec != nil {
				fname = rec.Name
			}
			st.mu.Unlock()
			if rec == nil {
				writeJSON(w, 404, map[string]any{"ok": false, "error": "文件不存在"})
				return
			}
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("Content-Type", "application/octet-stream")
			w.Header().Set("Content-Disposition", mime.FormatMediaType("attachment", map[string]string{"filename": fname}))
			http.ServeFile(w, r, filepath.Join(st.dir, "files", id))
		case http.MethodDelete: // 删除（管理 key）
			api(w, r, func(w http.ResponseWriter, r *http.Request) {
				st.mu.Lock()
				found := false
				nf := st.data.Files[:0]
				var fname string
				for _, fl := range st.data.Files {
					if fl.ID == id {
						found = true
						fname = fl.Name
						continue
					}
					nf = append(nf, fl)
				}
				if found {
					st.data.Files = nf
					st.log("文件删除", fname)
					_ = st.save()
				}
				st.mu.Unlock()
				if !found {
					writeJSON(w, 404, map[string]any{"ok": false, "error": "文件不存在"})
					return
				}
				_ = os.Remove(filepath.Join(st.dir, "files", id))
				writeJSON(w, 200, map[string]any{"ok": true})
			})
		default:
			writeJSON(w, 405, map[string]any{"ok": false, "error": "方法不允许"})
		}
	})

	// 贡献记录删除（管理 key）
	mux.HandleFunc("/api/contrib/", func(w http.ResponseWriter, r *http.Request) {
		id := strings.TrimPrefix(r.URL.Path, "/api/contrib/")
		if id == "" || strings.Contains(id, "/") {
			writeJSON(w, 404, map[string]any{"ok": false, "error": "not found"})
			return
		}
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			if r.Method != http.MethodDelete {
				writeJSON(w, 405, map[string]any{"ok": false, "error": "方法不允许"})
				return
			}
			st.mu.Lock()
			found := false
			nc := st.data.Contribs[:0]
			for _, c := range st.data.Contribs {
				if c.ID == id {
					found = true
					continue
				}
				nc = append(nc, c)
			}
			if found {
				st.data.Contribs = nc
				st.log("贡献删除", id)
				_ = st.save()
			}
			st.mu.Unlock()
			if !found {
				writeJSON(w, 404, map[string]any{"ok": false, "error": "记录不存在"})
				return
			}
			writeJSON(w, 200, map[string]any{"ok": true})
		})
	})

	// 迷你 QR 编码器（查看页扫码用）
	mux.HandleFunc("/qr.js", handleQRJS)

	// PWA：manifest + sw + 图标（手机可"添加到主屏幕"）
	mux.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		_, _ = w.Write([]byte(`{"name":"化学学科园地","short_name":"学科园地","description":"化学教研组的课标、真题、实验与资源共享园地","start_url":"/","display":"standalone","theme_color":"#2F6B45","background_color":"#F7F9F6","icons":[{"src":"/icon.svg?v=2","sizes":"any","type":"image/svg+xml","purpose":"any"}]}`))
	})
	mux.HandleFunc("/icon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=3600")
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96"><rect width="96" height="96" rx="22" fill="#E3F0E4"/><rect x="3" y="3" width="90" height="90" rx="19" fill="none" stroke="#2F6B45" stroke-width="3"/><path d="M39 20h18M43 20v20L30 66a8 8 0 0 0 7 12h22a8 8 0 0 0 7-12L53 40V20" stroke="#2F6B45" stroke-width="5.5" fill="none" stroke-linecap="round" stroke-linejoin="round"/><path d="M34 58h28" stroke="#2F6B45" stroke-width="4.5" stroke-linecap="round"/><path d="M64 44c7-1 11-6 12-13-7 .5-11.5 5-12 13z" fill="#4E9A63"/><circle cx="48" cy="13" r="3.5" fill="#E5A93F"/></svg>`))
	})
	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write([]byte(`self.addEventListener('install',function(e){self.skipWaiting()});
self.addEventListener('activate',function(e){e.waitUntil(self.clients.claim())});
self.addEventListener('fetch',function(e){
  if(e.request.method!=='GET')return;
  e.respondWith(fetch(e.request).catch(function(){
    return caches.open('th-v2').then(function(c){return c.match(e.request)}).then(function(r){return r||new Response('offline',{status:503})});
  }).then(function(resp){
    if(resp&&resp.ok){var cl=resp.clone();caches.open('th-v2').then(function(c){c.put(e.request,cl)})}
    return resp;
  }));
});`))
	})

	// 管理面板：/admin 页面（前端带 key 拉数据）+ /api/admin/ping（key 校验 + 每日浏览）
	mux.HandleFunc("/admin", handleAdmin)
	mux.HandleFunc("/api/admin/ping", func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Access-Key")
		if got == "" {
			got = r.URL.Query().Get("key")
		}
		if !st.keyMatches(got) {
			writeJSON(w, 401, map[string]any{"ok": false, "error": "key 不对"})
			return
		}
		st.mu.Lock()
		daily := map[string]int{}
		for k, v := range st.data.DailyViews {
			daily[k] = v
		}
		today := st.data.DailyViews[time.Now().Format("2006-01-02")]
		st.mu.Unlock()
		yk := time.Now().AddDate(0, 0, -1).Format("2006-01-02")
		// 存储占用
		var dirSize int64
		var fileCount, artifactCount int
		_ = filepath.WalkDir(st.dir, func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			if info, e := d.Info(); e == nil {
				dirSize += info.Size()
			}
			rel, _ := filepath.Rel(st.dir, path)
			if strings.HasPrefix(rel, "files"+string(os.PathSeparator)) {
				fileCount++
			}
			if strings.HasPrefix(rel, "artifacts"+string(os.PathSeparator)) {
				artifactCount++
			}
			return nil
		})
		writeJSON(w, 200, map[string]any{"ok": true, "today": today, "yesterday": daily[yk], "daily": daily, "disk_size": dirSize, "file_count": fileCount, "artifact_count": artifactCount})
	})

	// 管理备份导出：zip（store.json + artifacts + files 全量）
	mux.HandleFunc("/api/admin/export", func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Access-Key")
		if got == "" {
			got = r.URL.Query().Get("key")
		}
		if !st.keyMatches(got) {
			writeJSON(w, 401, map[string]any{"ok": false, "error": "key 不对"})
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="teacherhub-backup.zip"`)
		zw := zip.NewWriter(w)
		defer zw.Close()
		add := func(name, path string) {
			if b, err := os.ReadFile(path); err == nil {
				if fw, err := zw.Create(name); err == nil {
					_, _ = fw.Write(b)
				}
			}
		}
		add("store.json", filepath.Join(st.dir, "store.json"))
		for _, sub := range []string{"artifacts", "files"} { // files：校本资源文件一并入包
			_ = filepath.WalkDir(filepath.Join(st.dir, sub), func(path string, d fs.DirEntry, err error) error {
				if err != nil || d.IsDir() {
					return nil
				}
				rel, _ := filepath.Rel(st.dir, path)
				add(filepath.ToSlash(rel), path)
				return nil
			})
		}
	})

	// MCP 端点：AI 客户端（WorkBuddy 等）直连（JSON-RPC；读免 key、写留 key）
	mux.HandleFunc("/mcp", handleMCP(st))

	// 门户首页 + 统计页（合并；浏览全免 key，打开即看）
	portal := func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/" && r.URL.Path != "/stats" {
			http.NotFound(w, r)
			return
		}
		w.Header().Set("Content-Type", "text/html; charset=utf-8")
		_, _ = w.Write([]byte(st.portalHTML()))
	}
	mux.HandleFunc("/stats", portal)
	mux.HandleFunc("/", portal)

	log.Println("教师工作台云端已启动:", addr)
	log.Println("访问 key:", st.data.Key)
	log.Println("统计页:", "http://<地址>"+addr+"/stats?key="+st.data.Key)
	log.Fatal(http.ListenAndServe(addr, withGzip(withETag(mux))))
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

// workbenchLink 生成可直接下发的链接（统一 key 随链接走，群里/学生端即点即看）。
func workbenchLink(r *http.Request, id string) string {
	scheme := "http"
	if r.TLS != nil || r.Header.Get("X-Forwarded-Proto") == "https" {
		scheme = "https"
	}
	return fmt.Sprintf("%s://%s/w/%s", scheme, r.Host, id)
}

// clientIP 取真实客户端 IP：优先校验过的 CF-Connecting-IP/XFF，防伪造头，回落 RemoteAddr。
func clientIP(r *http.Request) string {
	for _, h := range []string{"CF-Connecting-IP", "X-Forwarded-For"} {
		v := strings.TrimSpace(r.Header.Get(h))
		if h == "X-Forwarded-For" && strings.Contains(v, ",") {
			v = strings.TrimSpace(strings.SplitN(v, ",", 2)[0])
		}
		if v != "" && net.ParseIP(v) != nil {
			return v
		}
	}
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i > 0 {
		ip = ip[:i]
	}
	return ip
}

// boardDedup 记去重键（超限整体清空防膨胀）。
func boardDedup(st *cloudStore, key string) (already bool) {
	st.mu.Lock()
	defer st.mu.Unlock()
	if st.boardLikes == nil {
		st.boardLikes = map[string]bool{}
	}
	if st.boardLikes[key] {
		return true
	}
	if len(st.boardLikes) > 20000 {
		st.boardLikes = map[string]bool{}
	}
	st.boardLikes[key] = true
	return false
}

// applyRating 评分（REST/MCP 共享）：先定位产物再计分与去重；found=false 表示产物不存在。
func applyRating(st *cloudStore, ip, id string, stars int) (rate float64, already, found bool) {
	if boardDedup(st, ip+"|rate|"+id) {
		st.mu.Lock()
		defer st.mu.Unlock()
		for i := range st.data.Workbenches {
			if st.data.Workbenches[i].ID == id {
				return rateOf(st.data.Workbenches[i]), true, true
			}
		}
		return 0, true, false
	}
	st.mu.Lock()
	for i := range st.data.Workbenches {
		if st.data.Workbenches[i].ID == id {
			st.data.Workbenches[i].RateSum += stars
			st.data.Workbenches[i].RateNum++
			rate = rateOf(st.data.Workbenches[i])
			found = true
			break
		}
	}
	if found {
		_ = st.save()
	}
	st.mu.Unlock()
	if !found { // 产物不存在：退还去重位，避免拼错 id 永久占坑
		st.mu.Lock()
		delete(st.boardLikes, ip+"|rate|"+id)
		st.mu.Unlock()
	}
	return rate, false, found
}

// listBackups 备份快照清单（REST/MCP 共享）。
func listBackups(st *cloudStore) []map[string]any {
	list := []map[string]any{}
	if entries, err := os.ReadDir(filepath.Join(st.dir, "backups")); err == nil {
		for i := len(entries) - 1; i >= 0; i-- {
			if fi, err := entries[i].Info(); err == nil {
				list = append(list, map[string]any{"name": entries[i].Name(), "size": fi.Size(), "ts": fi.ModTime().Unix()})
			}
		}
	}
	return list
}

// rateH 评分处理（免 key，每 IP 每产物一评）。
func rateH(st *cloudStore, w http.ResponseWriter, r *http.Request, id string) {
	var in struct {
		Stars int `json:"stars"`
	}
	if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Stars < 1 || in.Stars > 5 {
		writeJSON(w, 400, map[string]any{"ok": false, "error": "stars 须为 1-5"})
		return
	}
	rate, already, found := applyRating(st, clientIP(r), id, in.Stars)
	if !found {
		writeJSON(w, 404, map[string]any{"ok": false, "error": "产物不存在"})
		return
	}
	writeJSON(w, 200, map[string]any{"ok": true, "already": already, "rate": rate})
}

// rateOf 均分（0~5，保留 1 位）。
func rateOf(w Workbench) float64 {
	if w.RateNum == 0 {
		return 0
	}
	return float64(int(float64(w.RateSum)/float64(w.RateNum)*10+0.5)) / 10
}

func wbView(w Workbench, r *http.Request) map[string]any {
	pv := ""
	if w.HasPreview {
		pv = "/api/workbenches/" + w.ID + "/preview"
	}
	return map[string]any{
		"id": w.ID, "title": w.Title, "desc": w.Desc, "tags": w.Tags,
		"keywords": w.Keywords, "teacher": w.Teacher, "size": w.Size,
		"downloads": w.Downloads, "views": w.Views, "created_at": w.CreatedAt,
		"preview":  pv,
		"hidden":   w.Hidden,
		"rate":     rateOf(w),
		"rate_num": w.RateNum,
		"link":     workbenchLink(r, w.ID),
	}
}

// ---------- 相似度（中文友好的二元切片重叠） ----------

func shingles(s string) map[string]bool {
	out := map[string]bool{}
	s = strings.ToLower(s)
	var buf []rune
	flushWord := func() {
		if len(buf) > 0 {
			out[string(buf)] = true
			buf = nil
		}
	}
	runes := []rune(s)
	for i := 0; i < len(runes); i++ {
		c := runes[i]
		if c >= 0x4e00 && c <= 0x9fff { // CJK：二元组
			flushWord()
			if i+1 < len(runes) && runes[i+1] >= 0x4e00 && runes[i+1] <= 0x9fff {
				out[string(runes[i:i+2])] = true
			} else {
				out[string(c)] = true
			}
		} else if (c >= 'a' && c <= 'z') || (c >= '0' && c <= '9') {
			buf = append(buf, c)
		} else {
			flushWord()
		}
	}
	flushWord()
	return out
}

// similarity 查询串 vs 产物元数据（标题权重加倍），返回 0~1。
func similarity(q string, w Workbench) float64 {
	qs := shingles(q)
	if len(qs) == 0 {
		return 0
	}
	hay := shingles(w.Title + " " + w.Title + " " + strings.Join(w.Tags, " ") + " " + strings.Join(w.Keywords, " ") + " " + w.Desc)
	hit := 0
	for g := range qs {
		if hay[g] {
			hit++
		}
	}
	sc := float64(hit) / float64(len(qs))
	if w.Title != "" && strings.Contains(strings.ToLower(w.Title), strings.ToLower(strings.TrimSpace(q))) {
		sc = 1
	}
	return sc
}
