# teacherhub ocr 代码评审处置表（第二轮）

**评审范围**：`6edc7d8..cad59fe`（36 提交，两文件 +1718/-58）
**评审工具**：open-code-review (ocr) v1.12.9，`--from 6edc7d8 --to HEAD --audience agent`
**评审时间**：2026-11-17
**意见总数**：23 条（admin.go 8 / web_dashboard.html 15），全部核实为真、全部已修，无误报无设计如此
**回归**：probe_teacherhub **107/107**（顺手修掉上轮遗留 #7 恒过断言 + 新增 reset 3 断言），go vet 干净，node --check 过

---

## 严重度分布

| 严重度 | 条数 | 处置 |
|---|---|---|
| CRITICAL | 2 | 全修 |
| HIGH | 3 | 全修 |
| MEDIUM | 15 | 全修 |
| LOW | 3 | 全修 |

## 已修明细（23 条）

| # | 严重度 | 位置 | 问题 | 修复 |
|---|--------|------|------|------|
| 1 | CRIT | admin:581 | 「重置数据」假成功：POST /api/notice 只清公告却报"成绩/留言已清空" | 新增真 `POST /api/admin/reset`（cloud.go，key 校验，清成绩/留言/公告/文件记录+实体，实验与课标保留），admin 调真端点 + 带计数反馈；probe 补 3 断言 |
| 2 | CRIT | web:2071 | `kb_claims` 未声明变量（应 `KB_CLAIMS`），周报生成 ReferenceError 整段静默失败 | 改 `typeof KB_CLAIMS!=="undefined"` 守卫 |
| 3 | HIGH | admin:518 | `el.innerHTML=html` 拷贝残留（`html` 不存在），每次 boot ReferenceError | 删残留两行 |
| 4 | HIGH | admin:551/640 | `/api/admin/stats` 不存在（应 `/api/admin/ping`），存储条/系统状态永远 "—" | 两处改 ping；cloud.go ping 响应补 `score_count` |
| 5 | HIGH | web:1727 | 百分位倒置：above() 是"比你高的人占比"，文案"约超过 X%"反了；v≥150→0、v≤0→100 也反 | pts 源头翻转为超越%（`100-above`，曲线变递增），special cases 对调，显示语义全对 |
| 6 | MED | admin:399/587/608 | exportCsvBtn 按钮×2 + 同 handler×2 → 双请求双下载 | 按钮去重 + 只留一个 handler |
| 7 | MED | admin:593 | CSV 无转义（引号不翻倍）+ 公式注入（`=`/`+`/`-`/`@` 开头）无防护 | csvNum/csvStr 分离：数值转义、字符串加 `'` 前缀防公式注入 |
| 8 | MED | admin:596 | 导出读 `ex.segments` 但列表 API 不返回 → 分数段导出恒空 | Promise.all 逐场拉 detail 取 segments |
| 9 | MED | admin:557 | 存储构成条是硬编码假字节切片冒充测量 | 改真实计数卡片（文件/实验/成绩 N + 真实总占用） |
| 10 | MED | web:2308 | renderCmpEnhance setTimeout(300) 与 #cmpGo async 竞态（innerHTML= 与 += 对撞） | renderCmpEnhance(dA,dB) 改参数式，由 #cmpGo 拿 da/db 同步调用；删 setTimeout 钩子 |
| 11 | MED | web:1891 | 拖拽排序死：`<tr>` 无 data-fid（__dragFile 永不设）且无 draggable；th_file_order 只写不读 | tr 加 `data-fid`+`draggable="true"`；renderFiles 应用手动顺序 |
| 12 | MED | web:2968 | `q.name`/`qn` 未 esc() 注入 DOM（XSS 红线破口），另有 best.qn/worst.qn 两处 | 4 落点全 esc()（含 String() 强转） |
| 13 | MED | web:2570 | 每次切成绩 tab 扇出 4*N+12 个 detail 请求，无缓存无去重 | jgetExam 8 秒 TTL 共享缓存，22 调用点收口（比上轮"设计如此"轻量收敛） |
| 14 | MED | web:2612 | 新图表 Promise 链无 .catch，失败全 unhandled | jgetExam 失败回空骨架（questions/segments/scores:[]），下游走空态分支 |
| 15 | MED | admin:501 | 单分类时 arc 起止同点 SVG 不画 → 空饼图 | frac≥0.999 走整圆 `<circle>` 分支 |
| 16 | MED | web:3454 | 批量删除后用旧 FILES 渲染，已删行还显示 | 成功后 `FILES=FILES.filter(f=>!ids.includes(f.id))` 再渲染 |
| 17 | MED | web:2008 | 分享链接拼 `?exam=` 但无人消费 → 假分享 | 启动段 URLSearchParams 解析，设 `__scoreSel`+go("score") |
| 18 | MED | web:3383 | `window.__wbs` 从未赋值 → 收藏分组死代码；th_fav_tags 只读不写 | fallback `WBS`；dialog 补渲染收藏分组区 + 标签输入框（回车写 th_fav_tags） |
| 19 | MED | web:1864 | Markdown 表格正则坏：吞相邻 <tr>、对未包表格行拼 </table> | 分隔行 `\|---\|` 过滤 + 连续 tr 块整体包装单正则 |
| 20 | MED | web:1313 | doSearch 绑 input 每键存历史（na/nac/nacl 污染） | doSearch(q,commit) 参数化：input 不存、Enter/点历史才存 |
| 21 | LOW | web:2764 | 百分位缓存过期：N<5 早退不重置 `__cumData`，cumGo 拿陈旧数据 | 早退分支清 `__cumData=null` |
| 22 | LOW | web:1893 | 空查询 `new RegExp("()")` 每字符包 `<mark>` DOM 膨胀 | q 空时直接输出 esc(name) 不走高亮 |
| 23 | LOW | web:1358 | 删历史条目后 `doSearch(__searchQ)` 又把刚删的词存回去 | 与 #20 合并修：删历史调用不带 commit 不重存 |

## 上轮遗留测试缺口（本轮顺手补）

| # | 问题 | 修复 |
|---|------|------|
| 上轮 #7 | probe gzip 排除断言对已删 $FX 跑恒过 | 断言前重传真文件拿 $FXG 断言后删，恒过消除 |

---

**验证链**：`go build ./... && go vet ./...` ✅ → `node --check`（JS 提取）✅ → `probe_teacherhub.sh` **107/107** ✅ → Pages 部署 HTTP 200 ✅
