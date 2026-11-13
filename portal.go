package main

// portal.go —— 云端网页（浏览全免 key）：门户首页（统计/产物库/贡献榜/搜索）+ 产物查看页。
// 数据服务端渲染，搜索走开放的只读检索接口；所有用户字符串均 html 转义。

import (
	"fmt"
	"html"
	"sort"
	"strings"
	"time"
)

// portalHTML 门户首页（/ 与 /stats 同页）。
func (s *cloudStore) portalHTML() string {
	s.mu.Lock()
	wbs := make([]Workbench, len(s.data.Workbenches))
	copy(wbs, s.data.Workbenches)
	contribs := make([]Contrib, len(s.data.Contribs))
	copy(contribs, s.data.Contribs)
	s.mu.Unlock()
	sort.Slice(wbs, func(i, j int) bool { return wbs[i].CreatedAt > wbs[j].CreatedAt })
	sort.Slice(contribs, func(i, j int) bool { return contribs[i].At > contribs[j].At })

	// 教师聚合
	type row struct {
		Name string
		C, W int
		Last int64
	}
	m := map[string]*row{}
	for _, c := range contribs {
		r := m[c.Teacher]
		if r == nil {
			r = &row{Name: c.Teacher}
			m[c.Teacher] = r
		}
		r.C++
		if c.At > r.Last {
			r.Last = c.At
		}
	}
	for _, w2 := range wbs {
		if w2.Teacher == "" {
			continue
		}
		r := m[w2.Teacher]
		if r == nil {
			r = &row{Name: w2.Teacher}
			m[w2.Teacher] = r
		}
		r.W++
		if w2.CreatedAt > r.Last {
			r.Last = w2.CreatedAt
		}
	}
	rows := []*row{}
	for _, r := range m {
		rows = append(rows, r)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].C+rows[i].W != rows[j].C+rows[j].W {
			return rows[i].C+rows[i].W > rows[j].C+rows[j].W
		}
		return rows[i].Last > rows[j].Last
	})

	var b strings.Builder
	b.WriteString(`<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>教研工作台</title>
<style>
:root{--paper:#f4f2ec;--panel:#fff;--ink:#22282e;--muted:#6b7280;--line:#e3dfd4;--accent:#15616d;--accent-soft:#e2eef0;--amber:#c26a1e}
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:"PingFang SC","Microsoft YaHei",sans-serif;background:var(--paper);color:var(--ink)}
.hero{background:linear-gradient(135deg,#12333a 0%,#1c505b 60%,#21626e 100%);color:#eef6f6;padding:34px 22px 30px}
.wrap{max-width:1080px;margin:0 auto}
.brand{display:flex;align-items:center;gap:10px;font-size:14px;opacity:.92;letter-spacing:.5px}
.brand .logo{width:30px;height:30px;border-radius:9px;background:rgba(255,255,255,.16);display:flex;align-items:center;justify-content:center;font-size:16px}
h1{font-family:"Songti SC","STSong","SimSun",serif;font-size:30px;letter-spacing:1px;margin:16px 0 8px;font-weight:700}
.sub{font-size:14px;opacity:.8;line-height:1.8;max-width:560px}
.search{display:flex;gap:10px;margin:20px 0 22px;max-width:560px}
.search input{flex:1;min-height:46px;border-radius:12px;border:0;padding:0 16px;font-size:15px;font-family:inherit}
.search button{min-height:46px;padding:0 22px;border:0;border-radius:12px;background:#fff;color:#12333a;font-size:15px;font-weight:700;cursor:pointer}
.search button:hover{background:#d9e9ea}
.stats{display:flex;gap:26px;flex-wrap:wrap}
.stat b{display:block;font-family:Georgia,"Songti SC",serif;font-size:32px;line-height:1.1}
.stat span{font-size:12.5px;opacity:.75}
main{padding:22px}
.cols{display:grid;grid-template-columns:minmax(0,1.7fr) minmax(280px,1fr);gap:18px;align-items:start}
h2{font-size:16px;color:var(--accent);margin:4px 2px 12px;display:flex;align-items:center;gap:8px}
h2 .count{font-size:12px;background:var(--accent-soft);color:var(--accent);border-radius:999px;padding:2px 10px}
.wb-grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(280px,1fr));gap:14px}
.wb{display:block;background:var(--panel);border:1px solid var(--line);border-radius:14px;padding:16px 17px;text-decoration:none;color:var(--ink);box-shadow:0 2px 8px rgba(34,40,46,.05);transition:all .15s}
.wb:hover{transform:translateY(-2px);box-shadow:0 8px 20px rgba(34,40,46,.10);border-color:#c9d9dc}
.wb .t{font-size:16px;font-weight:700;line-height:1.4}
.wb .d{font-size:13px;color:var(--muted);margin:7px 0 12px;line-height:1.7;min-height:2.6em}
.wb .tags{display:flex;flex-wrap:wrap;gap:6px;margin-bottom:12px}
.tag{font-size:11.5px;background:var(--accent-soft);color:var(--accent);border-radius:999px;padding:3px 10px}
.wb .meta{display:flex;align-items:center;gap:8px;font-size:12px;color:var(--muted)}
.avatar{width:24px;height:24px;border-radius:50%;display:inline-flex;align-items:center;justify-content:center;color:#fff;font-size:12px;font-weight:700;flex:none}
.avatar.lg{width:34px;height:34px;font-size:15px}
.card{background:var(--panel);border:1px solid var(--line);border-radius:14px;padding:16px 17px;margin-bottom:16px;box-shadow:0 2px 8px rgba(34,40,46,.05)}
.trow{display:flex;align-items:center;gap:10px;padding:9px 2px;border-bottom:1px solid #f0eee8}
.trow:last-child{border-bottom:0}
.trow .nm{font-size:14px;font-weight:600;flex:1}
.trow .cnt{font-size:11.5px;background:var(--accent-soft);color:var(--accent);border-radius:999px;padding:2px 9px;white-space:nowrap}
.feed{font-size:13px}
.feed .item{display:flex;gap:10px;padding:9px 2px;border-bottom:1px solid #f0eee8;line-height:1.6}
.feed .item:last-child{border-bottom:0}
.feed .txt{flex:1}
.feed .tm{font-size:11px;color:var(--muted);white-space:nowrap}
.empty{color:var(--muted);font-size:13.5px;padding:26px 10px;text-align:center}
footer{text-align:center;color:var(--muted);font-size:12px;padding:22px}
@media (max-width:900px){
  .cols{grid-template-columns:1fr}
  h1{font-size:23px}
  .hero{padding:24px 16px}
  main{padding:14px}
}
</style></head><body>
<header class="hero"><div class="wrap">
  <div class="brand"><span class="logo">📚</span><span>教研工作台 · 教研组共享</span></div>
  <h1>教研成果，一处共享</h1>
  <div class="sub">每位老师贡献的资料、做出来的实验工作台，都在这里沉淀复用——先搜一下，别重复造。</div>
  <div class="search"><input id="q" placeholder="搜工作台：如「钠与水」「铁丝燃烧」"><button id="qbtn">搜索</button></div>
  <div class="stats">
    <div class="stat"><b>` + fmt.Sprint(len(rows)) + `</b><span>参与教师</span></div>
    <div class="stat"><b>` + fmt.Sprint(len(contribs)) + `</b><span>贡献记录</span></div>
    <div class="stat"><b>` + fmt.Sprint(len(wbs)) + `</b><span>工作台产物</span></div>
  </div>
</div></header>
<main><div class="wrap"><div class="cols">
  <section>
    <h2>工作台产物库 <span class="count" id="wb-count">` + fmt.Sprint(len(wbs)) + `</span></h2>
    <div class="wb-grid" id="grid">`)
	if len(wbs) == 0 {
		b.WriteString(`<div class="empty">还没有产物——在 WorkBuddy 里说「做个 XX 实验工作台」，生成后自动上架</div>`)
	}
	for _, w2 := range wbs {
		b.WriteString(wbCardHTML(w2))
	}
	b.WriteString(`</div>
  </section>
  <aside>
    <div class="card">
      <h2>教师贡献榜</h2>`)
	if len(rows) == 0 {
		b.WriteString(`<div class="empty">还没有贡献记录</div>`)
	}
	for _, r := range rows {
		fmt.Fprintf(&b, `<div class="trow"><span class="avatar lg" style="background:%s">%s</span><span class="nm">%s</span><span class="cnt">贡献 %d</span><span class="cnt">工作台 %d</span></div>`,
			avatarColor(r.Name), avatarChar(r.Name), html.EscapeString(r.Name), r.C, r.W)
	}
	b.WriteString(`</div>
    <div class="card">
      <h2>最近贡献</h2>
      <div class="feed">`)
	if len(contribs) == 0 {
		b.WriteString(`<div class="empty">还没有贡献，等你来记第一条</div>`)
	}
	if len(contribs) > 12 {
		contribs = contribs[:12]
	}
	for _, c := range contribs {
		fmt.Fprintf(&b, `<div class="item"><span class="avatar" style="background:%s">%s</span><span class="txt"><b>%s</b> %s</span><span class="tm">%s</span></div>`,
			avatarColor(c.Teacher), avatarChar(c.Teacher), html.EscapeString(c.Teacher), html.EscapeString(c.Text), time.Unix(c.At, 0).Format("01-02 15:04"))
	}
	b.WriteString(`</div></div>
  </aside>
</div></div></main>
<footer>教研工作台 · 零账号 · 浏览免 key · 写入走统一 key</footer>
<script>
(function(){
  var q=document.getElementById('q'),btn=document.getElementById('qbtn'),grid=document.getElementById('grid'),cnt=document.getElementById('wb-count');
  function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,function(c){return({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]})}
  function card(w){
    var tags=(w.tags||[]).map(function(t){return '<span class="tag">'+esc(t)+'</span>'}).join('');
    var ch=esc((w.teacher||'?').slice(0,1));
    return '<a class="wb" href="/w/'+encodeURIComponent(w.id)+'"><div class="t">'+esc(w.title)+'</div>'+
      '<div class="d">'+esc(w.desc||'')+'</div>'+
      (tags?'<div class="tags">'+tags+'</div>':'')+
      '<div class="meta"><span class="avatar" style="background:#15616d">'+ch+'</span>'+esc(w.teacher||'—')+'</div></a>';
  }
  async function search(){
    var v=q.value.trim();
    if(!v){location.reload();return}
    btn.disabled=true;btn.textContent='搜索中…';
    try{
      var r=await fetch('/api/workbenches/search?q='+encodeURIComponent(v)+'&limit=12');
      var j=await r.json();
      var ms=j.matches||[];
      cnt.textContent=ms.length;
      grid.innerHTML=ms.length?ms.map(function(m){return card(m.workbench)}).join(''):'<div class="empty">没有找到类似的，可以让老师在 WorkBuddy 里生成一个</div>';
    }catch(e){grid.innerHTML='<div class="empty">搜索失败，稍后再试</div>'}
    btn.disabled=false;btn.textContent='搜索';
  }
  btn.addEventListener('click',search);
  q.addEventListener('keydown',function(e){if(e.key==='Enter')search()});
})();
</script></body></html>`)
	return b.String()
}

