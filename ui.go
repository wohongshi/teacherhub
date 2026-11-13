package main

// ui.go —— 本地配置 WebUI（teacherhub ui）：云端地址 / 教师姓名 / 统一访问 key。
// 只监听 127.0.0.1，配置存 ~/.teacherhub/config.json（0600）。

import "fmt"

func configPageHTML(c localConfig) string {
	return fmt.Sprintf(`<!doctype html>
<html lang="zh-CN"><head><meta charset="utf-8">
<meta name="viewport" content="width=device-width,initial-scale=1">
<title>教师工作台 · 配置</title>
<style>
:root{--paper:#f4f2ec;--panel:#fff;--ink:#22282e;--muted:#6b7280;--line:#e3dfd4;--accent:#15616d;--ok:#2e7d4f;--danger:#c2382e}
*{box-sizing:border-box;margin:0;padding:0}
body{font-family:"PingFang SC","Microsoft YaHei",sans-serif;background:var(--paper);color:var(--ink);min-height:100vh}
header{background:#12333a;color:#f2f6f6;padding:18px 22px}
header h1{font-size:21px;font-weight:700}
header .sub{font-size:12.5px;opacity:.75;margin-top:3px}
main{max-width:560px;margin:22px auto;padding:0 16px}
.card{background:var(--panel);border:1px solid var(--line);border-radius:14px;padding:20px 22px;box-shadow:0 2px 10px rgba(34,40,46,.07)}
.field{margin-bottom:16px}
label{display:block;font-size:14px;font-weight:600;margin-bottom:7px}
input{width:100%%;min-height:44px;font-size:15px;padding:9px 12px;border:1px solid var(--line);border-radius:10px;font-family:inherit}
input:focus{outline:2px solid var(--accent);border-color:transparent}
.hint{font-size:12.5px;color:var(--muted);margin-top:6px;line-height:1.6}
.row{display:flex;gap:10px;margin-top:18px;flex-wrap:wrap}
button{min-height:44px;padding:0 22px;font-size:15px;font-weight:600;border-radius:10px;cursor:pointer;font-family:inherit;border:1px solid var(--line);background:#fff}
button.primary{background:var(--accent);border-color:var(--accent);color:#fff}
button.primary:hover{background:#12525c}
#msg{margin-top:14px;font-size:14px;font-weight:600}
#msg.ok{color:var(--ok)}#msg.err{color:var(--danger)}
footer{text-align:center;color:var(--muted);font-size:12px;padding:18px}
</style></head><body>
<header><h1>教师工作台 · 配置</h1><div class="sub">本地插件设置页 · 只存在你自己电脑上</div></header>
<main>
  <div class="card">
    <div class="field">
      <label for="cloud">云端地址</label>
      <input id="cloud" value="%s" placeholder="https://hub.example.com">
      <div class="hint">老师群里发的那个地址（管理员提供）。</div>
    </div>
    <div class="field">
      <label for="name">教师姓名</label>
      <input id="name" value="%s" placeholder="如：张老师">
      <div class="hint">贡献和工作台都用这个名字署名。</div>
    </div>
    <div class="field">
      <label for="key">访问 key（全员统一一把）</label>
      <input id="key" value="%s" placeholder="管理员发的 key">
      <div class="hint">这把钥匙=进群凭证，别发给学生。</div>
    </div>
    <div class="row">
      <button class="primary" id="save">保存</button>
      <button id="test">测试连接</button>
    </div>
    <div id="msg"></div>
  </div>
</main>
<footer>教师工作台 · 零账号 · 配置只存本机</footer>
<script>
function val(id){return document.getElementById(id).value.trim()}
function msg(t,cls){var m=document.getElementById('msg');m.textContent=t;m.className=cls}
document.getElementById('save').onclick=async function(){
  var body=JSON.stringify({cloud:val('cloud'),name:val('name'),key:val('key')});
  var r=await fetch('/save',{method:'POST',body:body});
  if(r.ok){msg('已保存 ✓','ok')}else{msg('保存失败','err')}
};
document.getElementById('test').onclick=async function(){
  msg('测试中…','');
  var body=JSON.stringify({cloud:val('cloud'),key:val('key')});
  var r=await fetch('/test',{method:'POST',body:body});
  var j=await r.json();
  if(j.ok){msg('连接成功 ✓','ok')}else{msg('连接失败：'+j.error,'err')}
};
</script>
</body></html>`, htmlEscape(c.Cloud), htmlEscape(c.Name), htmlEscape(c.Key))
}

// htmlEscape 配置值回填表单的最小转义。
func htmlEscape(s string) string {
	out := []rune{}
	for _, r := range s {
		switch r {
		case '&':
			out = append(out, []rune("&amp;")...)
		case '<':
			out = append(out, []rune("&lt;")...)
		case '>':
			out = append(out, []rune("&gt;")...)
		case '"':
			out = append(out, []rune("&quot;")...)
		default:
			out = append(out, r)
		}
	}
	return string(out)
}
