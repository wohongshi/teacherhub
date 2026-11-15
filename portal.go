package main

// portal.go —— 云端网页（浏览全免 key）：门户首页（统计/产物库/贡献榜/搜索）+ 产物查看页。
// 数据服务端渲染，搜索走开放的只读检索接口；所有用户字符串均 html 转义。

import (
	_ "embed"

	"fmt"
	"html"
	"strings"
	"time"
)

//go:embed web_dashboard.html
var dashboardHTML string

// portalHTML 门户首页（/ 与 /stats 同页）：化学学科园地仪表盘。
// 页面为纯前端 SPA，数据由浏览器 fetch 免 key API（/api/workbenches /api/stats /api/contrib）自取。
func (s *cloudStore) portalHTML() string {
	return dashboardHTML
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
	rateStr := ""
	if w.RateNum > 0 {
		rateStr = fmt.Sprintf("⭐ %.1f · ", rateOf(w))
	}
	fmt.Fprintf(&b, `<div class="meta"><span class="avatar" style="background:%s">%s</span>%s<span style="margin-left:auto">%s👁 浏览 %d · %s</span></div></a>`,
		avatarColor(who), avatarChar(who), html.EscapeString(who), rateStr, w.Views, time.Unix(w.CreatedAt, 0).Format("01-02"))
	return b.String()
}

// viewerHTML 产物查看页（顶栏 + iframe 包装，不干扰产物自身 DOM）。
func viewerHTML(w Workbench) string {
	title := html.EscapeString(w.Title)
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>` + title + `</title>
<style>
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,"PingFang SC","Microsoft YaHei","Noto Sans SC",sans-serif;background:#F7F9F6;background-image:radial-gradient(#E4EAE2 1px,transparent 1px);background-size:22px 22px;display:flex;flex-direction:column;height:100vh}
.bar{display:flex;align-items:center;gap:9px;padding:10px 16px;flex:none;background:rgba(255,255,255,.9);backdrop-filter:blur(10px);-webkit-backdrop-filter:blur(10px);border-bottom:1px solid #EBEFE9}
.bar a{color:#3E4A41;text-decoration:none;font-size:12.5px;padding:8px 13px;border-radius:10px;background:#fff;border:1px solid #EBEFE9;white-space:nowrap}
.bar a:hover{border-color:#8FC79B;color:#2F6B45}
.bar a.primary{background:#3C7D52;color:#fff;border-color:#3C7D52;font-weight:600}
.bar a.primary:hover{background:#2F6B45}
.bar .t{flex:1;font-size:14px;font-weight:700;color:#1F2A22;overflow:hidden;text-overflow:ellipsis;white-space:nowrap}
.ratebar{display:flex;align-items:center;gap:12px;padding:9px 16px;background:#fff;border-bottom:1px solid #EBEFE9;font-size:12.5px;color:#7C8A7F;flex:none}
.ratebar .stars{font-size:17px;color:#E5A93F;letter-spacing:2px;cursor:pointer}
iframe{flex:1;width:100%%;border:0;background:#fff}
@media (prefers-color-scheme:dark){
body{background:#131A15;background-image:radial-gradient(#243027 1px,transparent 1px)}
.bar{background:rgba(27,36,30,.92);border-color:#2A362D}
.bar a{background:#222D25;border-color:#2A362D;color:#C4D2C8}
.bar a:hover{border-color:#4E9A63;color:#8FD3A8}
.bar a.primary{background:#3C7D52;border-color:#3C7D52;color:#fff}
.bar .t{color:#E9F1EB}
.ratebar{background:#1B241E;border-color:#2A362D;color:#8C9C90}
iframe{background:#1B241E}
}
</style></head><body>
<div class="bar"><a href="/">← 学科园地</a><span class="t">` + title + `</span><a class="primary" href="/api/workbenches/` + w.ID + `/file">⬇ 下载</a><a class="primary" href="javascript:void(0)" id="qrbtn">📱 扫码</a><a href="javascript:void(0)" id="sharelink">🔗 复制链接</a><a href="/w/` + w.ID + `/raw" target="_blank">新窗口打开</a></div>
<div class="ratebar"><span id="ratetxt">评分加载中…</span><span class="stars" id="starslot"></span></div>
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
fetch('/api/workbenches/` + w.ID + `').then(function(r){return r.json()}).then(function(j){
  var w2=j.workbench||{};
  var rt=document.getElementById('ratetxt'),sl=document.getElementById('starslot');
  rt.textContent=w2.rate?'⭐ '+w2.rate+' 分（'+(w2.rate_num||0)+' 人评过）':'还没有人评分';
  sl.innerHTML=[1,2,3,4,5].map(function(n){return '<span data-s="'+n+'">☆</span>'}).join('');
  sl.addEventListener('click',async function(e){
    var t=e.target.closest('span[data-s]');if(!t)return;
    var r2=await fetch('/api/workbenches/` + w.ID + `/rate',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({stars:+t.getAttribute('data-s')})});
    var j2=await r2.json();
    if(j2.already){rt.textContent='你已经评过分啦';return}
    if(j2.ok){rt.textContent='⭐ '+j2.rate+' 分 · 感谢评分！';sl.innerHTML=[1,2,3,4,5].map(function(n){return '<span data-s="'+n+'">'+(n<=+t.getAttribute('data-s')?'★':'☆')+'</span>'}).join('')}
  });
});
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
