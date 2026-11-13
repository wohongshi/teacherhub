package main

// /admin 管理面板：统一 key 进入（localStorage 记住），一入口管统计/公告/留言/产物。
// 全部走现有 API：ping 验 key、GET/POST/DELETE 复用，不新增数据面。

import "net/http"

func adminHTML() string {
	return `<!doctype html><html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1"><title>教研工作台 · 管理</title>
<style>
:root{--paper:#f4f2ec;--panel:#fff;--ink:#22282e;--muted:#6b7280;--line:#e3dfd4;--accent:#15616d;--accent-soft:#e2eef0;--warn:#b3261e;--warn-soft:#fbe4e0}
*{box-sizing:border-box;margin:0;padding:0}
body{background:var(--paper);color:var(--ink);font-family:"PingFang SC","Microsoft YaHei",sans-serif;min-height:100vh}
header{background:linear-gradient(135deg,#123c44,#15616d);color:#fff;padding:16px 22px;display:flex;align-items:center;gap:14px}
header h1{font-size:18px;flex:1}
header a{color:#cfe3e5;text-decoration:none;font-size:13px;padding:7px 12px;border-radius:9px}
header a:hover{background:rgba(255,255,255,.12);color:#fff}
main{max-width:1060px;margin:18px auto;padding:0 16px;display:grid;grid-template-columns:1fr 1fr;gap:16px;align-items:start}
.card{background:var(--panel);border:1px solid var(--line);border-radius:16px;padding:16px 18px;margin-bottom:16px}
.card h2{font-size:15px;color:var(--accent);margin-bottom:11px}
#toast{position:fixed;left:50%;bottom:28px;transform:translateX(-50%) translateY(20px);background:rgba(28,40,44,.92);color:#fff;padding:10px 20px;border-radius:12px;font-size:13px;opacity:0;transition:all .25s;z-index:9999;pointer-events:none}
#toast.show{opacity:1;transform:translateX(-50%)}
.stats{display:grid;grid-template-columns:repeat(3,1fr);gap:10px}
.stats .s{background:var(--accent-soft);border-radius:11px;padding:10px 12px}
.stats .s b{display:block;font-size:21px;color:var(--accent)}
.stats .s span{font-size:11.5px;color:var(--muted)}
textarea,input{width:100%;border:1px solid var(--line);border-radius:10px;padding:9px 11px;font-size:13px;font-family:inherit}
textarea{min-height:62px;resize:vertical}
button{border:1px solid var(--line);background:#fff;color:var(--ink);border-radius:10px;min-height:38px;padding:0 15px;font-size:13px;cursor:pointer;font-family:inherit}
button.primary{background:var(--accent);border-color:var(--accent);color:#fff;font-weight:700}
button.danger{color:var(--warn);border-color:#e8b7ab}
button.danger:hover{background:var(--warn-soft)}
.row{display:flex;gap:8px;margin-top:9px;flex-wrap:wrap}
.item{padding:8px 2px;border-bottom:1px solid #f0ede4;font-size:12.5px;line-height:1.7;display:flex;gap:8px;align-items:center}
.item:last-child{border-bottom:0}
.item .tx{flex:1}
.item .tm{color:var(--muted);font-size:11px;white-space:nowrap}
.wbrow{display:flex;gap:8px;align-items:center;padding:7px 2px;border-bottom:1px solid #f0ede4;font-size:12.5px}
.wbrow:last-child{border-bottom:0}
.wbrow a{color:var(--accent);text-decoration:none;font-weight:600;flex:1}
.wbrow .n{color:var(--muted);font-size:11px;white-space:nowrap}
#login{max-width:380px;margin:110px auto}
.trend{width:100%;height:88px;background:var(--accent-soft);border-radius:10px;margin-top:12px}
@media(max-width:860px){main{grid-template-columns:1fr}}
</style></head><body>
<div id="toast"></div>
<header><h1>🛠 教研工作台 · 管理面板</h1><a href="/">← 回门户</a></header>
<div id="login" class="card">
  <h2>统一访问 key</h2>
  <input id="keyin" type="password" placeholder="输入管理 key">
  <div class="row"><button class="primary" id="keygo">进入</button><button id="keyout">清除记住的 key</button></div>
  <div id="keyerr" style="color:var(--warn);font-size:12px;margin-top:8px"></div>
</div>
<main id="app" style="display:none">
  <div>
    <div class="card">
      <h2>📊 统计概览</h2>
      <div class="stats" id="stats"></div>
      <div class="row"><button id="exportbtn">📦 导出备份（zip）</button><span style="font-size:11px;color:var(--muted);align-self:center">含全部数据与产物文件</span></div>
      <svg class="trend" id="trend" viewBox="0 0 300 66" preserveAspectRatio="none"></svg>
    </div>
    <div class="card">
      <h2>📢 公告</h2>
      <textarea id="noticein" placeholder="公告内容（留空发送即撤下）"></textarea>
      <div class="row"><button class="primary" id="noticesend">发布 / 更新</button><button class="danger" id="noticeclear">撤下</button><span id="noticemsg" style="font-size:12px;color:var(--muted);align-self:center"></span></div>
    </div>
  </div>
  <div>
    <div class="card">
      <h2>💬 留言治理</h2>
      <div id="boardlist"></div>
    </div>
    <div class="card">
      <h2>📚 产物清单</h2>
      <div id="wblist"></div>
    </div>
  </div>
</main>
<script>
(function(){
  var K='th_adminkey';
  function toast(msg){var t=document.getElementById('toast');t.textContent=msg;t.classList.add('show');clearTimeout(t._tm);t._tm=setTimeout(function(){t.classList.remove('show')},1800)}
  function key(){return localStorage.getItem(K)||''}
  function jget(u){return fetch(u,{headers:{'X-Access-Key':key()}}).then(function(r){return r.json()})}
  function jsend(m,u,b){return fetch(u,{method:m,headers:{'Content-Type':'application/json','X-Access-Key':key()},body:JSON.stringify(b||{})}).then(function(r){return r.json()})}
  function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,function(c){return({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]})}
  var login=document.getElementById('login'),app=document.getElementById('app');
  async function enter(){
    var r=await jget('/api/admin/ping');
    if(!r.ok){document.getElementById('keyerr').textContent='key 不对';return}
    document.getElementById('keyerr').textContent='';
    login.style.display='none';app.style.display='grid';
    load(r);
  }
  function load(ping){
    /* 统计 */
    jget('/api/workbenches?all=1').then(async function(wj){
      var bj=await jget('/api/board');
      var wbs=wj.workbenches||[],board=bj.board||[];
      var views=0,dl=0;
      wbs.forEach(function(w){views+=w.views||0;dl+=w.downloads||0});
      document.getElementById('stats').innerHTML=
        '<div class="s"><b>'+wbs.length+'</b><span>产物</span></div>'+
        '<div class="s"><b>'+views+'</b><span>累计浏览</span></div>'+
        '<div class="s"><b>'+dl+'</b><span>下载</span></div>'+
        '<div class="s"><b>'+((ping&&ping.today)||0)+'</b><span>今日浏览</span></div>'+
        '<div class="s"><b>'+board.length+'</b><span>留言</span></div>'+
        '<div class="s"><b>'+((ping&&ping.daily?Object.keys(ping.daily).length:0))+'</b><span>活跃天数</span></div>';
      /* 7 天趋势 */
      var d=(ping&&ping.daily)||{},days=[],now=new Date();
      for(var i=6;i>=0;i--){var dt=new Date(now.getTime()-i*86400000);days.push({k:dt.getFullYear()+'-'+String(dt.getMonth()+1).padStart(2,'0')+'-'+String(dt.getDate()).padStart(2,'0'),v:d[dt.getFullYear()+'-'+String(dt.getMonth()+1).padStart(2,'0')+'-'+String(dt.getDate()).padStart(2,'0')]||0})}
      var max=Math.max(1,Math.max.apply(null,days.map(function(x){return x.v})));
      var pts=days.map(function(x,i){return (10+i*48)+','+(56-x.v/max*44)}).join(' ');
      document.getElementById('trend').innerHTML='<polyline points="'+pts+'" fill="none" stroke="#15616d" stroke-width="2.5" stroke-linejoin="round"/><text x="8" y="12" font-size="9" fill="#15616d">近 7 天浏览（峰值 '+max+'）</text>';
      /* 留言 */
      document.getElementById('boardlist').innerHTML=board.length?board.map(function(m){
        return '<div class="item"><span class="tx"><b>'+esc(m.name)+'</b> '+esc(m.text)+(m.likes?' · 👍'+m.likes:'')+'</span><span class="tm">'+new Date(m.at*1000).toLocaleDateString()+'</span><button class="danger" data-del="'+esc(m.id)+'">删除</button></div>';
      }).join(''):'<div style="color:var(--muted);font-size:12.5px;padding:8px">暂无留言</div>';
      /* 产物 */
      document.getElementById('wblist').innerHTML=wbs.map(function(w){
        return '<div class="wbrow"><a href="/w/'+encodeURIComponent(w.id)+'" target="_blank">'+esc(w.title)+'</a>'+(w.hidden?'<span style="color:var(--warn);font-size:11px">已下架</span>':'')+'<span class="n">👁 '+w.views+' · ⬇ '+w.downloads+'</span><button data-hide="'+esc(w.id)+'" data-to="'+(w.hidden?'0':'1')+'">'+(w.hidden?'上架':'下架')+'</button><button class="danger" data-delw="'+esc(w.id)+'">删除</button></div>';
      }).join('');
    });
    jget('/api/notice').then(function(n){document.getElementById('noticein').value=n.notice||''});
  }
  document.getElementById('keygo').addEventListener('click',function(){localStorage.setItem(K,document.getElementById('keyin').value.trim());enter()});
  document.getElementById('keyin').addEventListener('keydown',function(e){if(e.key==='Enter')document.getElementById('keygo').click()});
  document.getElementById('keyout').addEventListener('click',function(){localStorage.removeItem(K);login.style.display='';app.style.display='none'});
  document.getElementById('noticesend').addEventListener('click',async function(){
    var r=await jsend('POST','/api/notice',{text:document.getElementById('noticein').value.trim()});
    document.getElementById('noticemsg').textContent=r.ok?'已发布':'失败:'+(r.error||'');
  });
  document.getElementById('noticeclear').addEventListener('click',async function(){
    var r=await jsend('POST','/api/notice',{text:''});
    document.getElementById('noticemsg').textContent=r.ok?'已撤下':'失败';
    document.getElementById('noticein').value='';
  });
  document.getElementById('boardlist').addEventListener('click',async function(e){
    var b=e.target.closest('[data-del]');if(!b)return;
    if(!confirm('删除这条留言？'))return;
    var r=await jsend('DELETE','/api/board',{id:b.getAttribute('data-del')});
    if(r.ok){var p=await jget('/api/admin/ping');load(p)}
  });
  document.getElementById('wblist').addEventListener('click',async function(e){
    var h=e.target.closest('[data-hide]');
    if(h){
      var r=await jsend('POST','/api/workbenches/'+h.getAttribute('data-hide')+'/hidden',{hidden:h.getAttribute('data-to')==='1'});
      if(r.ok){toast(h.getAttribute('data-to')==='1'?'已下架':'已上架');var p=await jget('/api/admin/ping');load(p)}
      return;
    }
    var b=e.target.closest('[data-delw]');if(!b)return;
    if(!confirm('删除这个产物？（历史备份会保留，可找我恢复）'))return;
    var r=await jsend('DELETE','/api/workbenches/'+b.getAttribute('data-delw'));
    if(r.ok){var p=await jget('/api/admin/ping');load(p)}
  });
  document.getElementById('exportbtn').addEventListener('click',async function(){
    var r=await fetch('/api/admin/export',{headers:{'X-Access-Key':key()}});
    if(!r.ok){toast('导出失败');return}
    var b=await r.blob();var a=document.createElement('a');
    a.href=URL.createObjectURL(b);a.download='teacherhub-backup.zip';a.click();
  });
  if(key())enter();
})();
</script></body></html>`
}

// handleAdmin 管理面板页（页面本身免 key，数据由前端带 key 拉取）。
func handleAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(adminHTML()))
}
