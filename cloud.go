package main

// cloud.go —— 云端服务（teacherhub serve）：零账号、统一访问 key。
// 职责两件：①ima 贡献统计 ②工作台产物库（相似检索 / 下发链接 / 统计页）。
// 存储：data/store.json（元数据）+ data/artifacts/{id}.html（产物）。

import (
	"crypto/rand"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"html"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"sort"
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
			writeJSON(w, 200, map[string]any{
				"ok": true, "total_workbenches": len(st.data.Workbenches),
				"total_contribs": len(st.data.Contribs),
				"by_teacher":     list, "recent": recent,
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
			st.mu.Lock()
			list := make([]Workbench, len(st.data.Workbenches))
			copy(list, st.data.Workbenches)
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
			list := make([]Workbench, len(st.data.Workbenches))
			copy(list, st.data.Workbenches)
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
		api(w, r, func(w http.ResponseWriter, r *http.Request) {
			id := strings.TrimPrefix(r.URL.Path, "/api/workbenches/")
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
			ip := r.Header.Get("CF-Connecting-IP")
			if ip == "" {
				ip = r.Header.Get("X-Forwarded-For")
				if i := strings.Index(ip, ","); i > 0 {
					ip = strings.TrimSpace(ip[:i])
				}
			}
			if ip == "" { // 直连（局域网/探针）
				ip = r.RemoteAddr
				if i := strings.LastIndex(ip, ":"); i > 0 {
					ip = ip[:i]
				}
			}
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
		ip := r.Header.Get("CF-Connecting-IP")
		if ip == "" {
			ip = r.RemoteAddr
			if i := strings.LastIndex(ip, ":"); i > 0 {
				ip = ip[:i]
			}
		}
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