// wbCardHTML 产物卡片。
func wbCardHTML(w Workbench) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<a class="wb" href="/w/%s"><div class="t">%s</div>`, w.ID, html.EscapeString(w.Title))
	if w.Desc != "" {
		fmt.Fprintf(&b, `<div class="d">%s</div>`, html.EscapeString(w.Desc))
	} else {
		b.WriteString(`<div class="d"></div>`)
	}
	if len(w.Tags) > 0 {
		b.WriteString(`<div class="tags">`)
		for _, t := range w.Tags {
			fmt.Fprintf(&b, `<span class="tag">%s</span>`, html.EscapeString(t))
		}
		b.WriteString(`</div>`)
	}
	who := w.Teacher
	if who == "" {
		who = "—"
	}
	fmt.Fprintf(&b, `<div class="meta"><span class="avatar" style="background:%s">%s</span>%s<span style="margin-left:auto">%s</span></div></a>`,
		avatarColor(who), avatarChar(who), html.EscapeString(who), time.Unix(w.CreatedAt, 0).Format("01-02"))
	return b.String()
}

// viewerHTML 产物查看页（顶栏 + iframe 包装，不干扰产物自身 DOM）。
func viewerHTML(w Workbench) string {
	title := html.EscapeString(w.Title)
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>` + title + `</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:"PingFang SC","Microsoft YaHei",sans-serif;background:#12333a;display:flex;flex-direction:column;height:100vh}
.bar{display:flex;align-items:center;gap:12px;padding:9px 14px;color:#eef6f6;flex:none}
.bar a{color:#cfe3e5;text-decoration:none;font-size:13.5px;padding:7px 12px;border-radius:9px}
.bar a:hover{background:rgba(255,255,255,.12);color:#fff}
.bar .t{flex:1;font-size:14px;font-weight:700;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
iframe{flex:1;width:100%%;border:0;background:#fff}
</style></head><body>
<div class="bar"><a href="/">← 教研工作台</a><span class="t">` + title + `</span><a href="/w/` + w.ID + `/raw" target="_blank">新窗口打开</a></div>
<iframe src="/w/` + w.ID + `/raw"></iframe>
</body></html>`
}

// avatarChar 头像字（名字首字）。
func avatarChar(name string) string {
	r := []rune(strings.TrimSpace(name))
	if len(r) == 0 {
		return "?"
	}
	return html.EscapeString(string(r[0]))
}

// avatarColor 头像底色（按名字稳定取色）。
func avatarColor(name string) string {
	palette := []string{"#15616d", "#c26a1e", "#4a6fa5", "#8a5a9e", "#2e7d4f", "#b0533f"}
	h := 0
	for _, c := range name {
		h = h*31 + int(c)
	}
	if h < 0 {
		h = -h
	}
	return palette[h%len(palette)]
}
