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

echo "== 统一 key 鉴权 =="
R=$(curl -s "$BASE/api/ping?key=$KEY")
echo "$R" | jassert 'assert d["version"]=="1.0.0", d' "带 key ping 通"
R=$(curl -s "$BASE/api/ping")
echo "$R" | jerr "key" "无 key 拒绝"
R=$(curl -s -H "X-Access-Key: 错的" "$BASE/api/ping")
echo "$R" | jerr "key" "错 key 拒绝"

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
[ -n "$WBID" ] && echo "$LINK" | grep -q "/w/$WBID?key=" && ok "推送产物 + 下发链接格式" || bad "推送异常: $R"
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

echo "== 下发链接 + 统计页 =="
R=$(curl -s "$LINK")
echo "$R" | grep -q "钠与水实验工作台内容" && ok "链接浏览器直看产物" || bad "链接打开异常"
R=$(curl -s "$BASE/api/workbenches/$WBID/file?key=$KEY")
echo "$R" | grep -q "钠与水实验工作台内容" && ok "产物下载字节一致" || bad "下载异常"
R=$(curl -s "$BASE/w/$WBID?key=错的")
[ -n "$R" ] && ! echo "$R" | grep -q "钠与水实验工作台内容" && ok "错 key 看不了产物" || bad "错 key 放行（严重）"
R=$(curl -s "$BASE/stats?key=$KEY")
echo "$R" | grep -q "张老师" && echo "$R" | grep -q "钠与水反应实验工作台" && ok "统计页渲染（贡献+产物）" || bad "统计页异常"

P=$(grep -c '^P$' "$CNT" 2>/dev/null); F=$(grep -c '^F$' "$CNT" 2>/dev/null)
echo
echo "probe_teacherhub: ${P:-0} 通过 / ${F:-0} 失败"
[ "${F:-0}" -eq 0 ] || exit 1
