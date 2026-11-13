package main

// cloud.go —— 云端服务（teacherhub serve）：零账号、统一访问 key。
// 职责两件：①ima 贡献统计 ②工作台产物库（相似检索 / 下发链接 / 统计页）。
// 存储：data/store.json（元数据）+ data/artifacts/{id}.html（产物）。

import (
	"crypto/rand"
	"archive/zip"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"io/fs"
	"log"
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
	ID        string   `json:"id"`
	Title     string   `json:"title"`
	Desc      string   `json:"desc,omitempty"`
	Tags      []string `json:"tags,omitempty"`
	Keywords  []string `json:"keywords,omitempty"`
	Teacher   string   `json:"teacher,omitempty"`
	Size      int64    `json:"size"`
	Downloads int      `json:"downloads"`
	Views     int      `json:"views"`
	HasPreview bool    `json:"has_preview"`
	Hidden     bool    `json:"hidden"`
	RateSum    int      `json:"rate_sum"`
	RateNum    int      `json:"rate_num"`
	CreatedAt int64    `json:"created_at"`
}

// BoardMsg 留言墙条目（开放留言，限速防刷）。
type BoardMsg struct {
	ID   string `json:"id"`
	Name string `json:"name"`
	Text string `json:"text"`
	Likes int   `json:"likes"`
	At   int64  `json:"at"`
}

type cloudStore struct {
	mu   sync.Mutex
	dir  string
	data struct {
		Key         string      `json:"key"`
		Contribs    []Contrib   `json:"contribs"`
		Workbenches []Workbench `json:"workbenches"`
		Board       []BoardMsg  `json:"board,omitempty"`
		Notice      string      `json:"notice,omitempty"`
		DailyViews  map[string]int `json:"daily_views,omitempty"`
	}
	boardLast map[string]int64 // IP → 上次留言时间（内存限速）
	boardLikes map[string]bool  // ip|msgID → 已赞
}

