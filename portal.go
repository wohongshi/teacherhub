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
	wbs := make([]Workbench, 0, len(s.data.Workbenches))
	for _, w2 := range s.data.Workbenches {
		if !w2.Hidden {
			wbs = append(wbs, w2)
		}
	}
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
<link rel="alternate" type="application/rss+xml" title="教研工作台 · 新产物" href="/feed.xml">
<link rel="manifest" href="/manifest.webmanifest">
<link rel="icon" href="/icon.svg" type="image/svg+xml">
<meta name="theme-color" content="#15616d">
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
.stats{display:grid;grid-template-columns:repeat(4,minmax(0,132px));gap:0}
.stats .stat+.stat{border-left:1px solid rgba(255,255,255,.2);padding-left:22px;margin-left:22px}
.stat b.zero{opacity:.42}
.stat b{display:block;font-family:Georgia,"Songti SC",serif;font-size:32px;line-height:1.1}
.stat b{display:block;font-family:Georgia,"Songti SC",serif;font-size:32px;line-height:1.1}
.stat span{font-size:12.5px;opacity:.75;display:block;margin-top:3px;letter-spacing:.04em}
main{padding:22px}
.cols{display:grid;grid-template-columns:minmax(0,1.7fr) minmax(280px,1fr);gap:18px;align-items:start}
.cols>aside{margin-top:14px}
h2{font-size:16px;color:var(--accent);margin:4px 2px 12px;display:flex;align-items:center;gap:8px}
h2 .count{font-size:12px;background:var(--accent-soft);color:var(--accent);border-radius:999px;padding:2px 10px}
.toolbar{display:flex;align-items:center;gap:10px;flex-wrap:wrap;margin:2px 2px 12px}
.fchips{display:flex;gap:8px 10px;flex-wrap:wrap}
.fchip{border:1px solid var(--line);background:#fff;color:var(--muted);border-radius:999px;padding:4px 11px;font-size:11.5px;cursor:pointer;font-family:inherit;line-height:1.5}
.fchip:hover{border-color:#9fc4ca;color:var(--accent)}
.fchip.on{background:var(--accent);border-color:var(--accent);color:#fff;font-weight:700}
.sbtns{margin-left:auto;display:flex;gap:6px}
.sbtn{border:1px solid var(--line);background:#fff;color:var(--muted);border-radius:9px;padding:5px 13px;font-size:12px;cursor:pointer;font-family:inherit}
.sbtn.on{background:var(--accent-soft);border-color:var(--accent);color:var(--accent);font-weight:700}
.wb-grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(260px,1fr));gap:14px}
.wb{display:block;position:relative;background:var(--panel);border:1px solid var(--line);border-radius:14px;padding:16px 17px;text-decoration:none;color:var(--ink);box-shadow:0 2px 8px rgba(34,40,46,.05);transition:all .15s}
.fav{position:absolute;top:10px;right:10px;border:0;background:transparent;font-size:17px;cursor:pointer;filter:grayscale(1);opacity:.4;line-height:1;padding:2px}
.fav:hover{opacity:.9}
.fav.on{filter:none;opacity:1}
.pv{width:100%;height:116px;object-fit:cover;border-radius:10px;display:block;margin-bottom:10px;background:#eef2f3}
.pv-ph{width:100%;height:116px;border-radius:10px;margin-bottom:10px;display:flex;align-items:center;justify-content:center;font-size:30px;color:rgba(255,255,255,.9);background:linear-gradient(135deg,#2b6b74,#4aa8bd)}
.search{position:relative}
#qdrop{position:absolute;top:calc(100% + 6px);left:0;right:0;background:#fff;border:1px solid var(--line);border-radius:12px;box-shadow:0 10px 26px rgba(34,40,46,.14);padding:10px 12px;z-index:30;display:none}
#qdrop .qh{font-size:11px;color:var(--muted);margin:4px 2px 6px}
#qdrop .qitem{display:inline-block;border:1px solid var(--line);border-radius:999px;padding:5px 12px;font-size:12px;color:var(--ink);cursor:pointer;margin:0 6px 6px 0;background:#f7f9f9}
#qdrop .qitem:hover{border-color:var(--accent);color:var(--accent)}
.boardform{display:flex;gap:7px;margin-top:10px;flex-wrap:wrap}
.boardform input{border:1px solid var(--line);border-radius:10px;min-height:38px;padding:0 11px;font-size:12.5px;font-family:inherit}
#bname{width:84px}
#btext{flex:1;min-width:120px}
.boardform button{min-height:38px;padding:0 15px;font-size:12.5px}
.bmsg{font-size:12.5px;line-height:1.7;padding:7px 2px;border-bottom:1px solid #f0ede4}
.bmsg:last-child{border-bottom:0}
.bmsg .bn{color:var(--accent);font-weight:700;margin-right:6px}
.bmsg .bt{color:var(--muted);font-size:11px;float:right}
header.hero{position:relative;overflow:hidden}
header.hero::before{content:"";position:absolute;width:540px;height:540px;right:-150px;top:-230px;border-radius:50%;background:radial-gradient(circle,rgba(255,255,255,.13),transparent 65%);pointer-events:none}
header.hero::after{content:"";position:absolute;width:400px;height:400px;left:-130px;bottom:-210px;border-radius:50%;background:radial-gradient(circle,rgba(255,196,130,.12),transparent 65%);pointer-events:none}
header.hero .wrap{position:relative;z-index:1}
.wb{animation:wbIn .38s ease both}
@keyframes wbIn{from{opacity:0;transform:translateY(12px)}to{opacity:1;transform:none}}
.wb:hover{transform:translateY(-2px);box-shadow:0 8px 20px rgba(34,40,46,.10);border-color:#c9d9dc}
.wb .t{font-size:16px;font-weight:700;line-height:1.4;text-wrap:balance}
.wb .d{font-size:13px;color:var(--muted);margin:6px 0 10px;line-height:1.7;display:-webkit-box;-webkit-line-clamp:3;-webkit-box-orient:vertical;overflow:hidden;min-height:3.7em}
.wb .tags{display:flex;flex-wrap:wrap;gap:6px;margin-bottom:12px}
.tag{font-size:11.5px;background:var(--accent-soft);color:var(--accent);border-radius:999px;padding:3px 10px}
.wb .meta{display:flex;align-items:center;gap:8px;font-size:12px;color:var(--muted);margin-top:14px;padding-top:12px;border-top:1px solid #eef2f3}
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
.empty{color:var(--muted);font-size:13.5px;padding:15px 10px;text-align:center;display:flex;flex-direction:column;align-items:center;gap:5px}
.empty::before{content:"📭";font-size:22px;opacity:.7}
footer{text-align:center;color:var(--muted);font-size:12px;padding:22px}
@media (max-width:900px){
  .cols{grid-template-columns:1fr}
  h1{font-size:23px}
  .hero{padding:24px 16px}
  main{padding:14px}
  .stat span{font-size:12px}
  .fchip{min-height:44px;padding:10px 15px;font-size:13px}
  .fontbtn,.themebtn{width:40px;height:40px;font-size:13px;opacity:.92}
  .wrap{padding-right:52px}
  .stat span{margin-top:5px}
  .sbtns{margin-left:0;width:100%;justify-content:flex-end}
  .sbtn{min-height:38px;font-size:13px}
  .wb .meta{font-size:12px}
  .cols>aside{margin-top:0}
}
html[data-theme=dark]{--paper:#12161a;--panel:#1c2229;--ink:#e7ecef;--muted:#95a1a8;--line:#2b333b;--accent:#5db4c2;--accent-soft:#22343c;--amber:#e0954f}
html[data-theme=dark] body{background:var(--paper);color:var(--ink)}
html[data-theme=dark] .hero{background:linear-gradient(135deg,#0c272d,#16454e)}
html[data-theme=dark] .card,html[data-theme=dark] .wb{background:var(--panel);border-color:var(--line)}
html[data-theme=dark] .wb:hover{border-color:#3d4b54;box-shadow:0 8px 20px rgba(0,0,0,.35)}
html[data-theme=dark] .fav{filter:grayscale(1) brightness(2.2);opacity:.5}
html[data-theme=dark] .fav.on{filter:none;opacity:1}
html[data-theme=dark] #qdrop{background:var(--panel);border-color:var(--line)}
html[data-theme=dark] .fchip,html[data-theme=dark] .sbtn,html[data-theme=dark] .boardform input,html[data-theme=dark] .boardform button,html[data-theme=dark] .qitem{background:#222a32;border-color:#2e3740;color:#c4ced4}
html[data-theme=dark] .fchip.on,html[data-theme=dark] .sbtn.on{background:var(--accent);border-color:var(--accent);color:#fff}
html[data-theme=dark] .search input{background:#222a32;color:var(--ink)}
html[data-theme=dark] .search button{background:#2e3740;color:#d7e2e6}
html[data-theme=dark] .tag,html[data-theme=dark] .trow .cnt{background:var(--accent-soft);color:#8fc7d1}
html[data-theme=dark] .bmsg{border-color:#2b333b}
html[data-theme=dark] .pv{background:#222a32}
html[data-theme=dark] .stat b{color:#fff}
#lightbox{position:fixed;inset:0;background:rgba(10,14,18,.82);display:none;align-items:center;justify-content:center;z-index:99;cursor:zoom-out}
#lightbox img{max-width:88%;max-height:84%;border-radius:14px;box-shadow:0 18px 60px rgba(0,0,0,.5)}
#toast{position:fixed;left:50%;bottom:28px;transform:translateX(-50%) translateY(20px);background:rgba(28,40,44,.92);color:#fff;padding:10px 20px;border-radius:12px;font-size:13px;opacity:0;transition:all .25s;z-index:9999;pointer-events:none}
#toast.show{opacity:1;transform:translateX(-50%)}
.new{display:inline-block;vertical-align:middle;background:#d64541;color:#fff;font-size:10px;font-weight:800;border-radius:8px;padding:2px 7px;letter-spacing:.05em;margin-right:8px;position:static}
.wb-grid.compact .pv,.wb-grid.compact .pv-ph{display:none}
.wb-grid.compact{gap:10px}
.wb-grid.compact .wb .d{-webkit-line-clamp:2;min-height:2.5em}
.recentbar{display:flex;align-items:center;gap:8px;flex-wrap:wrap;margin:0 2px 12px;font-size:12px;color:var(--muted)}
.recentbar a{color:var(--accent);background:var(--accent-soft);border-radius:999px;padding:4px 11px;text-decoration:none;font-size:11.5px}
.recentbar a:hover{text-decoration:underline}
.noticebar{background:var(--amber);color:#fff;text-align:center;padding:9px 16px;font-size:13px;font-weight:600}
html[data-theme=dark] .noticebar{background:#8a5a22}
.like{text-decoration:none;color:var(--accent);margin-left:8px;font-size:11px;cursor:pointer}
.like:hover{text-decoration:underline}
.fontbtn{position:fixed;z-index:60;width:46px;height:46px;border-radius:50%;font-size:15px;font-weight:700;box-shadow:0 6px 18px rgba(0,0,0,.25);padding:0;cursor:pointer;display:flex;align-items:center;justify-content:center;line-height:1}
#fontminus{right:16px;bottom:72px}
#fontplus{right:16px;bottom:128px}
.totop{position:fixed;left:16px;bottom:16px;z-index:60;width:46px;height:46px;border-radius:50%;font-size:19px;box-shadow:0 6px 18px rgba(0,0,0,.25);padding:0;cursor:pointer;opacity:0;pointer-events:none;transition:opacity .25s}
.totop.show{opacity:1;pointer-events:auto}
.helpov{position:fixed;inset:0;background:rgba(10,14,18,.55);display:none;align-items:center;justify-content:center;z-index:98;cursor:pointer}
.helpov .hbox{background:var(--panel);border-radius:16px;padding:22px 26px;max-width:340px;line-height:2.1;font-size:13.5px;color:var(--ink);box-shadow:0 18px 50px rgba(0,0,0,.35)}
.helpov kbd{background:var(--accent-soft);color:var(--accent);border-radius:6px;padding:2px 9px;font-size:12px;font-family:inherit;font-weight:700}
.themebtn{position:fixed;right:16px;bottom:16px;z-index:60;width:46px;height:46px;border-radius:50%;font-size:20px;box-shadow:0 6px 18px rgba(0,0,0,.25);padding:0;cursor:pointer}
</style></head><body>
<div id="toast"></div>
<button id="fontminus" class="fontbtn" title="缩小字号">A−</button><button id="fontplus" class="fontbtn" title="放大字号">A+</button>
<button id="themebtn" class="themebtn" title="深色/浅色切换">🌙</button>
<header class="hero"><div class="wrap">
  <div class="brand" style="display:flex;align-items:center"><span class="logo">📚</span><span>教研工作台 · 教研组共享</span><button id="rndbtn" style="margin-left:auto;background:rgba(255,255,255,.14);border:1px solid rgba(255,255,255,.28);color:#fff">🎲 随机看看</button></div>
  <h1>教研成果，一处共享</h1>
  <div class="sub">每位老师贡献的资料、做出来的实验工作台，都在这里沉淀复用——先搜一下，别重复造。</div>
  <div class="search"><input id="q" placeholder="搜工作台：如「钠与水」「铁丝燃烧」"><button id="qbtn">搜索</button></div>
  <div class="stats">
    <div class="stat"><b` + zeroCls(len(rows)) + `>` + fmt.Sprint(len(rows)) + `</b><span>参与教师</span></div>
    <div class="stat"><b` + zeroCls(len(contribs)) + `>` + fmt.Sprint(len(contribs)) + `</b><span>贡献记录</span></div>
    <div class="stat"><b` + zeroCls(len(wbs)) + `>` + fmt.Sprint(len(wbs)) + `</b><span>工作台产物</span></div>
    <div class="stat"><b` + zeroCls(s.data.DailyViews[time.Now().Format("2006-01-02")]) + `>` + fmt.Sprint(s.data.DailyViews[time.Now().Format("2006-01-02")]) + `</b><span>今日浏览</span></div>
  </div>
</div></header>
<div class="noticebar" id="noticebar" style="display:none">📢 <span id="noticetext"></span></div>
<main><div class="wrap"><div class="cols">
  <section>
    <h2>工作台产物库 <span class="count" id="wb-count">` + fmt.Sprint(len(wbs)) + `</span><div class="sbtns" id="sortbtns"><button class="sbtn on" data-sort="new">最新</button><button class="sbtn" data-sort="hot">最热</button><button class="sbtn" id="layoutbtn">▦ 紧凑</button></div></h2>`)
	// 标签筛选 + 排序工具条
	tagSet := map[string]bool{}
	for _, w2 := range wbs {
		for _, t := range w2.Tags {
			if t != "" {
				tagSet[t] = true
			}
		}
	}
	tagList := make([]string, 0, len(tagSet))
	for t := range tagSet {
		tagList = append(tagList, t)
	}
	sort.Strings(tagList)
	b.WriteString(`<div class="recentbar" id="recentbar" style="display:none"><span>🕘 最近看过</span></div>
<div class="toolbar"><div class="fchips" id="tagfilter"><button class="fchip on" data-tag="">全部</button><button class="fchip" data-tag="__fav__">⭐ 我的收藏</button>`)
	for _, t := range tagList {
		fmt.Fprintf(&b, `<button class="fchip" data-tag="%s">%s</button>`, html.EscapeString(t), html.EscapeString(t))
	}
	b.WriteString(`</div></div>
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
    <div class="card" id="boardcard">
      <h2>💬 留言墙</h2>
      <div id="boardlist"><div class="empty">还没有留言，来说两句</div></div>
      <div class="boardform"><input id="bname" placeholder="昵称" maxlength="16"><input id="btext" placeholder="说点什么（200 字内）" maxlength="200"><button id="bsend">发送</button></div>
    </div>
  </aside>
</div></div></main>
<button id="totop" class="totop" title="回到顶部">↑</button>
<div class="helpov" id="helpov"><div class="hbox">
<b>⌨ 快捷键</b><br>
<kbd>/</kbd> 聚焦搜索 · <kbd>D</kbd> 深色切换<br>
<kbd>T</kbd> 回到顶部 · <kbd>?</kbd> 帮助开关<br>
（点任意处关闭）
</div></div>
<div id="lightbox"><img alt=""></div>
<footer>教研工作台 · 零账号 · 浏览免 key · 写入走统一 key · <a href="/feed.xml" style="color:inherit">RSS 订阅</a></footer>
<script>
(function(){
  var q=document.getElementById('q'),btn=document.getElementById('qbtn'),grid=document.getElementById('grid'),cnt=document.getElementById('wb-count');
  function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,function(c){return({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]})}
  function card(w){
    var tags=(w.tags||[]).map(function(t){return '<span class="tag">'+esc(t)+'</span>'}).join('');
    var ch=esc((w.teacher||'?').slice(0,1));
    var nw=(Date.now()/1000-(w.created_at||0)<604800)?'<span class="new">NEW</span>':'';
    var pv=w.preview?'<img class="pv" src="'+esc(w.preview)+'" alt="">':'<div class="pv-ph">⚗</div>';
    return '<a class="wb in" href="/w/'+encodeURIComponent(w.id)+'" data-tags="'+esc((w.tags||[]).join(','))+'" data-views="'+(w.views||0)+'" data-created="'+(w.created_at||0)+'">'+nw+'<button class="fav" data-id="'+esc(w.id)+'" title="收藏（存本机浏览器）">⭐</button><div class="t">'+esc(w.title)+'</div>'+pv+
      '<div class="d">'+esc(w.desc||'')+'</div>'+
      (tags?'<div class="tags">'+tags+'</div>':'')+
      '<div class="meta"><span class="avatar" style="background:#15616d">'+ch+'</span>'+esc(w.teacher||'—')+'<span style="margin-left:auto">👁 浏览 '+(w.views||0)+'</span></div></a>';
  }
  var FAV_KEY='th_favs';
  function favs(){try{return JSON.parse(localStorage.getItem(FAV_KEY)||'[]')}catch(e){return []}}
  function toggleFav(id){var f=favs(),i=f.indexOf(id);if(i>=0)f.splice(i,1);else f.push(id);localStorage.setItem(FAV_KEY,JSON.stringify(f));return i<0}
  function syncFavs(){var f=favs();grid.querySelectorAll('.fav').forEach(function(b){b.classList.toggle('on',f.indexOf(b.getAttribute('data-id'))>=0)})}
  grid.addEventListener('click',function(e){
    var tg=e.target.closest('.tag');
    if(tg){
      e.preventDefault();e.stopPropagation();
      var name=tg.textContent.trim();
      var chip=null;
      [].slice.call(document.querySelectorAll('.fchip')).forEach(function(c){if(c.textContent.trim()===name)chip=c});
      if(chip)chip.click();
      return;
    }
    var b=e.target.closest('.fav');if(!b)return;
    e.preventDefault();e.stopPropagation();
    var on=toggleFav(b.getAttribute('data-id'));
    b.classList.toggle('on',on);
    if(curTag==='__fav__')applyFilter();
  });
  var curTag='',curSort='new';
  function applyFilter(){
    var cards=[].slice.call(grid.querySelectorAll('.wb'));
    var f=favs();
    cards.forEach(function(c){
      var ok;
      if(curTag==='__fav__'){
        var fb=c.querySelector('.fav');
        ok=fb&&f.indexOf(fb.getAttribute('data-id'))>=0;
      }else{
        var tags=(c.getAttribute('data-tags')||'').split(',');
        ok=(!curTag||tags.indexOf(curTag)>=0);
      }
      c.style.display=ok?'':'none';
    });
    cards.sort(function(a,b){
      return curSort==='hot'
        ?(+b.getAttribute('data-views'))-(+a.getAttribute('data-views'))
        :(+b.getAttribute('data-created'))-(+a.getAttribute('data-created'));
    });
    cards.forEach(function(c){grid.appendChild(c)});
    cnt.textContent=cards.filter(function(c){return c.style.display!=='none'}).length;
  }
  var tf=document.getElementById('tagfilter');
  if(tf)tf.addEventListener('click',function(e){
    var b=e.target.closest('.fchip');if(!b)return;
    tf.querySelectorAll('.fchip').forEach(function(x){x.classList.remove('on')});
    b.classList.add('on');curTag=b.getAttribute('data-tag')||'';applyFilter();
  });
  var sb=document.querySelector('.sbtns');
  if(sb)sb.addEventListener('click',function(e){
    var b=e.target.closest('.sbtn');if(!b)return;
    sb.querySelectorAll('.sbtn').forEach(function(x){x.classList.remove('on')});
    b.classList.add('on');curSort=b.getAttribute('data-sort')||'new';applyFilter();
  });
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
      pushHist(v);
    }catch(e){grid.innerHTML='<div class="empty">搜索失败，稍后再试</div>'}
    btn.disabled=false;btn.textContent='搜索';
  }
  /* 搜索历史 + 热门标签 */
  var HKEY='th_hists';
  function hists(){try{return JSON.parse(localStorage.getItem(HKEY)||'[]')}catch(e){return []}}
  function pushHist(v){var h=hists().filter(function(x){return x!==v});h.unshift(v);localStorage.setItem(HKEY,JSON.stringify(h.slice(0,5)))}
  var drop=document.createElement('div');drop.id='qdrop';q.parentNode.appendChild(drop);
  function renderDrop(){
    var h=hists();
    var hot=[].slice.call(document.querySelectorAll('.fchip')).map(function(c){return c.textContent.trim()}).filter(function(t){return t!=='全部'&&t.indexOf('我的收藏')<0}).slice(0,6);
    drop.innerHTML=(h.length?'<div class="qh">最近搜过</div>'+h.map(function(t){return '<span class="qitem">'+esc(t)+'</span>'}).join(''):'')+'<div class="qh">热门标签</div>'+hot.map(function(t){return '<span class="qitem">'+esc(t)+'</span>'}).join('');
  }
  q.addEventListener('focus',function(){renderDrop();drop.style.display='block'});
  drop.addEventListener('mousedown',function(e){
    var it=e.target.closest('.qitem');if(!it)return;
    e.preventDefault();q.value=it.textContent;drop.style.display='none';search();
  });
  document.addEventListener('click',function(e){if(!e.target.closest('.search'))drop.style.display='none'});
  if('serviceWorker' in navigator)navigator.serviceWorker.register('/sw.js').catch(function(){});
/* 长按看大图（手机）+ 最近看过足迹 */
(function(){
  var lp=null;
  grid.addEventListener('touchstart',function(e){
    var img=e.target.closest('img.pv');if(!img)return;
    lp=setTimeout(function(){
      var lb=document.getElementById('lightbox');lb.querySelector('img').src=img.src;lb.style.display='flex';
    },450);
  },{passive:true});
  grid.addEventListener('touchend',function(){clearTimeout(lp)});
  grid.addEventListener('touchmove',function(){clearTimeout(lp)},{passive:true});
  var RKEY='th_recent';
  try{
    var rec=JSON.parse(localStorage.getItem(RKEY)||'[]');
    if(rec.length){
      var bar=document.getElementById('recentbar');
      bar.style.display='flex';
      rec.slice(0,5).forEach(function(x){
        var a=document.createElement('a');a.href='/w/'+encodeURIComponent(x.id);a.textContent=x.title.slice(0,12);
        bar.appendChild(a);
      });
    }
  }catch(e){}
})();
/* 预览图大图 */
grid.addEventListener('click',function(e){
  var img=e.target.closest('img.pv');if(!img)return;
  e.preventDefault();e.stopPropagation();
  var lb=document.getElementById('lightbox');lb.querySelector('img').src=img.src;lb.style.display='flex';
});
document.getElementById('lightbox').addEventListener('click',function(){this.style.display='none'});
/* 字号调节 */
(function(){var K='th_zoom',zs=[0.85,1,1.15,1.3],zi=1;
function ap(){document.body.style.zoom=zs[zi];try{localStorage.setItem(K,String(zi))}catch(e){}}
try{var s=localStorage.getItem(K);if(s!==null)zi=Math.min(zs.length-1,Math.max(0,parseInt(s,10)||1))}catch(e){}
ap();
document.getElementById('fontminus').addEventListener('click',function(){if(zi>0){zi--;ap()}});
document.getElementById('fontplus').addEventListener('click',function(){if(zi<zs.length-1){zi++;ap()}});
})();
/* 深色模式 */
(function(){var K='th_theme';
function ap(t){document.documentElement.setAttribute('data-theme',t);var b=document.getElementById('themebtn');if(b)b.textContent=t==='dark'?'\u2600\uFE0F':'\uD83C\uDF19'}
var s=null;try{s=localStorage.getItem(K)}catch(e){}
ap(s||(window.matchMedia&&matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light'));
document.getElementById('themebtn').addEventListener('click',function(){var t=document.documentElement.getAttribute('data-theme')==='dark'?'light':'dark';try{localStorage.setItem(K,t)}catch(e){}ap(t)});
})();
/* 随机看看 */
document.getElementById('rndbtn').addEventListener('click',function(){
  var links=[].slice.call(document.querySelectorAll('a.wb')).map(function(a){return a.getAttribute('href')});
  if(links.length)location.href=links[Math.floor(Math.random()*links.length)];
});
/* 回顶 + 快捷键帮助 */
(function(){
  var tt=document.getElementById('totop');
  window.addEventListener('scroll',function(){tt.classList.toggle('show',window.scrollY>400)},{passive:true});
  setInterval(function(){tt.classList.toggle('show',window.scrollY>400)},300);
  tt.addEventListener('click',function(){window.scrollTo({top:0,behavior:'smooth'})});
  var hv=document.getElementById('helpov');
  hv.addEventListener('click',function(){this.style.display='none'});
  document.addEventListener('keydown',function(e){
    if(e.target&&e.target.tagName==='INPUT')return;
    if(e.key==='?'){e.preventDefault();hv.style.display=hv.style.display==='flex'?'none':'flex'}
    else if(e.key==='/'){e.preventDefault();document.getElementById('q').focus()}
    else if(e.key==='d'||e.key==='D'){document.getElementById('themebtn').click()}
    else if(e.key==='t'||e.key==='T'){window.scrollTo({top:0,behavior:'smooth'})}
  });
})();
/* toast 轻提示 */
function toast(msg){var t=document.getElementById('toast');t.textContent=msg;t.classList.add('show');clearTimeout(t._tm);t._tm=setTimeout(function(){t.classList.remove('show')},1800)}
/* 布局切换 */
(function(){
  var LKEY='th_layout';
  function applyLayout(l){
    grid.classList.toggle('compact',l==='compact');
    document.getElementById('layoutbtn').textContent=l==='compact'?'▦ 大图':'▦ 紧凑';
    try{localStorage.setItem(LKEY,l)}catch(e){}
  }
  applyLayout((function(){try{return localStorage.getItem(LKEY)||'big'}catch(e){return 'big'}})());
  document.getElementById('layoutbtn').addEventListener('click',function(){applyLayout(grid.classList.contains('compact')?'big':'compact')});
})();
/* 数字滚动 */
  document.querySelectorAll('.stat b').forEach(function(b){
    var target=parseInt(b.textContent,10)||0,t0=null;
    function step(ts){if(!t0)t0=ts;var p=Math.min((ts-t0)/700,1);b.textContent=Math.round(target*(1-Math.pow(1-p,3)));if(p<1)requestAnimationFrame(step)}
    requestAnimationFrame(step);
  });
  /* 留言墙 */
  var bl=document.getElementById('boardlist');
  function timeAgo(t){var d=Date.now()/1000-t;if(d<60)return '刚刚';if(d<3600)return Math.floor(d/60)+' 分钟前';if(d<86400)return Math.floor(d/3600)+' 小时前';return Math.floor(d/86400)+' 天前'}
  async function loadBoard(){
    try{
      var r=await fetch('/api/board');var j=await r.json();var list=j.board||[];
      bl.innerHTML=list.length?list.map(function(m){return '<div class="bmsg"><span class="bn">'+esc(m.name)+'</span>'+esc(m.text)+'<span class="bt">'+timeAgo(m.at)+(m.id?' <a class="like" data-id="'+esc(m.id)+'" href="javascript:void(0)">👍 '+(m.likes||0)+'</a>':'')+'</span></div>'}).join(''):'<div class="empty">还没有留言，来说两句</div>';
    }catch(e){}
  }
  document.getElementById('bsend').addEventListener('click',async function(){
    var n=document.getElementById('bname').value.trim(),t=document.getElementById('btext').value.trim();
    if(!n||!t){toast('填下昵称和内容');return}
    try{
      var r=await fetch('/api/board',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:n,text:t})});
      var j=await r.json();
      if(!j.ok){toast(j.error||'发送失败');return}
      document.getElementById('btext').value='';toast('留言已发送');loadBoard();
    }catch(e){toast('网络异常，稍后再试')}
  });
  loadBoard();
  bl.addEventListener('click',async function(e){
    var a=e.target.closest('.like');if(!a)return;
    e.preventDefault();
    try{var r=await fetch('/api/board/like',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id:a.getAttribute('data-id')})});var j=await r.json();if(j.ok)loadBoard()}catch(err){}
  });
  (async function(){try{var r=await fetch('/api/notice');var j=await r.json();if(j.notice){document.getElementById('noticetext').textContent=j.notice;document.getElementById('noticebar').style.display='block'}}catch(e){}})();
  btn.addEventListener('click',search);
  q.addEventListener('keydown',function(e){if(e.key==='Enter')search()});
  syncFavs();
})();
</script></body></html>`)
	return b.String()
}

// wbCardHTML 产物卡片。
func wbCardHTML(w Workbench) string {
	var b strings.Builder
	fmt.Fprintf(&b, `<a class="wb" href="/w/%s" data-tags="%s" data-views="%d" data-created="%d"><button class="fav" data-id="%s" title="收藏（存本机浏览器）">⭐</button><div class="t">%s</div>`,
		w.ID, html.EscapeString(strings.Join(w.Tags, ",")), w.Views, w.CreatedAt, w.ID, html.EscapeString(w.Title))
	if time.Now().Unix()-w.CreatedAt < 7*86400 {
		b.WriteString(`<span class="new">NEW</span>`)
	}
	if w.HasPreview {
		fmt.Fprintf(&b, `<img class="pv" src="/api/workbenches/%s/preview" alt="">`, w.ID)
	} else {
		b.WriteString(`<div class="pv-ph">⚗</div>`)
	}
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
	fmt.Fprintf(&b, `<div class="meta"><span class="avatar" style="background:%s">%s</span>%s<span style="margin-left:auto">👁 浏览 %d · %s</span></div></a>`,
		avatarColor(who), avatarChar(who), html.EscapeString(who), w.Views, time.Unix(w.CreatedAt, 0).Format("01-02"))
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
<div class="bar"><a href="/">← 教研工作台</a><span class="t">` + title + `</span><a href="/api/workbenches/` + w.ID + `/file">⬇ 下载</a><a href="javascript:void(0)" id="qrbtn">📱 扫码</a><a href="javascript:void(0)" id="sharelink">🔗 复制链接</a><a href="/w/` + w.ID + `/raw" target="_blank">新窗口打开</a></div>
<iframe src="/w/` + w.ID + `/raw"></iframe>
<script src="/qr.js"></script>
<div class="lightbox" id="qrov" style="display:none;position:fixed;inset:0;background:rgba(10,14,18,.78);align-items:center;justify-content:center;z-index:99;cursor:zoom-out">
  <div style="background:#fff;border-radius:16px;padding:20px 24px;text-align:center">
    <canvas id="qrcv"></canvas>
    <div style="font-size:12.5px;color:#556;margin-top:10px">手机扫码直达本页</div>
  </div>
</div>
<script>
document.getElementById('qrbtn').addEventListener('click',function(){
  var ov=document.getElementById('qrov');
  if(ov.style.display!=='flex'){qrRender(document.getElementById('qrcv'),location.href);ov.style.display='flex'}
  else ov.style.display='none';
});
document.getElementById('qrov').addEventListener('click',function(){this.style.display='none'});

try{
  var RKEY='th_recent';
  var rec=JSON.parse(localStorage.getItem(RKEY)||'[]').filter(function(x){return x.id!=='` + w.ID + `'});
  rec.unshift({id:'` + w.ID + `',title:'` + title + `'.slice(0,16)});
  localStorage.setItem(RKEY,JSON.stringify(rec.slice(0,5)));
}catch(e){}
document.getElementById('sharelink').addEventListener('click',async function(){
  var btn=this,url=location.href;
  if(navigator.share){try{await navigator.share({title:document.title,url:url});return}catch(e){}}
  try{await navigator.clipboard.writeText(url);btn.textContent='已复制 ✓';setTimeout(function(){btn.textContent='🔗 复制链接'},1500)}catch(e){prompt('复制链接：',url)}
});
</script>
</body></html>`
}

// zeroCls 统计值为 0 时降级显示。
func zeroCls(n int) string {
	if n == 0 {
		return ` class="zero"`
	}
	return ""
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
