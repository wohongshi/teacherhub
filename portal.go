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
<link rel="alternate" type="application/rss+xml" title="教研工作台 · 新产物" href="/feed.xml">
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
.stats{display:grid;grid-template-columns:repeat(3,minmax(0,152px));gap:20px}
.stat b{display:block;font-family:Georgia,"Songti SC",serif;font-size:32px;line-height:1.1}
.stat span{font-size:12.5px;opacity:.75;display:block;margin-top:3px;letter-spacing:.04em}
main{padding:22px}
.cols{display:grid;grid-template-columns:minmax(0,1.7fr) minmax(280px,1fr);gap:18px;align-items:start}
.cols>aside{margin-top:14px}
h2{font-size:16px;color:var(--accent);margin:4px 2px 12px;display:flex;align-items:center;gap:8px}
h2 .count{font-size:12px;background:var(--accent-soft);color:var(--accent);border-radius:999px;padding:2px 10px}
.toolbar{display:flex;align-items:center;gap:10px;flex-wrap:wrap;margin:2px 2px 12px}
.fchips{display:flex;gap:6px 8px;flex-wrap:wrap}
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
  .fchip{min-height:38px;padding:8px 14px;font-size:13px}
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
.noticebar{background:var(--amber);color:#fff;text-align:center;padding:9px 16px;font-size:13px;font-weight:600}
html[data-theme=dark] .noticebar{background:#8a5a22}
.like{text-decoration:none;color:var(--accent);margin-left:8px;font-size:11px;cursor:pointer}
.like:hover{text-decoration:underline}
.themebtn{position:fixed;right:16px;bottom:16px;z-index:60;width:46px;height:46px;border-radius:50%;font-size:20px;box-shadow:0 6px 18px rgba(0,0,0,.25);padding:0;cursor:pointer}
</style></head><body>
<button id="themebtn" class="themebtn" title="深色/浅色切换">🌙</button>
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
<div class="noticebar" id="noticebar" style="display:none">📢 <span id="noticetext"></span></div>
<main><div class="wrap"><div class="cols">
  <section>
    <h2>工作台产物库 <span class="count" id="wb-count">` + fmt.Sprint(len(wbs)) + `</span><div class="sbtns" id="sortbtns"><button class="sbtn on" data-sort="new">最新</button><button class="sbtn" data-sort="hot">最热</button></div></h2>`)
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
	b.WriteString(`<div class="toolbar"><div class="fchips" id="tagfilter"><button class="fchip on" data-tag="">全部</button><button class="fchip" data-tag="__fav__">⭐ 我的收藏</button>`)
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
<div id="lightbox"><img alt=""></div>
<footer>教研工作台 · 零账号 · 浏览免 key · 写入走统一 key · <a href="/feed.xml" style="color:inherit">RSS 订阅</a></footer>
<script>
(function(){
  var q=document.getElementById('q'),btn=document.getElementById('qbtn'),grid=document.getElementById('grid'),cnt=document.getElementById('wb-count');
  function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,function(c){return({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]})}
  function card(w){
    var tags=(w.tags||[]).map(function(t){return '<span class="tag">'+esc(t)+'</span>'}).join('');
    var ch=esc((w.teacher||'?').slice(0,1));
    var pv=w.preview?'<img class="pv" src="'+esc(w.preview)+'" alt="">':'<div class="pv-ph">⚗</div>';
    return '<a class="wb" href="/w/'+encodeURIComponent(w.id)+'" data-tags="'+esc((w.tags||[]).join(','))+'" data-views="'+(w.views||0)+'" data-created="'+(w.created_at||0)+'"><button class="fav" data-id="'+esc(w.id)+'" title="收藏（存本机浏览器）">⭐</button><div class="t">'+esc(w.title)+'</div>'+pv+
      '<div class="d">'+esc(w.desc||'')+'</div>'+
      (tags?'<div class="tags">'+tags+'</div>':'')+
      '<div class="meta"><span class="avatar" style="background:#15616d">'+ch+'</span>'+esc(w.teacher||'—')+'<span style="margin-left:auto">👁 浏览 '+(w.views||0)+'</span></div></a>';
  }
  var FAV_KEY='th_favs';
  function favs(){try{return JSON.parse(localStorage.getItem(FAV_KEY)||'[]')}catch(e){return []}}
  function toggleFav(id){var f=favs(),i=f.indexOf(id);if(i>=0)f.splice(i,1);else f.push(id);localStorage.setItem(FAV_KEY,JSON.stringify(f));return i<0}
  function syncFavs(){var f=favs();grid.querySelectorAll('.fav').forEach(function(b){b.classList.toggle('on',f.indexOf(b.getAttribute('data-id'))>=0)})}
  grid.addEventListener('click',function(e){
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
  /* 预览图大图 */
grid.addEventListener('click',function(e){
  var img=e.target.closest('img.pv');if(!img)return;
  e.preventDefault();e.stopPropagation();
  var lb=document.getElementById('lightbox');lb.querySelector('img').src=img.src;lb.style.display='flex';
});
document.getElementById('lightbox').addEventListener('click',function(){this.style.display='none'});
/* 深色模式 */
(function(){var K='th_theme';
function ap(t){document.documentElement.setAttribute('data-theme',t);var b=document.getElementById('themebtn');if(b)b.textContent=t==='dark'?'\u2600\uFE0F':'\uD83C\uDF19'}
var s=null;try{s=localStorage.getItem(K)}catch(e){}
ap(s||(window.matchMedia&&matchMedia('(prefers-color-scheme: dark)').matches?'dark':'light'));
document.getElementById('themebtn').addEventListener('click',function(){var t=document.documentElement.getAttribute('data-theme')==='dark'?'light':'dark';try{localStorage.setItem(K,t)}catch(e){}ap(t)});
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
      bl.innerHTML=list.length?list.map(function(m){return '<div class="bmsg"><span class="bn">'+esc(m.name)+'</span>'+esc(m.text)+'<span class="bt">'+timeAgo(m.at)+' <a class="like" data-id="'+esc(m.id)+'" href="javascript:void(0)">👍 '+(m.likes||0)+'</a></span></div>'}).join(''):'<div class="empty">还没有留言，来说两句</div>';
    }catch(e){}
  }
  document.getElementById('bsend').addEventListener('click',async function(){
    var n=document.getElementById('bname').value.trim(),t=document.getElementById('btext').value.trim();
    if(!n||!t){alert('填下昵称和内容');return}
    try{
      var r=await fetch('/api/board',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({name:n,text:t})});
      var j=await r.json();
      if(!j.ok){alert(j.error||'发送失败');return}
      document.getElementById('btext').value='';loadBoard();
    }catch(e){alert('网络异常，稍后再试')}
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
<div class="bar"><a href="/">← 教研工作台</a><span class="t">` + title + `</span><a href="/api/workbenches/` + w.ID + `/file">⬇ 下载</a><a href="/w/` + w.ID + `/raw" target="_blank">新窗口打开</a></div>
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
