#!/usr/bin/env bash
# teacherhub 云端探针：统一 key 鉴权 / 贡献统计 / 产物库 / 相似检索 / 下发链接 / 统计页。
# 前置：go build -o /tmp/teacherhub-bin .
set -u -o pipefail
BIN=/tmp/teacherhub-bin
BASE=http://127.0.0.1:18771
KEY=probe-key-统一
D=/tmp/th-probe
CNT=/tmp/th-count
ok()  { echo "  ✓ $1"; echo P >> "$CNT"; }
bad() { echo "  ✗ $1"; echo F >> "$CNT"; }
jassert() { python3 -c "
import json,sys
d=json.load(sys.stdin)
assert d.get('ok')==True, d
$1" 2>/dev/null && ok "$2" || bad "$2"; }
jerr() { python3 -c "
import json,sys
d=json.load(sys.stdin)
assert d.get('ok')==False and '''$1''' in d.get('error',''), d" 2>/dev/null && ok "$2" || bad "$2"; }

[ -x "$BIN" ] || { echo "缺少 $BIN（先 go build -o $BIN .）"; exit 2; }
SP=""
cleanup() { [ -n "$SP" ] && kill "$SP" 2>/dev/null; wait "$SP" 2>/dev/null; rm -rf "$D"; }
trap cleanup EXIT INT TERM
rm -rf "$D"; : > "$CNT"

"$BIN" serve --addr 127.0.0.1:18771 --data "$D" --key "$KEY" > /tmp/th-probe.log 2>&1 &
SP=$!
for _ in $(seq 1 30); do curl -s --max-time 1 "$BASE/api/ping?key=$KEY" 2>/dev/null | grep -q '"ok":true' && break
  kill -0 "$SP" 2>/dev/null || { echo "服务起不来"; tail -5 /tmp/th-probe.log; exit 2; }
  sleep 0.5; done

echo "== 访问政策：读免 key / 写留 key =="
R=$(curl -s "$BASE/api/ping?key=$KEY")
echo "$R" | jassert 'assert d["version"]=="1.0.0", d' "带 key ping 通"
R=$(curl -s "$BASE/api/ping")
echo "$R" | jassert 'assert d["version"]=="1.0.0", d' "读接口免 key（打开即看）"
R=$(curl -s -X POST "$BASE/api/contrib" -H 'Content-Type: application/json' --data-raw '{"teacher":"x","text":"y"}')
echo "$R" | jerr "key" "写操作无 key 拒绝"
R=$(curl -s -X POST "$BASE/api/workbenches" -H 'Content-Type: application/json' --data-raw '{"title":"x","content":"y"}')
echo "$R" | jerr "key" "产物推送无 key 拒绝"

echo "== 贡献统计 =="
R=$(curl -s -X POST "$BASE/api/contrib?key=$KEY" -H 'Content-Type: application/json' --data-raw '{"teacher":"张老师","text":"添加了《钠与水反应》到化学实验库","kb":"化学实验库","item":"钠与水反应"}')
echo "$R" | jassert 'assert d["id"].startswith("cg_"), d' "贡献上报"
curl -s -X POST "$BASE/api/contrib?key=$KEY" -H 'Content-Type: application/json' --data-raw '{"teacher":"张老师","text":"补充了《铁丝燃烧》的实验视频"}' >/dev/null
curl -s -X POST "$BASE/api/contrib?key=$KEY" -H 'Content-Type: application/json' --data-raw '{"teacher":"李老师","text":"添加了《丁达尔效应》演示"}' >/dev/null
R=$(curl -s "$BASE/api/contrib?key=$KEY")
echo "$R" | python3 -c "
import json,sys
cs=json.load(sys.stdin)['contribs']
assert len(cs)>=3 and cs[0]['at']>=cs[-1]['at'], cs" 2>/dev/null \
  && ok "贡献清单（新→旧）" || bad "贡献清单异常: $R"
R=$(curl -s "$BASE/api/stats?key=$KEY")
echo "$R" | python3 -c "
import json,sys
d=json.load(sys.stdin)
by={t['teacher']:t for t in d['by_teacher']}
assert by['张老师']['contribs']==2 and by['李老师']['contribs']==1, d
assert d['total_contribs']>=3, d" 2>/dev/null \
  && ok "按教师聚合统计" || bad "统计聚合异常: $R"

echo "== 产物库 + 相似检索 =="
printf '<html><body>钠与水实验工作台内容</body></html>' > /tmp/th-a.html
R=$(curl -s -X POST "$BASE/api/workbenches?key=$KEY" -H 'Content-Type: application/json' --data-raw "{\"title\":\"钠与水反应实验工作台\",\"desc\":\"浮熔游响红\",\"tags\":[\"化学\",\"实验\"],\"keywords\":[\"钠\",\"水\",\"反应\"],\"teacher\":\"张老师\",\"content\":\"$(cat /tmp/th-a.html)\"}")
WBID=$(echo "$R" | python3 -c "import json,sys;d=json.load(sys.stdin);print(d['id'])" 2>/dev/null)
LINK=$(echo "$R" | python3 -c "import json,sys;print(json.load(sys.stdin)['link'])" 2>/dev/null)
[ -n "$WBID" ] && [[ "$LINK" == *"/w/$WBID"* ]] && ! [[ "$LINK" == *"key="* ]] && ok "推送产物 + 下发链接格式（免 key）" || bad "推送异常: $R"
R=$(curl -s -G "$BASE/api/workbenches/search?key=$KEY" --data-urlencode "q=钠与水 反应")
echo "$R" | python3 -c "
import json,sys
m=json.load(sys.stdin)['matches']
assert len(m)>=1 and m[0]['workbench']['title']=='钠与水反应实验工作台' and m[0]['score']>=0.5, m" 2>/dev/null \
  && ok "相似检索命中（标题一致）" || bad "检索异常: $R"
R=$(curl -s -G "$BASE/api/workbenches/search?key=$KEY" --data-urlencode "q=量子引力与黑洞蒸发模拟")
echo "$R" | python3 -c "
import json,sys
assert json.load(sys.stdin)['matches']==[], '不该命中'" 2>/dev/null \
  && ok "不相关内容零命中" || bad "误命中: $R"
printf '<html><body>铁丝燃烧工作台内容</body></html>' > /tmp/th-b.html
curl -s -X POST "$BASE/api/workbenches?key=$KEY" -H 'Content-Type: application/json' --data-raw "{\"title\":\"铁丝在氧气中燃烧工作台\",\"tags\":[\"化学\",\"实验\"],\"teacher\":\"李老师\",\"content\":\"$(cat /tmp/th-b.html)\"}" >/dev/null
R=$(curl -s -G "$BASE/api/workbenches/search?key=$KEY" --data-urlencode "q=化学 实验 工作台")
echo "$R" | python3 -c "
import json,sys
m=json.load(sys.stdin)['matches']
assert len(m)>=2 and m[0]['score']>=m[-1]['score'], m" 2>/dev/null \
  && ok "多命中按相似度排序" || bad "排序异常: $R"

echo "== 下发链接 + 门户（浏览全免 key） =="
R=$(curl -s "$LINK")
[[ "$R" == *"/w/$WBID/raw"* ]] && [[ "$R" == *"钠与水反应实验工作台"* ]] && ok "查看页顶栏包装（免 key）" || bad "查看页异常: ${R:0:80}"
R=$(curl -s "$BASE/w/$WBID/raw")
[[ "$R" == *"钠与水实验工作台内容"* ]] && ok "原文直看（免 key）" || bad "原文异常"
R=$(curl -s "$BASE/api/workbenches/$WBID/file")
[[ "$R" == *"钠与水实验工作台内容"* ]] && ok "产物下载字节一致（免 key）" || bad "下载异常"
R=$(curl -s "$BASE/stats")
[[ "$R" == *"化学学科园地"* ]] && [[ "$R" == *'id="forest"'* ]] && curl -s "$BASE/api/workbenches" | grep -q "钠与水反应实验工作台" && curl -s "$BASE/api/stats" | grep -q "张老师" && ok "门户页渲染（免 key，SPA 壳+API 数据）" || bad "门户异常"

# ---- 功能增强断言：热度计数 / PUT 更新 / 门户筛选 ----
V0=$(curl -s "$BASE/api/workbenches" | python3 -c "import json,sys;d=json.load(sys.stdin);print([w['views'] for w in d['workbenches'] if w['id']=='$WBID'][0])" 2>/dev/null)
curl -s "$BASE/w/$WBID" -o /dev/null
V1=$(curl -s "$BASE/api/workbenches" | python3 -c "import json,sys;d=json.load(sys.stdin);print([w['views'] for w in d['workbenches'] if w['id']=='$WBID'][0])" 2>/dev/null)
[ -n "$V1" ] && [ "$V1" -gt "$V0" ] && ok "查看页热度计数 +1（$V0→$V1）" || bad "查看页热度计数 +1（$V0→$V1）"
curl -s -X PUT "$BASE/api/workbenches/$WBID" -H 'Content-Type: application/json' -H "X-Access-Key: $KEY" \
  --data-raw '{"desc":"PUT 更新测试","content":"<html><body>updated</body></html>"}' | jassert 'assert d.get("id"), d' "PUT 更新产物须 key 且成功"
curl -s "$BASE/api/workbenches/$WBID/file" | grep -q 'updated' && ok "PUT 后内容已替换" || bad "PUT 后内容已替换"
curl -s -X PUT "$BASE/api/workbenches/$WBID" -H 'Content-Type: application/json' --data-raw '{"desc":"x"}' | jerr key "PUT 无 key 被拒"
curl -s "$BASE/" | grep -q 'class="lab-toolbar"' && curl -s "$BASE/" | grep -q 'id="labChips"' && ok "门户含筛选工具条" || bad "门户含筛选工具条"

# ---- MCP 端点断言 ----
curl -s -X POST "$BASE/mcp" -H 'Content-Type: application/json' --data-raw '{"jsonrpc":"2.0","id":1,"method":"initialize","params":{}}' | python3 -c "import json,sys;d=json.load(sys.stdin);assert d['result']['serverInfo']['name']=='teacherhub',d" 2>/dev/null && ok "MCP 握手（initialize）" || bad "MCP 握手"
curl -s -X POST "$BASE/mcp" -H 'Content-Type: application/json' --data-raw '{"jsonrpc":"2.0","id":2,"method":"tools/list","params":{}}' | python3 -c "import json,sys;d=json.load(sys.stdin);ns={t['name'] for t in d['result']['tools']};assert len(ns)==13 and {'rate_workbench','list_backups'}<=ns,ns" 2>/dev/null && ok "MCP 工具清单十三件（含评分/备份）" || bad "MCP 工具清单"
curl -s -X POST "$BASE/mcp" -H 'Content-Type: application/json' --data-raw '{"jsonrpc":"2.0","id":3,"method":"tools/call","params":{"name":"push_workbench","arguments":{"title":"x","content":"y"}}}' | python3 -c "import json,sys;d=json.load(sys.stdin);assert d['result'].get('isError')==True,d" 2>/dev/null && ok "MCP 写工具无 key 拒" || bad "MCP 写工具鉴权"

# ---- 预览图 / 留言墙 / RSS 断言 ----
PVP=$(python3 -c "import base64;png=base64.b64decode('iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mNk+M9QDwADhgGAWjR9awAAAABJRU5ErkJggg==');print('data:image/png;base64,'+base64.b64encode(png*200).decode())")
R=$(curl -s -X POST "$BASE/api/workbenches?key=$KEY" -H 'Content-Type: application/json' --data-raw "{\"title\":\"预览测试\",\"content\":\"<html>p</html>\",\"preview\":\"$PVP\"}")
WPID=$(echo "$R" | python3 -c "import json,sys;print(json.load(sys.stdin)['id'])" 2>/dev/null)
curl -s -o /dev/null -w '%{http_code}' "$BASE/api/workbenches/$WPID/preview" | grep -q 200 && ok "预览图上传并可取（200）" || bad "预览图可取"
curl -s -X POST "$BASE/api/board" -H 'Content-Type: application/json' --data-raw '{"name":"探针","text":"留言断言"}' | python3 -c "import json,sys;assert json.load(sys.stdin).get('ok')==True" 2>/dev/null && ok "留言发布成功（免 key）" || bad "留言发布"
curl -s -X POST "$BASE/api/board" -H 'Content-Type: application/json' --data-raw '{"name":"探针","text":"连发"}' | python3 -c "import json,sys;assert json.load(sys.stdin).get('ok')==False" 2>/dev/null && ok "留言限速生效（60s/条）" || bad "留言限速"
curl -s "$BASE/feed.xml" | grep -q '<rss' && ok "RSS feed 输出" || bad "RSS feed"

# ---- 公告 / 点赞断言 ----
curl -s -X POST "$BASE/api/notice" -H 'Content-Type: application/json' --data-raw '{"text":"公告测试"}' | python3 -c "import json,sys;assert json.load(sys.stdin).get('ok')==False" 2>/dev/null && ok "公告无 key 拒" || bad "公告鉴权"
curl -s -X POST "$BASE/api/notice?key=$KEY" -H 'Content-Type: application/json' --data-raw '{"text":"公告测试"}' | python3 -c "import json,sys;assert json.load(sys.stdin).get('ok')==True" 2>/dev/null && ok "公告发布（管理 key）" || bad "公告发布"
curl -s "$BASE/api/notice" | python3 -c "import json,sys;assert json.load(sys.stdin).get('notice')=='公告测试'" 2>/dev/null && ok "公告读取（免 key）" || bad "公告读取"
BID=$(curl -s "$BASE/api/board" | python3 -c "import json,sys;d=json.load(sys.stdin);print(d['board'][0]['id'] if d['board'] else '')" 2>/dev/null)
curl -s -X POST "$BASE/api/board/like" -H 'Content-Type: application/json' --data-raw "{\"id\":\"$BID\"}" | python3 -c "import json,sys;assert json.load(sys.stdin).get('ok')==True" 2>/dev/null && ok "留言点赞成功" || bad "留言点赞"
curl -s "$BASE/api/board" | python3 -c "import json,sys;d=json.load(sys.stdin);m=[x for x in d['board'] if x['id']=='$BID'];assert m and m[0]['likes']>=1,m" 2>/dev/null && ok "点赞计数累加" || bad "点赞计数"

# ---- 版本历史 / 留言删除 / 今日浏览 断言 ----
curl -s -X PUT "$BASE/api/workbenches/$WBID" -H 'Content-Type: application/json' -H "X-Access-Key: $KEY" --data-raw '{"content":"<html><body>v2</body></html>"}' >/dev/null
HIST=$(curl -s "$BASE/api/workbenches/$WBID/history" | python3 -c "import json,sys;d=json.load(sys.stdin);print(len(d['history']))" 2>/dev/null)
[ -n "$HIST" ] && [ "$HIST" -ge 1 ] && ok "版本历史备份（$HIST 份）" || bad "版本历史"
TS=$(curl -s "$BASE/api/workbenches/$WBID/history" | python3 -c "import json,sys;print(json.load(sys.stdin)['history'][0]['ts'])" 2>/dev/null)
curl -s -X POST "$BASE/api/workbenches/$WBID/restore" -H 'Content-Type: application/json' -H "X-Access-Key: $KEY" --data-raw "{\"ts\":$TS}" | python3 -c "import json,sys;assert json.load(sys.stdin).get('ok')==True" 2>/dev/null && ok "历史版本恢复" || bad "版本恢复"
curl -s -X DELETE "$BASE/api/board" -H 'Content-Type: application/json' -H "X-Access-Key: $KEY" --data-raw "{\"id\":\"$BID\"}" | python3 -c "import json,sys;assert json.load(sys.stdin).get('ok')==True" 2>/dev/null && ok "留言删除（管理 key）" || bad "留言删除"
curl -s "$BASE/api/board" | python3 -c "import json,sys;d=json.load(sys.stdin);assert not [x for x in d['board'] if x['id']=='$BID'],d" 2>/dev/null && ok "删除后已移除" || bad "删除核验"
curl -s -X POST "$BASE/mcp" -H 'Content-Type: application/json' --data-raw '{"jsonrpc":"2.0","id":9,"method":"tools/call","params":{"name":"get_stats","arguments":{}}}' | python3 -c "import json,sys;d=json.load(sys.stdin);assert 'workbenches' in d['result']['content'][0]['text'],d" 2>/dev/null && ok "MCP get_stats" || bad "MCP get_stats"
# ---- 管理面板断言 ----
curl -s -o /dev/null -w '%{http_code}' "$BASE/admin" | grep -q 200 && ok "管理面板页可开" || bad "管理面板页"
curl -s "$BASE/api/admin/ping" | python3 -c "import json,sys;assert json.load(sys.stdin).get('ok')==False" 2>/dev/null && ok "admin ping 无 key 拒" || bad "admin ping 鉴权"
curl -s -H "X-Access-Key: $KEY" "$BASE/api/admin/ping" | python3 -c "import json,sys;d=json.load(sys.stdin);assert d.get('ok')==True and 'daily' in d,d" 2>/dev/null && ok "admin ping 带 key 返回每日浏览" || bad "admin ping"

# ---- 产物删除 / 备份导出 断言 ----
curl -s -X DELETE "$BASE/api/workbenches/$WPID" -H 'Content-Type: application/json' -H "X-Access-Key: $KEY" --data-raw '{}' | python3 -c "import json,sys;assert json.load(sys.stdin).get('ok')==True" 2>/dev/null && ok "产物删除（管理 key）" || bad "产物删除"
curl -s -o /dev/null -w '%{http_code}' "$BASE/api/workbenches/$WPID/file" | grep -q 404 && ok "删除后产物文件已下线" || bad "删除核验"
curl -s -H "X-Access-Key: $KEY" -o /tmp/th-export.zip -w '%{http_code}' "$BASE/api/admin/export" | grep -q 200 && ok "备份导出 zip（200）" || bad "备份导出"
python3 -c "import zipfile;z=zipfile.ZipFile('/tmp/th-export.zip');assert 'store.json' in z.namelist(),z.namelist()" 2>/dev/null && ok "备份 zip 含 store.json" || bad "备份内容"
# ---- 公开统计 / PWA / QR 断言 ----
curl -s "$BASE/api/stats" | python3 -c "import json,sys;d=json.load(sys.stdin);assert d.get('ok')==True and 'total_workbenches' in d and 'daily' in d.get('totals',{}),d" 2>/dev/null && ok "公开统计 API（免 key）" || bad "公开统计"
curl -s -o /dev/null -w '%{http_code}' "$BASE/manifest.webmanifest" | grep -q 200 && ok "PWA manifest" || bad "PWA manifest"
curl -s -o /dev/null -w '%{http_code}' "$BASE/sw.js" | grep -q 200 && ok "PWA service worker" || bad "PWA sw"
curl -s -o /dev/null -w '%{http_code}' "$BASE/qr.js" | grep -q 200 && ok "QR 编码器脚本" || bad "qr.js"

# ---- 下架/上架 / 自动备份 断言 ----
curl -s -X POST "$BASE/api/workbenches/$WBID/hidden" -H 'Content-Type: application/json' -H "X-Access-Key: $KEY" --data-raw '{"hidden":true}' | python3 -c "import json,sys;assert json.load(sys.stdin).get('ok')==True" 2>/dev/null && ok "产物下架" || bad "产物下架"
curl -s "$BASE/api/workbenches" | python3 -c "import json,sys;d=json.load(sys.stdin);assert not [w for w in d['workbenches'] if w['id']=='$WBID'],d" 2>/dev/null && ok "下架后列表隐藏" || bad "下架隐藏"
curl -s "$BASE/api/workbenches?all=1" | python3 -c "import json,sys;d=json.load(sys.stdin);assert [w for w in d['workbenches'] if w['id']=='$WBID'],d" 2>/dev/null && ok "all=1 可见（管理视角）" || bad "all 视角"
curl -s -X POST "$BASE/api/workbenches/$WBID/hidden" -H 'Content-Type: application/json' -H "X-Access-Key: $KEY" --data-raw '{"hidden":false}' >/dev/null && ok "产物上架恢复" || bad "上架恢复"
ls "$D/backups/"backup-*.zip >/dev/null 2>&1 && ok "自动备份 zip 已生成" || bad "自动备份"
# ---- 评分 / 备份清单 断言 ----
curl -s -o /dev/null -w '%{http_code}' -X POST "$BASE/api/workbenches/$WBID/rate" -H 'Content-Type: application/json' --data-raw '{"stars":6}' | grep -q 400 && ok "评分越界拒绝（400）" || bad "评分越界"
curl -s "$BASE/api/workbenches/$WBID" | python3 -c "import json,sys;w=json.load(sys.stdin)['workbench'];assert w['rate']==0 and w['rate_num']==0,w" 2>/dev/null && ok "越界评分未计分" || bad "越界未计分"
curl -s -X POST "$BASE/api/workbenches/$WBID/rate" -H 'Content-Type: application/json' --data-raw '{"stars":5}' | python3 -c "import json,sys;d=json.load(sys.stdin);assert d.get('ok')==True and d.get('rate')==5.0,d" 2>/dev/null && ok "评分成功（免 key，均分 5.0）" || bad "评分"
curl -s -X POST "$BASE/api/workbenches/$WBID/rate" -H 'Content-Type: application/json' --data-raw '{"stars":3}' | python3 -c "import json,sys;assert json.load(sys.stdin).get('already')==True" 2>/dev/null && ok "评分限一评（二评 already）" || bad "评分限一"
curl -s "$BASE/api/workbenches/$WBID" | python3 -c "import json,sys;w=json.load(sys.stdin)['workbench'];assert w['rate']==5.0 and w['rate_num']==1,w" 2>/dev/null && ok "重复评分不计分（5.0/1 人）" || bad "重复评分被计分"
curl -s "$BASE/api/admin/backups" | python3 -c "import json,sys;assert json.load(sys.stdin).get('ok')==False" 2>/dev/null && ok "备份清单无 key 拒" || bad "备份清单鉴权"
curl -s --path-as-is -o /tmp/th-trav.txt -w '%{http_code}' -H "X-Access-Key: $KEY" "$BASE/api/admin/backups/../store.json" | grep -qE '^(30[0-9]|40[0-9])$' && ! grep -q '"key"' /tmp/th-trav.txt && ok "备份下载拒绝路径穿越（不泄露）" || bad "备份路径穿越"
curl -s -H "X-Access-Key: $KEY" "$BASE/api/admin/backups" | python3 -c "import json,sys;d=json.load(sys.stdin);assert d.get('ok')==True and len(d.get('backups',[]))>=1,d" 2>/dev/null && ok "备份清单可列（≥1 份）" || bad "备份清单"
BN=$(curl -s -H "X-Access-Key: $KEY" "$BASE/api/admin/backups" | python3 -c "import json,sys;print(json.load(sys.stdin)['backups'][0]['name'])" 2>/dev/null)
[ -n "$BN" ] && curl -s -H "X-Access-Key: $KEY" -o /tmp/th-bn.zip -w '%{http_code} %{size_download}' "$BASE/api/admin/backups/$BN" | grep -qE '^200 [1-9][0-9]*$' && ok "备份单份下载（200，非空）" || bad "备份下载（BN=${BN:-空}）"
curl -s -X POST "$BASE/mcp" -H 'Content-Type: application/json' --data-raw '{"jsonrpc":"2.0","id":4,"method":"tools/call","params":{"name":"list_backups","arguments":{}}}' | python3 -c "import json,sys;assert json.load(sys.stdin)['result'].get('isError')==True" 2>/dev/null && ok "MCP 备份清单无 key 拒" || bad "MCP 备份清单鉴权"
P=$(grep -c '^P$' "$CNT" 2>/dev/null); F=$(grep -c '^F$' "$CNT" 2>/dev/null)
echo
echo "probe_teacherhub: ${P:-0} 通过 / ${F:-0} 失败"
[ "${F:-0}" -eq 0 ] || exit 1
