package main

// admin.go —— 化学学科园地 · 管理后台（/admin，统一 key 门）。
// 左侧导航七板块：总览 / 内容管理 / 成绩管理 / 课标认领 / 互动治理 / 备份数据 / 操作日志。
// 复用既有 REST API；操作日志由服务端在写操作时记录（/api/admin/logs）。

import "net/http"

// handleAdmin 管理后台页（页面本身免 key，数据由前端带 key 拉取）。
func handleAdmin(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_, _ = w.Write([]byte(adminPage))
}

const adminPage = `<!doctype html>
<html lang="zh-CN">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>管理后台 · 化学学科园地</title>
<style>
:root{
  --green-900:#2F6B45;--green-700:#3C7D52;--green-600:#4E9A63;--green-400:#8FC79B;
  --green-100:#E3F0E4;--green-50:#F0F6EF;--sand-100:#F7E8C8;--sand-700:#B08A4A;
  --ink-900:#1F2A22;--ink-700:#3E4A41;--ink-500:#7C8A7F;--ink-300:#A8B2A9;
  --bg:#F7F9F6;--surface:#fff;--line:#EBEFE9;--line2:#EDF1EC;
  --danger:#C2453F;
}
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:-apple-system,"PingFang SC","Microsoft YaHei","Noto Sans SC",sans-serif;background:var(--bg);background-image:radial-gradient(#E4EAE2 1px,transparent 1px);background-size:22px 22px;color:var(--ink-700);font-size:13.5px;line-height:1.6}
a{color:var(--green-700);text-decoration:none}
button{font-family:inherit;cursor:pointer;border:none;background:none}
input,textarea,select{font-family:inherit;font-size:13px;color:var(--ink-900)}
#toast{position:fixed;left:50%;top:16px;transform:translateX(-50%) translateY(-14px);background:var(--green-900);color:#fff;padding:10px 22px;border-radius:999px;font-size:13px;opacity:0;transition:all .25s;z-index:999;pointer-events:none;box-shadow:0 6px 20px rgba(47,107,69,.3)}
#toast.show{opacity:1;transform:translateX(-50%)}
@media (prefers-color-scheme:dark){
:root{
  --green-900:#8FD3A8;--green-700:#6FBE8C;--green-600:#5EAF7E;--green-400:#4E9A63;
  --green-100:#22362A;--green-50:#1E2C23;
  --sand-100:#3A3423;--sand-700:#D9B872;
  --ink-900:#E9F1EB;--ink-700:#C4D2C8;--ink-500:#8C9C90;--ink-300:#66756B;
  --bg:#131A15;--surface:#1B241E;--line:#2A362D;--line2:#263028;
}
body{background:var(--bg);background-image:radial-gradient(#243027 1px,transparent 1px)}
.side{background:var(--surface);border-color:var(--line)}
.side-item:hover{background:#222D25}
.card{background:var(--surface);border-color:var(--line2)}
.stat{background:#1E2C23;border-color:var(--green-100)}
.btn.primary{background:#3C7D52}
.btn.primary:hover{background:#4E9A63}
.btn.line,.btn.danger{background:#222D25;border-color:var(--line);color:var(--ink-700)}
.btn.danger{border-color:#5A3634;color:#E08A85}
input.f,textarea.f,select.f{background:#222D25;border-color:var(--line);color:var(--ink-900)}
.tag.g{background:#1E2C23;color:#6FBE8C}
.tag.y{background:#3A3423;color:#D9B872}
.tag.r{background:#3A2422;color:#E08A85}
.tbl tr:hover td{background:#212D24}
.mono{background:#1E2C23;border-color:var(--green-600)}
.login{background:var(--surface);border-color:var(--line2)}
.login button{background:#3C7D52}
.login button:hover{background:#4E9A63}
#toast{background:rgba(78,154,99,.95)}
}
/* ===== 登录 ===== */
.login-wrap{min-height:100vh;display:flex;align-items:center;justify-content:center;padding:20px}
.login{width:380px;max-width:100%;background:var(--surface);border:1px solid var(--line2);border-radius:20px;padding:34px 32px;box-shadow:0 8px 30px rgba(31,42,34,.07)}
.login h1{font-size:19px;color:var(--ink-900)}
.login p{font-size:12.5px;color:var(--ink-500);margin:8px 0 20px}
.login input{width:100%;height:46px;border:1.5px solid var(--line);border-radius:12px;padding:0 15px;outline:none}
.login input:focus{border-color:var(--green-600);box-shadow:0 0 0 3px rgba(78,154,99,.14)}
.login button{width:100%;height:46px;background:var(--green-700);color:#fff;border-radius:12px;font-size:14px;font-weight:600;margin-top:14px}
.login button:hover{background:var(--green-900)}
.login .err{color:var(--danger);font-size:12px;margin-top:10px;min-height:17px}
/* ===== 后台布局 ===== */
.layout{display:flex;min-height:100vh}
.side{width:196px;flex:none;background:var(--surface);border-right:1px solid var(--line);position:sticky;top:0;height:100vh;overflow-y:auto;padding:18px 0 14px;display:flex;flex-direction:column}
.side-brand{display:flex;align-items:center;gap:9px;padding:0 18px 14px}
.side-brand .logo{width:36px;height:36px;border-radius:11px;background:var(--green-100);display:flex;align-items:center;justify-content:center;font-size:17px}
.side-brand b{font-size:14px;color:var(--ink-900)}
.side-brand small{display:block;font-size:10.5px;color:var(--ink-300)}
.side-nav{padding:0 10px;flex:1}
.side-item{display:flex;align-items:center;gap:9px;height:40px;padding:0 12px;border-radius:10px;color:var(--ink-700);font-size:13px;cursor:pointer;user-select:none;margin-bottom:2px;transition:background .15s,color .15s}
.side-item:hover{background:#F4F7F3}
.side-item.on{background:var(--green-100);color:var(--green-900);font-weight:600}
.side-item .ic{width:18px;text-align:center;font-size:14px}
.side-foot{padding:12px 18px 0;border-top:1px solid var(--line);margin:10px 8px 0}
.side-foot button{width:100%;height:38px;border:1px solid var(--line);border-radius:10px;font-size:12px;color:var(--ink-500)}
.side-foot button:hover{border-color:var(--green-400);color:var(--green-900)}
.main{flex:1;min-width:0;padding:26px 30px 60px;max-width:1180px}
.view{display:none}
.view.on{display:block;animation:vIn .22s ease both}
@keyframes vIn{from{opacity:0;transform:translateY(8px)}to{opacity:1;transform:none}}
h2.title{font-size:19px;color:var(--ink-900);margin-bottom:4px}
p.sub{font-size:12px;color:var(--ink-500);margin-bottom:18px}
.card{background:var(--surface);border:1px solid var(--line2);border-radius:16px;padding:20px 22px;box-shadow:0 1px 2px rgba(31,42,34,.04),0 4px 12px rgba(31,42,34,.03);margin-bottom:18px;transition:box-shadow .2s}
.card:hover{box-shadow:0 4px 16px rgba(31,42,34,.06)}
.card h3{font-size:14px;color:var(--ink-900);margin-bottom:12px}
.stats{display:grid;grid-template-columns:repeat(3,1fr);gap:14px}
.stat{background:var(--green-50);border:1px solid var(--green-100);border-radius:13px;padding:16px 18px}
.stat .l{font-size:11.5px;color:var(--ink-500)}
.stat .n{font-size:23px;font-weight:700;color:var(--ink-900);margin-top:3px}
.quick{display:flex;flex-wrap:wrap;gap:10px;margin-top:14px}
.btn{height:40px;padding:0 18px;border-radius:11px;font-size:12.5px;font-weight:600;display:inline-flex;align-items:center;gap:6px;transition:all .15s}
.btn:hover{transform:translateY(-1px)}
.btn:active{transform:translateY(0)}
.btn.primary{background:var(--green-700);color:#fff}
.btn.primary:hover{background:var(--green-900)}
.btn.line{background:#fff;border:1px solid var(--line);color:var(--ink-700)}
.btn.line:hover{border-color:var(--green-400);color:var(--green-900)}
.btn.danger{background:#fff;border:1px solid #E8C5C3;color:var(--danger)}
.btn.danger:hover{background:#FBF0EF}
.btn.sm{height:30px;padding:0 11px;font-size:11.5px;border-radius:8px}
.tbl-wrap{overflow-x:auto}
table.tbl{width:100%;border-collapse:collapse;font-size:12.5px;min-width:560px}
.tbl th{font-size:11px;font-weight:600;color:var(--ink-500);text-align:left;padding:9px 11px;border-bottom:1px solid var(--line);letter-spacing:.5px;white-space:nowrap}
.tbl td{padding:10px 11px;border-bottom:1px solid var(--line);vertical-align:middle}
.tbl tr:last-child td{border-bottom:none}
.tbl tr:hover td{background:#FAFCF9}
.tag{display:inline-block;font-size:10.5px;padding:2px 9px;border-radius:999px;white-space:nowrap}
.tag.g{background:var(--green-100);color:var(--green-700)}
.tag.y{background:var(--sand-100);color:var(--sand-700)}
.tag.r{background:#F7E3E2;color:var(--danger)}
.row{display:flex;flex-wrap:wrap;gap:10px;align-items:center}
input.f,textarea.f,select.f{width:100%;border:1.5px solid var(--line);border-radius:11px;padding:10px 13px;outline:none;background:#fff}
input.f:focus,textarea.f:focus,select.f:focus{border-color:var(--green-600);box-shadow:0 0 0 3px rgba(78,154,99,.12)}
textarea.f{min-height:92px;resize:vertical;line-height:1.7}
label.lb{display:block;font-size:11.5px;color:var(--ink-500);margin:11px 0 5px;font-weight:600}
.empty{padding:26px 0;text-align:center;color:var(--ink-300);font-size:12.5px}
.mono{font-family:ui-monospace,Consolas,monospace;font-size:11.5px;background:var(--green-50);border:1px dashed var(--green-400);border-radius:9px;padding:8px 12px;word-break:break-all;color:var(--green-900)}
.hist-item{display:flex;align-items:center;gap:10px;padding:7px 0;border-bottom:1px solid var(--line);font-size:12px}
.hist-item:last-child{border-bottom:none}
.form-grid{display:grid;grid-template-columns:1fr 1fr;gap:0 16px}
@media (max-width:860px){
  .layout{flex-direction:column}
  .side{width:100%;height:auto;position:static;flex-direction:row;align-items:center;overflow-x:auto;padding:10px 12px;gap:8px}
  .side-brand{padding:0 8px 0 0;flex:none}
  .side-brand small{display:none}
  .side-nav{display:flex;gap:6px;padding:0;flex:none}
  .side-item{margin:0;white-space:nowrap;height:38px}
  .side-foot{border:none;margin:0;padding:0 0 0 8px;flex:none}
  .side-foot button{width:auto;padding:0 14px}
  .main{padding:18px 14px 60px}
  .stats{grid-template-columns:repeat(2,1fr)}
  .form-grid{grid-template-columns:1fr}
}
</style>
</head>
<body>
<div id="toast"></div>

<div class="login-wrap" id="loginWrap">
  <div class="login">
    <h1>🧪 化学学科园地 · 管理后台</h1>
    <p>输入统一访问 key 进入。key 找教研组长，别转发进学生群。</p>
    <input id="keyin" type="password" placeholder="管理 key" autocomplete="off">
    <button id="keygo">进入后台</button>
    <div class="err" id="keyerr"></div>
  </div>
</div>

<div class="layout" id="app" style="display:none">
  <aside class="side">
    <div class="side-brand">
      <div class="logo">⚗️</div>
      <div><b>管理后台</b><small>化学学科园地</small></div>
    </div>
    <nav class="side-nav" id="sideNav">
      <div class="side-item on" data-v="home"><span class="ic">📊</span>总览</div>
      <div class="side-item" data-v="content"><span class="ic">📚</span>内容管理</div>
      <div class="side-item" data-v="score"><span class="ic">📈</span>成绩管理</div>
      <div class="side-item" data-v="kb"><span class="ic">🗺️</span>课标认领</div>
      <div class="side-item" data-v="social"><span class="ic">💬</span>互动治理</div>
      <div class="side-item" data-v="backup"><span class="ic">💾</span>备份数据</div>
      <div class="side-item" data-v="logs"><span class="ic">📜</span>操作日志</div>
    </nav>
    <div class="side-foot">
      <button id="backPortal">← 回门户</button>
    </div>
  </aside>

  <main class="main">
    <!-- ===== 总览 ===== -->
    <section class="view on" id="v-home">
      <h2 class="title">总览</h2>
      <p class="sub">平台运行数据与快捷操作。</p>
      <div class="stats" id="homeStats"></div>
      <div class="quick">
        <button class="btn primary" data-goto="score">📈 录成绩</button>
        <button class="btn line" data-goto="social">📢 发公告</button>
        <button class="btn line" data-goto="content">📚 管内容</button>
        <button class="btn line" id="exportBtn">📦 导出全量 zip</button>
      </div>
      <div class="card" style="margin-top:18px">
        <h3>近 7 天浏览</h3>
        <div id="homeTrend"></div>
      </div>
      <div class="card">
        <h3>成绩趋势</h3>
        <div id="scoreMiniTrend"><div style="font-size:12px;color:#7C8A7F">录入 ≥2 场考试后显示趋势</div></div>
      </div>
      <div class="card">
        <h3>文件分类统计</h3>
        <div id="fileMiniChart"><div style="font-size:12px;color:#7C8A7F">暂无文件</div></div>
      </div>
      <div class="card">
        <h3>系统信息</h3>
        <label class="lb">当前管理 key</label>
        <div class="row">
          <div class="mono" id="keyMask" style="flex:1;min-width:220px">••••</div>
          <button class="btn danger" id="rotateBtn">🔄 轮换 key</button>
        </div>
        <label class="lb">MCP 接入地址（给 AI 客户端）</label>
        <div class="mono" id="mcpUrl"></div>
        <label class="lb">存储占用</label>
        <div class="mono" id="diskInfo">加载中…</div>
        <label class="lb">门户与管理入口</label>
        <div class="mono" id="portalUrl"></div>
      </div>
    </section>

    <!-- ===== 内容管理 ===== -->
    <section class="view" id="v-content">
      <h2 class="title">内容管理</h2>
      <p class="sub">实验模块与产物：新建、上下架、版本恢复、删除。</p>
      <div class="card">
        <h3>产物清单</h3>
        <div class="tbl-wrap"><table class="tbl" id="wbTable">
          <thead><tr><th>标题</th><th>作者</th><th>浏览</th><th>下载</th><th>评分</th><th>状态</th><th style="min-width:210px">操作</th></tr></thead>
          <tbody></tbody>
        </table></div>
        <div class="empty" id="wbEmpty" style="display:none">还没有产物——用下面的表单或让 AI 生成</div>
      </div>
      <div class="card">
        <h3>资源文件</h3>
        <div class="tbl-wrap"><table class="tbl" id="fileTable">
          <thead><tr><th>文件名</th><th>大小</th><th>上传人</th><th>下载</th><th>时间</th><th>操作</th></tr></thead>
          <tbody></tbody>
        </table></div>
        <div class="empty" id="fileEmpty" style="display:none">暂无共享文件</div>
      </div>
      <div class="card">
        <h3>内置实验模板</h3>
        <div class="row" style="margin-bottom:10px">
          <button class="btn primary" id="seedBtn">🧪 一键上架四个实验模板</button>
          <span style="font-size:11.5px;color:var(--ink-300)">幂等操作——已有同名产物自动跳过</span>
        </div>
        <div id="seedResult" style="font-size:12px;color:var(--ink-500)"></div>
      </div>
      <div class="card">
        <h3>新建产物</h3>
        <div class="form-grid">
          <div><label class="lb">标题 *</label><input class="f" id="nwTitle" placeholder="如 钠与水反应实验工作台"></div>
          <div><label class="lb">作者</label><input class="f" id="nwTeacher" placeholder="教研组"></div>
        </div>
        <label class="lb">简介</label><input class="f" id="nwDesc" placeholder="一句话说明">
        <label class="lb">标签（逗号分隔）</label><input class="f" id="nwTags" placeholder="化学,实验,钠">
        <label class="lb">HTML 内容 *</label><textarea class="f" id="nwContent" placeholder="粘贴单文件 HTML 全文"></textarea>
        <div class="row" style="margin-top:12px">
          <button class="btn primary" id="nwCreate">创建产物</button>
          <span style="font-size:11.5px;color:var(--ink-300)">创建后获得固定链接，发群里打开即用</span>
        </div>
      </div>
    </section>

    <!-- ===== 成绩管理 ===== -->
    <section class="view" id="v-score">
      <h2 class="title">成绩管理</h2>
      <p class="sub">录入成绩自动生成统计；原始分只进统计不公开。</p>
      <div class="card">
        <h3>录入考试</h3>
        <div class="form-grid">
          <div><label class="lb">考试名 *</label><input class="f" id="sxName" placeholder="如 高三月考一"></div>
          <div><label class="lb">分数列表 *（空格/逗号/换行分隔）</label><input class="f" id="sxScores" placeholder="88 92 76 45 100"></div>
        </div>
        <label class="lb">题目数据（可选 · 逐题失分分析）</label>
        <input class="f" id="sxQs" placeholder="格式 题号:满分:均分，分号分隔，如 1:12:9.5; 2:10:6.2; 3:8:3.1">
        <div class="row" style="margin-top:12px">
          <button class="btn primary" id="sxAdd">录入</button>
          <span style="font-size:11.5px;color:var(--ink-300)">分数 0—150，最多 500 个</span>
        </div>
      </div>
      <div class="card">
        <h3>批量导入</h3>
        <p style="font-size:12px;color:var(--ink-500);margin-bottom:10px">粘贴 CSV 格式（每行一场考试）：考试名,分数1,分数2,...</p>
        <textarea class="f" id="bulkScores" placeholder="月考一,88,92,76,45,100,67&#10;月考二,90,85,88,60,95,70" style="min-height:120px"></textarea>
        <div class="row" style="margin-top:10px">
          <button class="btn primary" id="bulkImport">批量导入</button>
          <span id="bulkMsg" style="font-size:12px;color:var(--ink-500)"></span>
        </div>
      </div>
      <div class="card">
        <h3>考试列表</h3>
        <div style="display:flex;gap:8px;margin-bottom:10px;align-items:center">
          <button class="btn danger sm" id="batchDelSx">🗑️ 批量删除选中</button>
          <span style="font-size:11.5px;color:var(--ink-400)">勾选后一键删除多场考试</span>
        </div>
        <div class="tbl-wrap"><table class="tbl" id="sxTable">
          <thead><tr><th><input type="checkbox" id="sxCheckAll" title="全选"/></th><th>考试</th><th>人数</th><th>均分</th><th>最高</th><th>最低</th><th>及格率</th><th>时间</th><th>操作</th></tr></thead>
          <tbody></tbody>
        </table></div>
        <div class="empty" id="sxEmpty" style="display:none">还没有成绩记录</div>
      </div>
    </section>

    <!-- ===== 课标认领 ===== -->
    <section class="view" id="v-kb">
      <h2 class="title">课标认领</h2>
      <p class="sub">课标模块的映射认领管理。</p>
      <div class="card">
        <h3>手动认领 / 改派</h3>
        <div class="form-grid">
          <div><label class="lb">模块</label>
            <select class="f" id="kbModule">
              <option>化学实验基础</option><option>物质结构与元素周期律</option>
              <option>化学反应与能量</option><option>有机化合物</option><option>化学反应原理</option>
            </select>
          </div>
          <div><label class="lb">认领人</label><input class="f" id="kbBy" placeholder="教师姓名"></div>
        </div>
        <button class="btn primary" id="kbClaim" style="margin-top:12px">认领</button>
      </div>
      <div class="card">
        <h3>认领清单</h3>
        <div class="tbl-wrap"><table class="tbl" id="kbTable">
          <thead><tr><th>模块</th><th>认领人</th><th>时间</th><th>操作</th></tr></thead>
          <tbody></tbody>
        </table></div>
        <div class="empty" id="kbEmpty" style="display:none">还没有认领记录</div>
      </div>
    </section>

    <!-- ===== 互动治理 ===== -->
    <section class="view" id="v-social">
      <h2 class="title">互动治理</h2>
      <p class="sub">留言、公告与贡献记录。</p>
      <div class="card">
        <h3>公告</h3>
        <textarea class="f" id="noticeIn" placeholder="公告内容（留空发送即撤下）"></textarea>
        <div class="row" style="margin-top:10px">
          <button class="btn primary" id="noticeSend">发布 / 更新</button>
          <button class="btn danger" id="noticeClear">撤下</button>
        </div>
      </div>
      <div class="card">
        <h3>留言治理</h3>
        <div class="tbl-wrap"><table class="tbl" id="boardTable">
          <thead><tr><th>昵称</th><th>内容</th><th>👍</th><th>时间</th><th>操作</th></tr></thead>
          <tbody></tbody>
        </table></div>
        <div class="empty" id="boardEmpty" style="display:none">暂无留言</div>
      </div>
      <div class="card">
        <h3>贡献记录</h3>
        <div class="tbl-wrap"><table class="tbl" id="cgTable">
          <thead><tr><th>教师</th><th>内容</th><th>时间</th><th>操作</th></tr></thead>
          <tbody></tbody>
        </table></div>
        <div class="empty" id="cgEmpty" style="display:none">暂无贡献记录</div>
      </div>
    </section>

    <!-- ===== 备份数据 ===== -->
    <section class="view" id="v-backup">
      <h2 class="title">备份数据</h2>
      <p class="sub">自动快照与全量导出。</p>
      <div class="card">
        <h3>自动备份快照（每 24h 轮转 7 份）</h3>
        <div class="row" style="margin-bottom:10px">
          <button class="btn primary" id="backupNow">⚡ 立即备份</button>
          <button class="btn line" id="cleanupBtn">🧹 清理旧备份</button>
          <span style="font-size:11.5px;color:var(--ink-300)">含数据 + 产物 + 共享文件</span>
        </div>
        <div id="backupList"></div>
      </div>
      <div class="card">
        <h3>全量导出</h3>
        <p style="font-size:12px;color:var(--ink-500);margin-bottom:10px">打包全部数据与产物文件（store.json + artifacts）。</p>
        <button class="btn primary" id="exportBtn2">📦 导出 zip</button>
      </div>
    </section>

    <!-- ===== 操作日志 ===== -->
    <section class="view" id="v-logs">
      <h2 class="title">操作日志</h2>
      <p class="sub">管理操作留痕（滚动保留 500 条）。</p>
      <div class="card">
        <div style="display:flex;gap:8px;margin-bottom:12px;align-items:center;flex-wrap:wrap">
          <select id="logFilter" style="height:32px;border:1.5px solid #ddd;border-radius:8px;padding:0 10px;font-size:12px">
            <option value="">全部操作</option>
            <option value="创建">创建</option>
            <option value="更新">更新</option>
            <option value="删除">删除</option>
            <option value="备份">备份</option>
            <option value="密钥">密钥</option>
            <option value="上传">上传</option>
            <option value="导入">导入</option>
          </select>
          <input id="logSearch" placeholder="搜索日志关键词…" style="height:32px;border:1.5px solid #ddd;border-radius:8px;padding:0 12px;font-size:12px;flex:1;min-width:160px"/>
          <button class="btn line sm" id="exportLogs">📥 导出 CSV</button>
        </div>
        <div class="tbl-wrap"><table class="tbl" id="logTable">
          <thead><tr><th>时间</th><th>操作</th><th>详情</th></tr></thead>
          <tbody></tbody>
        </table></div>
        <div class="empty" id="logEmpty" style="display:none">暂无操作记录</div>
      </div>
    </section>
  </main>
</div>

<script>
"use strict";
var $=function(s){return document.querySelector(s)};
function esc(s){return String(s==null?'':s).replace(/[&<>"']/g,function(c){return({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'})[c]})}
function toast(m){var t=$('#toast');t.textContent=m;t.classList.add('show');clearTimeout(window.__tT);window.__tT=setTimeout(function(){t.classList.remove('show')},2200)}
var KEY='th_adminkey';
function key(){try{return localStorage.getItem(KEY)||''}catch(e){return ''}}
function jget(u){return fetch(u,{headers:{'X-Access-Key':key()}}).then(function(r){return r.json()})}
function jsend(m,u,b){return fetch(u,{method:m,headers:{'Content-Type':'application/json','X-Access-Key':key()},body:JSON.stringify(b||{})}).then(function(r){return r.json()})}
function fmtTs(ts){if(!ts)return'';var d=new Date(Number(ts)*1000);if(isNaN(d.getTime())||d.getFullYear()<2000)d=new Date(String(ts).replace(' ','T')+'Z');var p=function(x){return String(x).padStart(2,'0')};return d.getFullYear()+'-'+p(d.getMonth()+1)+'-'+p(d.getDate())+' '+p(d.getHours())+':'+p(d.getMinutes())}
function maskKey(k){if(!k)return'••••';return k.slice(0,4)+'••••••••'+k.slice(-4)}

/* ===== 登录 ===== */
function enter(){
  $('#loginWrap').style.display='none';
  $('#app').style.display='flex';
  boot();
}
$('#keygo').addEventListener('click',doLogin);
$('#keyin').addEventListener('keydown',function(e){if(e.key==='Enter')doLogin()});
function doLogin(){
  var k=$('#keyin').value.trim();
  if(!k){$('#keyerr').textContent='输入 key';return}
  localStorage.setItem(KEY,k);
  jget('/api/admin/ping').then(function(d){
    if(d.ok){$('#keyerr').textContent='';enter()}else{$('#keyerr').textContent=d.error||'key 不对'}
  }).catch(function(){$('#keyerr').textContent='网络错误'});
}
if(key()){jget('/api/admin/ping').then(function(d){if(d.ok)enter()}).catch(function(){})}

/* ===== 导航 ===== */
document.addEventListener('click',function(e){
  var it=e.target.closest('[data-v]');
  if(it){
    document.querySelectorAll('.side-item').forEach(function(n){n.classList.remove('on')});
    it.classList.add('on');
    document.querySelectorAll('.view').forEach(function(v){v.classList.remove('on')});
    var v=$('#v-'+it.dataset.v);if(v)v.classList.add('on');
    window.scrollTo(0,0);
  }
  var g=e.target.closest('[data-goto]');
  if(g){var t=document.querySelector('[data-v="'+g.dataset.goto+'"]');if(t)t.click()}
});
$('#backPortal').addEventListener('click',function(){location.href='/'});

/* ===== 总览 ===== */
function renderFileMini(){
  jget('/api/files').then(function(d){
    var files=d.files||[];
    if(!files.length)return;
    var cats={"图片":0,"文档":0,"表格":0,"其他":0};
    files.forEach(function(f){
      var ext=(f.name||"").split(".").pop().toLowerCase();
      if(["png","jpg","jpeg","gif","webp","svg","bmp"].indexOf(ext)>=0)cats["图片"]++;
      else if(["doc","docx","pdf","txt","md","ppt","pptx"].indexOf(ext)>=0)cats["文档"]++;
      else if(["xls","xlsx","csv"].indexOf(ext)>=0)cats["表格"]++;
      else cats["其他"]++;
    });
    var colors={"图片":"#5B9BD5","文档":"#4E9A63","表格":"#E5A93F","其他":"#A8B2A9"};
    var html='<div style="display:flex;gap:12px;flex-wrap:wrap;margin-top:8px">';
    Object.keys(cats).forEach(function(k){
      if(!cats[k])return;
      var pct=Math.round(cats[k]/files.length*100);
      html+='<div style="display:flex;align-items:center;gap:6px"><div style="width:12px;height:12px;border-radius:3px;background:'+colors[k]+'"></div><span style="font-size:12px">'+k+' '+cats[k]+'个('+pct+'%)</span></div>';
    });
    html+='</div>';
    var el=document.getElementById('fileMiniChart');
    if(el)el.innerHTML=html;
  });
}
function renderScoreMini(){
  jget('/api/scores').then(function(sc){
    var list=(sc&&sc.exams)||[];
    if(list.length<2){return}
    var sorted=list.slice().sort(function(a,b){return (a.at||0)-(b.at||0)});
    var W=560,H=130;
    var avgs=sorted.map(function(e){return e.avg});
    var mn=Math.min.apply(null,avgs),mx=Math.max.apply(null,avgs),span=Math.max(1,mx-mn);
    var pts=sorted.map(function(e,i){
      var x=20+i/Math.max(1,sorted.length-1)*(W-40);
      var y=H-25-((e.avg-mn)/span)*(H-55);
      return [x,y,e];
    });
    var path=pts.map(function(p,i){return (i?"L":"M")+p[0].toFixed(1)+" "+p[1].toFixed(1)}).join(" ");
    var svg='<svg viewBox="0 0 '+W+' '+H+'" style="width:100%;height:'+H+'px">';
    svg+='<path d="'+path+'" stroke="#4E9A63" stroke-width="2.4" fill="none" stroke-linecap="round"/>';
    pts.forEach(function(p){
      svg+='<circle cx="'+p[0].toFixed(1)+'" cy="'+p[1].toFixed(1)+'" r="4" fill="#fff" stroke="#2F6B45" stroke-width="2"/>';
      svg+='<text x="'+p[0].toFixed(1)+'" y="'+(p[1]-10).toFixed(1)+'" font-size="10.5" fill="#2F6B45" text-anchor="middle" font-weight="700">'+p[2].avg+'</text>';
    });
    svg+='</svg>';
    var el=document.getElementById('scoreMiniTrend');
    if(el)el.innerHTML=svg;
  });
}
document.addEventListener("click",function(e){
  if(e.target.id==="cleanupBtn"){
    if(!confirm("将删除 10 个以外的旧备份，确认？"))return;
    fetch("/api/admin/cleanup-backups",{method:"POST",headers:{"Content-Type":"application/json","X-Access-Key":key()}}).then(function(r){return r.json()}).then(function(d){
      if(d.ok){alert("已清理 "+d.removed+" 个旧备份（保留最近 10 个）");renderBackups()}
      else alert(d.error||"清理失败");
    }).catch(function(){alert("清理失败")});
  }
});
function renderHome(){
  Promise.all([
    jget('/api/stats').catch(function(){return {}}),
    jget('/api/workbenches?all=1').catch(function(){return {workbenches:[]}}),
    jget('/api/contrib').catch(function(){return {contribs:[]}}),
    jget('/api/board').catch(function(){return {board:[]}})
  ]).then(function(rs){
    var t=(rs[0].totals)||{},wbs=rs[1].workbenches||[],cgs=rs[2].contribs||[],bds=rs[3].board||[];
    $('#homeStats').innerHTML=[
      ['今日浏览',t.today_views||0],
      ['总浏览',t.views||0],
      ['总下载',t.downloads||0],
      ['产物',wbs.length],
      ['贡献',cgs.length],
      ['留言',bds.length]
    ].map(function(x){return '<div class="stat"><div class="l">'+x[0]+'</div><div class="n">'+x[1]+'</div></div>'}).join('');
    var daily=t.daily||{},days=Object.keys(daily).sort();
    if(days.length>=2){
      var W=680,H=110,mx=Math.max.apply(null,days.map(function(k){return daily[k]}))||1;
      var pts=days.map(function(k,i){var x=20+i/(days.length-1)*(W-40),y=H-26-(daily[k]/mx)*(H-52);return [x,y,daily[k],k]});
      var path=pts.map(function(p,i){return (i?'L':'M')+p[0].toFixed(1)+' '+p[1].toFixed(1)}).join(' ');
      $('#homeTrend').innerHTML='<svg viewBox="0 0 '+W+' '+H+'" style="width:100%;height:'+H+'px"><path d="'+path+'" stroke="#4E9A63" stroke-width="2.2" fill="none" stroke-linecap="round"/>'+pts.map(function(p){return '<circle cx="'+p[0].toFixed(1)+'" cy="'+p[1].toFixed(1)+'" r="3" fill="#2F6B45"><title>'+esc(p[3])+': '+p[2]+'</title></circle>'}).join('')+'<text x="20" y="12" font-size="10" fill="#A8B2A9">峰值 '+mx+'</text></svg>';
    }else{$('#homeTrend').innerHTML='<div class="empty">数据积累中（至少 2 天）</div>'}
    if(d.disk_size!=null){
      var mb=(d.disk_size/1048576).toFixed(1);
      $('#diskInfo').textContent='数据目录 '+mb+' MB · 共享文件 '+d.file_count+' 个 · 产物 '+d.artifact_count+' 个';
    }
    $('#keyMask').textContent=maskKey(key());
    $('#mcpUrl').textContent=location.origin+'/mcp';
    $('#portalUrl').textContent=location.origin+'/  ·  后台 '+location.origin+'/admin';
  });
}
$('#rotateBtn').addEventListener('click',function(){
  if(!confirm('轮换 key？旧 key 立即作废，所有同事须换新 key。'))return;
  if(!confirm('再次确认：这会切断现有管理连接。'))return;
  jsend('POST','/api/admin/rotate-key').then(function(d){
    if(d.ok){
      localStorage.setItem(KEY,d.key);
      $('#keyMask').textContent=maskKey(d.key);
      toast('新 key 已生成并启用：'+d.key.slice(0,6)+'…');
      renderLogs();
    }else{toast(d.error||'轮换失败')}
  });
});
function doExport(){
  fetch('/api/admin/export',{headers:{'X-Access-Key':key()}}).then(function(r){
    if(!r.ok){toast('导出失败');return}
    return r.blob().then(function(b){
      var a=document.createElement('a');a.href=URL.createObjectURL(b);a.download='teacherhub-backup.zip';a.click();
      toast('导出已开始下载');
    });
  }).catch(function(){toast('导出失败')});
}
$('#exportBtn').addEventListener('click',doExport);
$('#exportBtn2').addEventListener('click',doExport);

/* ===== 内容管理 ===== */
function renderWb(){
  jget('/api/workbenches?all=1').then(function(d){
    var list=d.workbenches||[];
    $('#wbEmpty').style.display=list.length?'none':'block';
    $('#wbTable').querySelector('tbody').innerHTML=list.map(function(w){
      var rate=w.rate_num?('<span class="tag g">⭐'+Number(w.rate).toFixed(1)+'</span>'):'<span style="color:var(--ink-300)">—</span>';
      return '<tr><td><a href="/w/'+encodeURIComponent(w.id)+'" target="_blank">'+esc(w.title)+'</a></td><td>'+esc(w.teacher||'教研组')+'</td><td>'+(w.views||0)+'</td><td>'+(w.downloads||0)+'</td><td>'+rate+'</td><td>'+(w.hidden?'<span class="tag y">已下架</span>':'<span class="tag g">上架中</span>')+'</td><td class="row" style="gap:6px">'+
        '<button class="btn line sm" data-hist="'+esc(w.id)+'">历史</button>'+
        '<button class="btn line sm" data-hide="'+esc(w.id)+'" data-to="'+(w.hidden?'0':'1')+'">'+(w.hidden?'上架':'下架')+'</button>'+
        '<button class="btn danger sm" data-delwb="'+esc(w.id)+'">删除</button></td></tr>';
    }).join('');
  });
}
$('#wbTable').addEventListener('click',function(e){
  var b;
  if(b=e.target.closest('[data-hide]')){
    jsend('POST','/api/workbenches/'+b.dataset.hide+'/hidden',{hidden:b.dataset.to==='1'}).then(function(d){if(d.ok){toast('已更新');renderWb();renderLogs()}else{toast(d.error||'失败')}});
  }else if(b=e.target.closest('[data-delwb]')){
    if(!confirm('删除这个产物？.hist 历史仍保留可恢复。'))return;
    jsend('DELETE','/api/workbenches/'+b.dataset.delwb).then(function(d){if(d.ok){toast('已删除');renderWb();renderLogs()}else{toast(d.error||'失败')}});
  }else if(b=e.target.closest('[data-hist]')){
    var id=b.dataset.hist;
    jget('/api/workbenches/'+id+'/history').then(function(d){
      var list=d.history||d.versions||[];
      if(!list.length){toast('没有历史版本');return}
      if(!confirm('历史版本 '+list.length+' 份，恢复最近一份？'))return;
      var ts=(list[0]&&list[0].ts!=null)?list[0].ts:((list[0]&&list[0].at!=null)?list[0].at:list[0]);
      jsend('POST','/api/workbenches/'+id+'/restore',{ts:ts}).then(function(r){if(r.ok){toast('已恢复历史版本');renderWb();renderLogs()}else{toast(r.error||'恢复失败')}});
    });
  }
});
$('#seedBtn').addEventListener('click',function(){
  var btn=this;btn.disabled=true;btn.textContent='上架中…';
  jsend('POST','/api/admin/seed-templates').then(function(d){
    btn.disabled=false;btn.textContent='🧪 一键上架四个实验模板';
    if(d.ok){
      var created=d.results.filter(function(r){return r.status==='created'}).length;
      var skipped=d.results.filter(function(r){return r.status==='skipped'}).length;
      $('#seedResult').innerHTML=d.results.map(function(r){
        var icon=r.status==='created'?'✅':(r.status==='skipped'?'⏭':'❌');
        return icon+' '+r.title+' → '+r.status;
      }).join('<br>');
      toast('上架完成：'+created+' 个新建，'+skipped+' 个跳过');
      renderWb();renderLogs();
    }else{toast(d.error||'上架失败')}
  });
});
$('#nwCreate').addEventListener('click',function(){
  var title=$('#nwTitle').value.trim(),content=$('#nwContent').value;
  if(!title||!content){toast('标题和内容必填');return}
  var tags=$('#nwTags').value.split(/[,，、\s]+/).filter(Boolean);
  jsend('POST','/api/workbenches',{title:title,desc:$('#nwDesc').value.trim(),tags:tags,teacher:$('#nwTeacher').value.trim()||'教研组',content:content}).then(function(d){
    if(d.ok){toast('已创建：'+(d.id||''));$('#nwTitle').value='';$('#nwDesc').value='';$('#nwTags').value='';$('#nwContent').value='';renderWb();renderLogs()}
    else{toast(d.error||'创建失败')}
  });
});

/* ===== 资源文件管理 ===== */
function renderFilesAdmin(){
  jget('/api/files').then(function(d){
    var list=d.files||[];
    $('#fileEmpty').style.display=list.length?'none':'block';
    $('#fileTable').querySelector('tbody').innerHTML=list.map(function(x){
      var sz=x.size<1048576?((x.size||0)/1024).toFixed(1)+' KB':((x.size||0)/1048576).toFixed(2)+' MB';
      return '<tr><td><a href="/api/files/'+encodeURIComponent(x.id)+'">'+esc(x.name)+'</a></td><td>'+sz+'</td><td>'+esc(x.teacher||'匿名')+'</td><td>'+(x.downloads||0)+'</td><td>'+fmtTs(x.at)+'</td><td><button class="btn danger sm" data-delfile="'+esc(x.id)+'">删除</button></td></tr>';
    }).join('');
  });
}
$('#fileTable').addEventListener('click',function(e){
  var b=e.target.closest('[data-delfile]');
  if(!b)return;
  if(!confirm('删除这个文件？'))return;
  jsend('DELETE','/api/files/'+b.dataset.delfile).then(function(d){if(d.ok){toast('已删除');renderFilesAdmin();renderLogs()}else{toast(d.error||'失败')}});
});

/* ===== 成绩管理 ===== */
function renderSx(){
  jget('/api/scores').then(function(d){
    var list=d.exams||[];
    $('#sxEmpty').style.display=list.length?'none':'block';
    $('#sxTable').querySelector('tbody').innerHTML=list.slice().reverse().map(function(e){
      return '<tr><td><input type="checkbox" class="sx-check" data-id="'+esc(e.id)+'"/></td><td><b>'+esc(e.name)+'</b></td><td>'+e.count+' 人</td><td>'+e.avg+'</td><td>'+e.max+'</td><td>'+e.min+'</td><td>'+e.pass_rate+'%</td><td>'+fmtTs(e.at)+'</td><td class="row" style="gap:6px"><a class="btn line sm" href="/api/scores/csv/'+esc(e.id)+'?key='+encodeURIComponent(key())+'">CSV</a><button class="btn danger sm" data-delsx="'+esc(e.id)+'">删除</button></td></tr>';
    }).join('');
  });
}
$('#sxAdd').addEventListener('click',function(){
  var name=$('#sxName').value.trim(),txt=$('#sxScores').value.trim();
  if(!name||!txt){toast('填考试名和分数');return}
  var arr=txt.split(/[\s,，;；]+/).filter(Boolean).map(Number);
  if(!arr.length||arr.some(function(v){return !isFinite(v)||v<0||v>150})){toast('分数须为 0-150 数字');return}
  if(arr.length>500){toast('最多 500 个');return}
  var qs=[];
  var qtxt=($('#sxQs').value||'').trim();
  if(qtxt){
    var bad=false;
    qtxt.split(/[;；]+/).forEach(function(part){
      var t=part.trim();if(!t)return;
      var p=t.split(':');
      if(p.length!==3||!p[0].trim()){bad=true;return}
      var mx=Number(p[1]),av=Number(p[2]);
      if(!isFinite(mx)||!isFinite(av)||mx<=0||av<0||av>mx){bad=true;return}
      qs.push({name:p[0].trim(),max:mx,avg:av});
    });
    if(bad){toast('题目格式不对（题号:满分:均分）');return}
    if(qs.length>50){toast('题目最多 50 题');return}
  }
  jsend('POST','/api/scores',{name:name,scores:arr,questions:qs}).then(function(d){
    if(d.ok){toast('已录入 '+arr.length+' 个分数'+(qs.length?'（含 '+qs.length+' 题统计）':''));$('#sxName').value='';$('#sxScores').value='';$('#sxQs').value='';renderSx();renderLogs()}
    else{toast(d.error||'失败')}
  });
});
$('#bulkImport').addEventListener('click',function(){
  var txt=($('#bulkScores').value||'').trim();
  if(!txt){$('#bulkMsg').textContent='粘贴 CSV 内容';return}
  var lines=txt.split('\n').filter(function(l){return l.trim()});
  var imported=0,errors=[];
  var promises=lines.map(function(line,i){
    var parts=line.split(',').map(function(x){return x.trim()}).filter(Boolean);
    if(parts.length<2){errors.push('第'+(i+1)+'行格式不对');return null}
    var name=parts[0];
    var scores=parts.slice(1).map(Number).filter(function(v){return isFinite(v)&&v>=0&&v<=150});
    if(!name||!scores.length){errors.push('第'+(i+1)+'行数据不对');return null}
    return jsend('POST','/api/scores',{name:name,scores:scores}).then(function(d){
      if(d.ok)imported++;else errors.push(name+': '+(d.error||'失败'));
    });
  }).filter(Boolean);
  Promise.all(promises).then(function(){
    $('#bulkMsg').textContent='导入 '+imported+' 场'+(errors.length?'，失败 '+errors.length+' 条':'');
    if(errors.length)$('#bulkMsg').textContent+='（'+errors.slice(0,3).join('；')+'）';
    $('#bulkScores').value='';
    renderSx();renderLogs();
    toast('批量导入完成：'+imported+' 场');
  });
});
$('#sxTable').addEventListener('click',function(e){
  if(e.target.id==='sxCheckAll'){
    var checked=e.target.checked;
    document.querySelectorAll('.sx-check').forEach(function(c){c.checked=checked});
    return;
  }
  if(e.target.id==='batchDelSx'){
    var ids=[];
    document.querySelectorAll('.sx-check:checked').forEach(function(c){ids.push(c.dataset.id)});
    if(!ids.length){toast('先勾选要删除的考试');return}
    if(!confirm('确认删除 '+ids.length+' 场考试？此操作不可恢复'))return;
    var done=0,fail=0;
    ids.forEach(function(id){
      jsend('DELETE','/api/scores/'+id).then(function(d){
        done++;
        if(d&&d.ok===false)fail++;
        if(done===ids.length){
          if(fail)toast('删除完成：成功 '+(ids.length-fail)+'，失败 '+fail);
          else toast('已删除 '+ids.length+' 场考试');
          renderSx();renderLogs()
        }
      }).catch(function(){done++;fail++;if(done===ids.length){toast('删除完成：成功 '+(ids.length-fail)+'，失败 '+fail);renderSx();renderLogs()}});
    });
    return;
  }
  var b=e.target.closest('[data-delsx]');
  if(!b)return;
  if(!confirm('删除这场考试？'))return;
  jsend('DELETE','/api/scores/'+b.dataset.delsx).then(function(d){if(d.ok){toast('已删除');renderSx();renderLogs()}else{toast(d.error||'失败')}});
});

/* ===== 课标认领 ===== */
function renderKb(){
  jget('/api/kb').then(function(d){
    var claims=d.claims||{},mods=Object.keys(claims);
    $('#kbEmpty').style.display=mods.length?'none':'block';
    $('#kbTable').querySelector('tbody').innerHTML=mods.map(function(m){
      var c=claims[m];
      return '<tr><td>'+esc(m)+'</td><td><b>'+esc(c.by)+'</b></td><td>'+fmtTs(c.at)+'</td><td><button class="btn danger sm" data-unclaim="'+esc(m)+'">清除</button></td></tr>';
    }).join('');
  });
}
$('#kbClaim').addEventListener('click',function(){
  var mod=$('#kbModule').value,by=$('#kbBy').value.trim();
  if(!by){toast('填认领人');return}
  jsend('POST','/api/kb/claim',{module:mod,by:by}).then(function(d){if(d.ok){toast('已认领');renderKb();renderLogs()}else{toast(d.error||'失败')}});
});
$('#kbTable').addEventListener('click',function(e){
  var b=e.target.closest('[data-unclaim]');
  if(!b)return;
  if(!confirm('清除该模块认领？'))return;
  jsend('DELETE','/api/kb/claim',{module:b.dataset.unclaim}).then(function(d){if(d.ok){toast('已清除');renderKb();renderLogs()}else{toast(d.error||'失败')}});
});

/* ===== 互动治理 ===== */
function renderSocial(){
  jget('/api/board').then(function(d){
    var list=d.board||[];
    $('#boardEmpty').style.display=list.length?'none':'block';
    $('#boardTable').querySelector('tbody').innerHTML=list.slice().reverse().map(function(m){
      return '<tr><td>'+esc(m.name)+'</td><td>'+esc(m.text)+'</td><td>'+(m.likes||0)+'</td><td>'+fmtTs(m.at)+'</td><td><button class="btn danger sm" data-delbd="'+esc(m.id)+'">删除</button></td></tr>';
    }).join('');
  });
  jget('/api/contrib').then(function(d){
    var list=d.contribs||[];
    $('#cgEmpty').style.display=list.length?'none':'block';
    $('#cgTable').querySelector('tbody').innerHTML=list.slice().reverse().map(function(c){
      return '<tr><td>'+esc(c.teacher)+'</td><td>'+esc(c.text)+'</td><td>'+fmtTs(c.at)+'</td><td><button class="btn danger sm" data-delcg="'+esc(c.id)+'">删除</button></td></tr>';
    }).join('');
  });
}
$('#noticeSend').addEventListener('click',function(){
  jsend('POST','/api/notice',{text:$('#noticeIn').value.trim()}).then(function(d){if(d.ok){toast('公告已更新');renderLogs()}else{toast(d.error||'失败')}});
});
$('#noticeClear').addEventListener('click',function(){
  $('#noticeIn').value='';
  jsend('POST','/api/notice',{text:''}).then(function(d){if(d.ok){toast('公告已撤下');renderLogs()}});
});
$('#boardTable').addEventListener('click',function(e){
  var b=e.target.closest('[data-delbd]');
  if(!b)return;
  if(!confirm('删除这条留言？'))return;
  jsend('DELETE','/api/board',{id:b.dataset.delbd}).then(function(d){if(d.ok){toast('已删除');renderSocial();renderLogs()}else{toast(d.error||'失败')}});
});
$('#cgTable').addEventListener('click',function(e){
  var b=e.target.closest('[data-delcg]');
  if(!b)return;
  if(!confirm('删除这条贡献记录？'))return;
  jsend('DELETE','/api/contrib/'+b.dataset.delcg).then(function(d){if(d.ok){toast('已删除');renderSocial();renderLogs()}else{toast(d.error||'失败')}});
});

/* ===== 备份数据 ===== */
function renderBackups(){
  jget('/api/admin/backups').then(function(d){
    var list=d.backups||[];
    if(!list.length){$('#backupList').innerHTML='<div class="empty">暂无备份（服务启动后自动生成）</div>';return}
    $('#backupList').innerHTML=list.map(function(b){
      return '<div class="hist-item"><b style="flex:1">'+esc(b.name)+'</b><span style="color:var(--ink-500)">'+(b.size?(Number(b.size)/1024/1024).toFixed(2)+' MB':'')+'</span><button class="btn line sm" data-dlbackup="'+esc(b.name)+'">下载</button></div>';
    }).join('');
  });
}

$('#backupList').addEventListener('click',function(e){
  var b=e.target.closest('[data-dlbackup]');
  if(!b)return;
  fetch('/api/admin/backups/'+encodeURIComponent(b.dataset.dlbackup),{headers:{'X-Access-Key':key()}}).then(function(r){
    if(!r.ok){toast('下载失败');return}
    return r.blob().then(function(blob){
      var a=document.createElement('a');a.href=URL.createObjectURL(blob);a.download=b.dataset.dlbackup;a.click();
      toast('备份已开始下载');
    });
  }).catch(function(){toast('下载失败')});
});
$('#backupNow').addEventListener('click',function(){
  var btn=this;btn.disabled=true;btn.textContent='备份中…';
  jsend('POST','/api/admin/backup').then(function(d){
    btn.disabled=false;btn.textContent='⚡ 立即备份';
    if(d.ok){toast('快照已生成');renderBackups();renderLogs()}else{toast(d.error||'备份失败')}
  });
});

/* ===== 操作日志 ===== */
document.addEventListener("click",function(e){
  if(e.target.id!=="exportLogs")return;
  var rows=document.querySelectorAll("#logTable tbody tr");
  if(!rows.length){toast("暂无日志");return}
  var csv="\ufeff时间,操作,详情\n";
  rows.forEach(function(tr){
    var tds=tr.querySelectorAll("td");
    if(tds.length>=3){
      csv+='"'+tds[0].textContent.trim()+'","'+tds[1].textContent.trim().replace(/"/g,'""')+'","'+tds[2].textContent.trim().replace(/"/g,'""')+'"\n';
    }
  });
  var blob=new Blob([csv],{type:"text/csv;charset=utf-8"});
  var a=document.createElement("a");a.href=URL.createObjectURL(blob);a.download="操作日志.csv";a.click();URL.revokeObjectURL(a.href);
});
function filterLogs(){
  var fl=document.getElementById("logFilter");
  var ls=document.getElementById("logSearch");
  var mode=fl?fl.value:"";
  var q=ls?ls.value.trim().toLowerCase():"";
  var rows=document.querySelectorAll("#logTable tbody tr");
  rows.forEach(function(tr){
    var txt=tr.textContent;
    var matchType=!mode||txt.indexOf(mode)>=0;
    var matchQ=!q||txt.toLowerCase().indexOf(q)>=0;
    tr.style.display=(matchType&&matchQ)?"":"none";
  });
}
document.addEventListener("input",function(e){if(e.target.id==="logSearch")filterLogs()});
document.addEventListener("change",function(e){if(e.target.id==="logFilter")filterLogs()});
function renderLogs(){
  jget('/api/admin/logs').then(function(d){
    var list=d.logs||[];
    $('#logEmpty').style.display=list.length?'none':'block';
    $('#logTable').querySelector('tbody').innerHTML=list.slice(0,120).map(function(l){
      return '<tr><td style="white-space:nowrap;color:var(--ink-500)">'+fmtTs(l.at)+'</td><td><span class="tag g">'+esc(l.op)+'</span></td><td>'+esc(l.detail)+'</td></tr>';
    }).join('');
  });
}

/* ===== 启动 ===== */
function boot(){
  renderHome();renderWb();renderFilesAdmin();renderSx();renderKb();renderSocial();renderBackups();renderLogs();renderScoreMini();renderFileMini();
}
</script>
</body>
</html>
`
