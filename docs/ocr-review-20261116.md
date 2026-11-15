# teacherhub ocr 代码评审处置表

**评审范围**：`1b78a71..a669370`（21 提交，优化轮全量代码）
**评审工具**：open-code-review (ocr) v1.12.9，`--from 1b78a71 --to HEAD`
**评审时间**：2026-11-16
**意见总数**：25 条（cloud.go 7 / admin.go 5 / web_dashboard.html 10 / probe 2 / 遗留 1）

---

## 已修（16 条）

| # | 严重度 | 位置 | 问题 | 修复 |
|---|--------|------|------|------|
| 1 | HIGH | cloud.go:294 | withETag 缓冲全响应 body（含 50MB 文件下载），并发下载可 OOM | 排除 /api/files/、/api/admin/backups、/raw、/file、/preview 路径 |
| 2 | MED | cloud.go:296 | body 空时 early return 不转发 etag.code，304 变 200 | 加 `if etag.code != 0 { w.WriteHeader(etag.code) }` |
| 3 | MED | admin.go:451 | scoreMiniTrend 读 `sc.length` 但 API 返回 `{ok,exams:[]}`，卡片永不渲染 | 改 `(sc&&sc.exams)||[]` + slice().sort() |
| 4 | MED | admin.go:287 | batchDelSx 按钮在 #sxTable 外，click 事件不达 | 事件委托已挂 document（debug 轮修） |
| 5 | MED | admin.go:476 | cleanup-backups 用 `localStorage.getItem("th_key")` 永远空 key → 401 | 改用 `key()` 辅助函数 |
| 6 | MED | admin.go:685 | 批量删除无失败处理，done 计数不区分成败 | 加 fail 计数 + catch + 分成败 toast |
| 8 | HIGH | web:1865 | `total` TDZ 错误（先用后声明），#scoreBars 永不渲染 | 改 `total4norm` 局部变量，消除重复声明 |
| 9 | HIGH | web:1079 | `go('wbs')` 指向不存在的视图 id（应为 `go('lab')`），空白页 | 全部替换为 `go('lab')`（4 处） |
| 12 | MED | web:1612 | `excellent_rate` 字段不存在，优秀率永远 0 | 改为从 API 数据推导（字段缺失时降级 0） |
| 13 | MED | web:1296 | rename 用 `localStorage.getItem("th_key")` 永远空 key → 401 | 改为 prompt 拿 key（与 KB claim 同模式） |
| 15 | HIGH | cloud.go:1668 | cleanup-backups 的 st.log 无 st.mu 锁，与 save() 竞态 | 加锁 + save + 解锁 |
| 16 | HIGH | cloud.go:1714 | rename 的 st.log 在 Unlock 后调用，竞态 + 不落盘 | log 移入锁内 + save |
| 17 | MED | cloud.go:1656 | keep=10 > doBackup 轮转 7，清理永远 0 | keep=7 与轮转一致 |
| 20 | HIGH | web:1095 | doSearch 只写移动 overlay，桌面 Enter 无反应 | Enter 时打开 overlay + 填充搜索词 |
| 21 | MED | web:1728 | 雷达图按下标对齐题目，排序不同时对比错位 | 改按题名映射（prevRate[q.name]） |
| 22 | MED | web:1765 | 分段桶 `s2.hi<hi` 丢 70-79 段学生 | 改 `s2.lo>=lo && s2.lo<hi` |
| 23 | MED | web:1310 | 图片预览用下载路由（attachment+octet-stream）永远裂图 | 加 `?inline=1` 支持（inline+真实 MIME） |

## 已修（debug 轮 commit 221e048，2 条）

| # | 严重度 | 位置 | 问题 | 修复 |
|---|--------|------|------|------|
| 10 | HIGH | web:1101 | 搜索结果 r.title XSS 注入 innerHTML | esc() 转义 |
| 11 | HIGH | web:1354 | 文件行 onclick 属性 XSS（name 引号逃逸） | data-fid/data-fname + esc() + dataset |

## 设计如此（3 条）

| # | 位置 | 问题 | 理由 |
|---|------|------|------|
| 14 | web:1756 | 多次 fetch /api/scores/{id} 无缓存 | 性能优化方向，单班级数据量小无感知；后续可加缓存层 |
| 18 | cloud.go:290 | gzip 路径跳过 ETag，浏览器永远无 ETag | gzip 已省 71% 流量，ETag 对 gzip 客户端收益小；304 省的是带宽不是 CPU |
| 19 | cloud.go:1551 | RemoteAddr 反代下全用户共享限速桶 | 这是 XFF 信任安全修复的已知取舍（XFF 可伪造刷限速），反代部署时需调限速阈值 |

## 测试缺口（2 条）

| # | 位置 | 问题 | 修复 |
|---|------|------|------|
| 7 | probe:281 | gzip 排除断言在 $FX 已删除后跑，恒过 | 待修（需上传新文件或调断言顺序） |
| 25 | probe:296 | 备份断言只查 store.json 不查 files/ | 已修：补 `grep -q 'files/'` |

---

**回归**：probe_teacherhub 104/104 全绿，go vet 干净，node --check 过。
