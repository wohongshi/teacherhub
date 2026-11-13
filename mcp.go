package main

// /mcp —— teacherhub 的 MCP 端点（MCP over Streamable HTTP 的单请求单响应子集）。
// 供 WorkBuddy / 任意 AI 客户端直连：检索产物库、取产物、报贡献、推产物。
// 访问政策与 REST 一致：读工具免 key；写工具（add_contrib / push_workbench）须统一访问 key
// （HTTP 头 X-Access-Key 或查询串 ?key=）。
//
// JSON-RPC 2.0 方法：initialize / notifications/initialized / tools/list / tools/call。

import (
	"encoding/json"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const mcpProtocolVersion = "2024-11-05"

type mcpReq struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      json.RawMessage `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params"`
}

type mcpTool struct {
	Name        string          `json:"name"`
	Description string          `json:"description"`
	InputSchema json.RawMessage `json:"inputSchema"`
}

// mcpTools 五件工具的清单（schema 用紧凑 JSON 字面量，零依赖）。
var mcpTools = []mcpTool{
	{Name: "search_workbenches", Description: "按关键词相似检索产物库（中文模糊匹配）。返回最相似的工作台产物列表（含 id/标题/简介/标签/链接）。",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"q":{"type":"string","description":"检索关键词，如「钠与水」「凸透镜」"},"limit":{"type":"integer","description":"返回条数上限，默认 10"}},"required":["q"]}`)},
	{Name: "get_workbench", Description: "按 id 获取单个工作台产物的元数据（标题/简介/标签/作者/热度/链接）。",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"id":{"type":"string","description":"产物 id，如 wb_xxxxxxxx"}},"required":["id"]}`)},
	{Name: "list_workbenches", Description: "列出产物库全部工作台（按时间倒序，含元数据与链接）。",
		InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
	{Name: "add_contrib", Description: "记录一条教师贡献（写操作，需要统一访问 key）。用于把「某老师贡献了什么资料/实验」记入云端统计。",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"teacher":{"type":"string","description":"教师姓名"},"text":{"type":"string","description":"贡献内容描述"},"kb":{"type":"string","description":"所属知识库（可选）"},"item":{"type":"string","description":"相关条目（可选）"}},"required":["teacher","text"]}`)},
	{Name: "push_workbench", Description: "推送一个工作台产物到云端产物库（写操作，需要统一访问 key）。content 为完整单文件 HTML。推送成功返回可下发的免 key 链接。",
		InputSchema: json.RawMessage(`{"type":"object","properties":{"title":{"type":"string","description":"产物标题"},"desc":{"type":"string","description":"一句话简介"},"tags":{"type":"array","items":{"type":"string"},"description":"标签，如 [\"化学\",\"实验\"]"},"keywords":{"type":"array","items":{"type":"string"},"description":"检索关键词"},"teacher":{"type":"string","description":"作者姓名"},"content":{"type":"string","description":"完整单文件 HTML 内容"}},"required":["title","content"]}`)},
	{Name: "get_board", Description: "读取留言墙最近留言（免 key）。返回昵称/内容/点赞数/时间。", InputSchema: json.RawMessage(`{"type":"object","properties":{"limit":{"type":"integer","description":"返回条数，默认 10，最多 30"}}}`)},
	{Name: "get_stats", Description: "获取云端统计（免 key）：参与教师/贡献/产物/浏览热度/今日浏览/留言数。", InputSchema: json.RawMessage(`{"type":"object","properties":{}}`)},
}

// mcpText 成功结果包装成 MCP content[] 文本。
func mcpText(v any) map[string]any {
	b, _ := json.MarshalIndent(v, "", "  ")
	return map[string]any{"content": []map[string]any{{"type": "text", "text": string(b)}}}
}

// mcpErr 工具执行错误（isError，友好文本）。
func mcpErr(msg string) map[string]any {
	return map[string]any{"content": []map[string]any{{"type": "text", "text": msg}}, "isError": true}
}

func handleMCP(st *cloudStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			writeJSON(w, 405, map[string]any{"jsonrpc": "2.0", "error": map[string]any{"code": -32600, "message": "只支持 POST"}})
			return
		}
		var req mcpReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			writeJSON(w, 400, map[string]any{"jsonrpc": "2.0", "error": map[string]any{"code": -32700, "message": "JSON 解析失败"}})
			return
		}
		reply := func(result any) {
			writeJSON(w, 200, map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(firstNotNull(req.ID)), "result": result})
		}
		switch req.Method {
		case "initialize":
			reply(map[string]any{
				"protocolVersion": mcpProtocolVersion,
				"capabilities":    map[string]any{"tools": map[string]any{}},
				"serverInfo":      map[string]any{"name": "teacherhub", "version": version},
				"instructions":    "教研工作台云端：检索/获取工作台产物免 key；报贡献与推产物为写操作，需在 X-Access-Key 头或 ?key= 传统一访问 key。",
			})
		case "notifications/initialized", "notifications/cancelled":
			w.WriteHeader(202) // 通知无响应体
		case "tools/list":
			reply(map[string]any{"tools": mcpTools})
		case "tools/call":
			callMCP(st, w, r, req)
		default:
			writeJSON(w, 200, map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(firstNotNull(req.ID)), "error": map[string]any{"code": -32601, "message": "未知方法: " + req.Method}})
		}
	}
}

// firstNotNull 保证 id 非空（JSON-RPC 响应 id 不能缺）。
func firstNotNull(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("null")
	}
	return raw
}

func callMCP(st *cloudStore, w http.ResponseWriter, r *http.Request, req mcpReq) {
	var p struct {
		Name      string          `json:"name"`
		Arguments json.RawMessage `json:"arguments"`
	}
	_ = json.Unmarshal(req.Params, &p)
	args := map[string]any{}
	_ = json.Unmarshal(p.Arguments, &args)
	reply := func(result any) {
		writeJSON(w, 200, map[string]any{"jsonrpc": "2.0", "id": json.RawMessage(firstNotNull(req.ID)), "result": result})
	}
	// 写操作鉴权：X-Access-Key 或 ?key=（与 REST 同一政策）
	keyOK := func() bool {
		got := r.Header.Get("X-Access-Key")
		if got == "" {
			got = r.URL.Query().Get("key")
		}
		if k, _ := args["_key"].(string); got == "" && k != "" {
			got = k
		}
		return got != "" && got == st.data.Key
	}
	switch p.Name {
	case "search_workbenches":
		q, _ := args["q"].(string)
		limit := 10
		if n, ok := args["limit"].(float64); ok && n > 0 && n <= 50 {
			limit = int(n)
		}
		q = strings.TrimSpace(q)
		st.mu.Lock()
		list := make([]Workbench, len(st.data.Workbenches))
		copy(list, st.data.Workbenches)
		st.mu.Unlock()
		type match struct {
			Workbench map[string]any `json:"workbench"`
			Score     float64        `json:"score"`
		}
		matches := []match{}
		if q != "" {
			for _, wb := range list {
				if sc := similarity(q, wb); sc >= 0.3 {
					matches = append(matches, match{Workbench: wbView(wb, r), Score: sc})
				}
			}
			// 插入排序按分数降序（零依赖）
			for i := 1; i < len(matches); i++ {
				for j := i; j > 0 && matches[j].Score > matches[j-1].Score; j-- {
					matches[j], matches[j-1] = matches[j-1], matches[j]
				}
			}
		}
		if len(matches) > limit {
			matches = matches[:limit]
		}
		reply(mcpText(map[string]any{"ok": true, "matches": matches}))
	case "get_workbench":
		id, _ := args["id"].(string)
		st.mu.Lock()
		for _, wb := range st.data.Workbenches {
			if wb.ID == id {
				st.mu.Unlock()
				reply(mcpText(map[string]any{"ok": true, "workbench": wbView(wb, r)}))
				return
			}
		}
		st.mu.Unlock()
		reply(mcpErr("产物不存在: " + id))
	case "list_workbenches":
		st.mu.Lock()
		list := make([]Workbench, len(st.data.Workbenches))
		copy(list, st.data.Workbenches)
		st.mu.Unlock()
		// 时间倒序
		for i := 1; i < len(list); i++ {
			for j := i; j > 0 && list[j].CreatedAt > list[j-1].CreatedAt; j-- {
				list[j], list[j-1] = list[j-1], list[j]
			}
		}
		out := []map[string]any{}
		for _, wb := range list {
			out = append(out, wbView(wb, r))
		}
		reply(mcpText(map[string]any{"ok": true, "workbenches": out}))
	case "add_contrib":
		if !keyOK() {
			reply(mcpErr("写操作需要统一访问 key（X-Access-Key 头或 ?key=）"))
			return
		}
		teacher, _ := args["teacher"].(string)
		text, _ := args["text"].(string)
		kb, _ := args["kb"].(string)
		item, _ := args["item"].(string)
		if teacher == "" || text == "" {
			reply(mcpErr("缺 teacher/text"))
			return
		}
		c := Contrib{Teacher: strings.TrimSpace(teacher), Text: strings.TrimSpace(text), KB: kb, Item: item, At: time.Now().Unix()}
		st.mu.Lock()
		st.data.Contribs = append(st.data.Contribs, c)
		_ = st.save()
		st.mu.Unlock()
		reply(mcpText(map[string]any{"ok": true, "at": c.At}))
	case "push_workbench":
		if !keyOK() {
			reply(mcpErr("写操作需要统一访问 key（X-Access-Key 头或 ?key=）"))
			return
		}
		title, _ := args["title"].(string)
		content, _ := args["content"].(string)
		if title == "" || content == "" {
			reply(mcpErr("缺 title/content"))
			return
		}
		desc, _ := args["desc"].(string)
		teacher, _ := args["teacher"].(string)
		strs := func(k string) []string {
			out := []string{}
			if arr, ok := args[k].([]any); ok {
				for _, v := range arr {
					if s, ok := v.(string); ok {
						out = append(out, s)
					}
				}
			}
			return out
		}
		wb := Workbench{
			ID: "wb_" + randToken()[:12], Title: strings.TrimSpace(title),
			Desc: strings.TrimSpace(desc), Tags: strs("tags"), Keywords: strs("keywords"),
			Teacher: strings.TrimSpace(teacher), Size: int64(len(content)),
			CreatedAt: time.Now().Unix(),
		}
		if err := os.WriteFile(filepath.Join(st.dir, "artifacts", wb.ID+".html"), []byte(content), 0o644); err != nil {
			reply(mcpErr("落盘失败: " + err.Error()))
			return
		}
		st.mu.Lock()
		st.data.Workbenches = append(st.data.Workbenches, wb)
		_ = st.save()
		st.mu.Unlock()
		reply(mcpText(map[string]any{"ok": true, "id": wb.ID, "link": workbenchLink(r, wb.ID)}))
	case "get_board":
		limit := 10
		if n, ok := args["limit"].(float64); ok && n > 0 && n <= 30 {
			limit = int(n)
		}
		st.mu.Lock()
		blist := make([]BoardMsg, len(st.data.Board))
		copy(blist, st.data.Board)
		st.mu.Unlock()
		for i := 1; i < len(blist); i++ {
			for j := i; j > 0 && blist[j].At > blist[j-1].At; j-- {
				blist[j], blist[j-1] = blist[j-1], blist[j]
			}
		}
		if len(blist) > limit {
			blist = blist[:limit]
		}
		reply(mcpText(map[string]any{"ok": true, "board": blist}))
	case "get_stats":
		st.mu.Lock()
		teachers := map[string]bool{}
		views, downloads := 0, 0
		today := st.data.DailyViews[time.Now().Format("2006-01-02")]
		for _, c := range st.data.Contribs {
			teachers[c.Teacher] = true
		}
		for _, w2 := range st.data.Workbenches {
			if w2.Teacher != "" {
				teachers[w2.Teacher] = true
			}
			views += w2.Views
			downloads += w2.Downloads
		}
		stats := map[string]any{"ok": true, "teachers": len(teachers), "contribs": len(st.data.Contribs),
			"workbenches": len(st.data.Workbenches), "views": views, "downloads": downloads,
			"today_views": today, "board_msgs": len(st.data.Board)}
		st.mu.Unlock()
		reply(mcpText(stats))
	default:
		reply(mcpErr("未知工具: " + p.Name))
	}
}