func openStore(dir, key string) (*cloudStore, error) {
	if err := os.MkdirAll(filepath.Join(dir, "artifacts"), 0o755); err != nil {
		return nil, err
	}
	s := &cloudStore{dir: dir, boardLast: map[string]int64{}, boardLikes: map[string]bool{}}
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

func randToken() string {
	b := make([]byte, 16)
	_, _ = rand.Read(b)
	return hex.EncodeToString(b)
}

// autoBackup 启动时与每 24h 自动快照（zip：store.json + artifacts，轮转留 7 份）。
func autoBackup(st *cloudStore) {
	do := func() {
		bdir := filepath.Join(st.dir, "backups")
		_ = os.MkdirAll(bdir, 0o755)
		f, err := os.Create(filepath.Join(bdir, fmt.Sprintf("backup-%d.zip", time.Now().Unix())))
		if err != nil {
			return
		}
		zw := zip.NewWriter(f)
		add := func(n, p string) {
			if b, err := os.ReadFile(p); err == nil {
				if fw, err := zw.Create(n); err == nil {
					_, _ = fw.Write(b)
				}
			}
		}
		add("store.json", filepath.Join(st.dir, "store.json"))
		_ = filepath.WalkDir(filepath.Join(st.dir, "artifacts"), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(st.dir, path)
			add(filepath.ToSlash(rel), path)
			return nil
		})
		_ = zw.Close()
		_ = f.Close()
		if entries, err := os.ReadDir(bdir); err == nil && len(entries) > 7 {
			sort.Slice(entries, func(a, b int) bool { return entries[a].Name() < entries[b].Name() })
			for _, e := range entries[:len(entries)-7] {
				_ = os.Remove(filepath.Join(bdir, e.Name()))
			}
		}
	}
	do()
	go func() {
		for range time.Tick(24 * time.Hour) {
			do()
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
		if got == "" || got != st.data.Key {
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
				if err := json.NewDecoder(r.Body).Decode(&in); err != nil || in.Title == "" || in.Content == "" {
					writeJSON(w, 400, map[string]any{"ok": false, "error": "缺 title/content"})
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
						found = true
						break
					}
				}
				if found {
					_ = st.save()
				}
				st.mu.Unlock()
				if found {
					_ = os.Remove(filepath.Join(st.dir, "artifacts", id+".html"))
					_ = os.Remove(filepath.Join(st.dir, "artifacts", id+".preview"))
				}
				writeJSON(w, 200, map[string]any{"ok": found})
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

	// 留言墙：免 key 开放留言（60s/条限速 + 200 字限制，滚动存 200 条）
	mux.HandleFunc("/api/board", func(w http.ResponseWriter, r *http.Request) {
		if r.Method == http.MethodDelete { // 删除留言（管理 key）
			got := r.Header.Get("X-Access-Key")
			if got == "" {
				got = r.URL.Query().Get("key")
			}
			if got != st.data.Key {
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
			if got != st.data.Key {
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
		b.WriteString(`<?xml version="1.0" encoding="UTF-8"?><rss version="2.0"><channel><title>教研工作台 · 新产物</title><link>` + base + `/</link><description>教研工作台产物库更新订阅</description>`)
		for _, wb := range list {
			b.WriteString(`<item><title>` + html.EscapeString(wb.Title) + `</title><link>` + base + `/w/` + wb.ID + `</link><pubDate>` + time.Unix(wb.CreatedAt, 0).Format(time.RFC1123Z) + `</pubDate><description>` + html.EscapeString(wb.Desc) + `</description></item>`)
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
		if got != st.data.Key {
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

	// 迷你 QR 编码器（查看页扫码用）
	mux.HandleFunc("/qr.js", handleQRJS)

	// PWA：manifest + sw + 图标（手机可"添加到主屏幕"）
	mux.HandleFunc("/manifest.webmanifest", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/manifest+json; charset=utf-8")
		_, _ = w.Write([]byte(`{"name":"教研工作台","short_name":"教研台","start_url":"/","display":"standalone","theme_color":"#15616d","background_color":"#f4f2ec","icons":[{"src":"/icon.svg","sizes":"any","type":"image/svg+xml","purpose":"any"}]}`))
	})
	mux.HandleFunc("/icon.svg", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "image/svg+xml")
		w.Header().Set("Cache-Control", "public, max-age=86400")
		_, _ = w.Write([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 96 96"><rect width="96" height="96" rx="22" fill="#15616d"/><path d="M24 30c8-4 16-4 24 0v40c-8-4-16-4-24 0V30z" fill="#fff"/><path d="M72 30c-8-4-16-4-24 0v40c8-4 16-4 24 0V30z" fill="#bfe0e4"/><circle cx="48" cy="22" r="5" fill="#e8a33d"/></svg>`))
	})
	mux.HandleFunc("/sw.js", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/javascript; charset=utf-8")
		_, _ = w.Write([]byte(`self.addEventListener('install',function(e){self.skipWaiting()});
self.addEventListener('activate',function(e){e.waitUntil(self.clients.claim())});
self.addEventListener('fetch',function(e){
  if(e.request.method!=='GET')return;
  e.respondWith(fetch(e.request).catch(function(){
    return caches.open('th-v1').then(function(c){return c.match(e.request)}).then(function(r){return r||new Response('offline',{status:503})});
  }).then(function(resp){
    if(resp&&resp.ok){var cl=resp.clone();caches.open('th-v1').then(function(c){c.put(e.request,cl)})}
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
		if got != st.data.Key {
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
		writeJSON(w, 200, map[string]any{"ok": true, "today": today, "yesterday": daily[yk], "daily": daily})
	})

	// 管理备份导出：zip（store.json + artifacts 全量）
	mux.HandleFunc("/api/admin/export", func(w http.ResponseWriter, r *http.Request) {
		got := r.Header.Get("X-Access-Key")
		if got == "" {
			got = r.URL.Query().Get("key")
		}
		if got != st.data.Key {
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
		_ = filepath.WalkDir(filepath.Join(st.dir, "artifacts"), func(path string, d fs.DirEntry, err error) error {
			if err != nil || d.IsDir() {
				return nil
			}
			rel, _ := filepath.Rel(st.dir, path)
			add(filepath.ToSlash(rel), path)
			return nil
		})
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
	log.Fatal(http.ListenAndServe(addr, mux))
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
		"preview":   pv,
		"hidden":    w.Hidden,
		"rate":      rateOf(w),
		"rate_num":  w.RateNum,
		"link":      workbenchLink(r, w.ID),
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


