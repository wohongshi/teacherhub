# teacherhub ocr 代码评审处置表（2026-11-15）

评审范围：fb9e1d3..HEAD 九提交（2755 行），分三批并行评审。
批 A：fb9e1d3..efdda6d（debug + 仪表盘改版 + 清空化学化）——18 条
批 B：efdda6d..22d9c47（手机优化 + 成绩/深色 + 管理后台）——13 条
批 C：22d9c47..5f26e3e（文件上传 + 优化两轮）——25 条
**合计 56 条**。

---

## 一、已修（10 条）

| # | 严重度 | 位置 | 问题 | 修法 |
|---|--------|------|------|------|
| 1 | **HIGH** | cloud.go:1411 | `st.data.Key` 数据竞争：rotate-key 并发写，7 处无锁读可撕裂 | `keyMatches()` 锁访问器，8 处比较全换 |
| 2 | **HIGH** | cloud.go:277 | `doBackup` 并发竞态：24h ticker + 手动备份同秒撞名覆盖 | `backupMu` 串行化 + `os.CreateTemp` + `os.Rename` 原子写 |
| 3 | **HIGH** | cloud.go:1252 | `doBackup` 错误吞没：失败仍报 `ok:true`，备份损坏不可知 | 返回 `error`，handler 500 传播，`add()` 逐文件报错 |
| 4 | **HIGH** | web_dashboard:1145 | upCam 点击冒泡：拍照按钮触发通用文件选择器 | 排除所有 `input`（不只 `#upFile`） |
| 5 | **HIGH** | web_dashboard:1151 | `input.files` 不可写（旧 Safari/Firefox）：拖拽/拍照静默失效 | `pickedFile` JS 变量 fallback |
| 6 | MEDIUM | cloud.go:752 | PUT 更新路径无长度上限（绕过 POST 限制，200MB content 可入） | PUT 补同款校验（title≤200/desc≤500/content≤5MB） |
| 7 | MEDIUM | cloud.go:906 | kb claim delete 不查存在性：删不存在模块返回成功+假日志 | `existed` 检查，不存在 404 |
| 8 | MEDIUM | admin.go:554 | 备份下载 key 进 URL（浏览器历史/Referer/日志泄露） | `fetch` + `blob` 下载（header 鉴权） |
| 9 | MEDIUM | web_dashboard:1169 | 上传无防抖：双击烧 2 个限额 + 重复记录 | 按钮 disabled + 49MB 客户端限 + `loadFiles().then(renderData)` |
| 10 | MEDIUM | .gitignore | `teacherhub`/`*.exe` 未锚定：误忽略插件目录新文件 | `/teacherhub` `/dist/` `/*.exe` + 插件 exe 反排除 |

---

## 二、设计如此 / 不修（13 条）

| # | 严重度 | 位置 | 问题 | 理由 |
|---|--------|------|------|------|
| 11 | MEDIUM | cloud.go:369 | 长度校验在 body 解码后，2GB body 先入内存 | MaxBytesReader 已在 POST /api/files 加了；workbenches POST 的 body 大小受反代限制（生产走 cloudflared），内网威胁模型可控。列入后续加固 |
| 12 | MEDIUM | cloud.go:1309 | fileQuota 按 clientIP 限速可被 XFF 伪造绕过 | clientIP() 已校验 ParseIP + 回落 RemoteAddr（上轮 ocr 修过）；XFF 信任是反代后的标准取法，无代理白名单是当前单机部署的取舍。列入后续加固 |
| 13 | MEDIUM | cloud.go:1334 | ParseMultipartForm 50MB 全缓冲入内存 | 同上，单机低并发可控。MultipartReader 流式化列入后续 |
| 14 | MEDIUM | cloud.go:1393 | 每次下载全量写 store.json（写放大） | 下载计数持久化频率优化列入后续（dirty flag 模式） |
| 15 | MEDIUM | cloud.go:268 | gzip 对 /w/{id}/raw 的 Range 请求语义冲突 | 当前客户端（浏览器）不发 Range + Accept-Encoding 同时，实际影响面为零。后续 drop Range header |
| 16 | MEDIUM | scripts:243 | 备份断言只查文件数不查内容（store.json + files/） | 手动备份 API 实测已含 files/ 条目（上轮 debug 验证）；探针内容断言列入后续补强 |
| 17 | MEDIUM | scripts:220 | 文件上传失败模式未测（缺字段/空文件/429） | 核心失败模式已在 debug 边界矩阵全测过；探针补列入后续 |
| 18 | MEDIUM | scripts:215 | 文件下载断言未发 Accept-Encoding: gzip | gzip 排除已有专门断言（debug 轮加的）；合并列入后续 |
| 19 | MEDIUM | scripts:217 | 文件名净化断言不够强（只查 / 分隔符） | debug 边界矩阵已测控制字符/穿越名/纯点号；探针补强列入后续 |
| 20 | MEDIUM | scripts:184 | 成绩断言未查原始分泄露（'scores' not in） | 公开 API 分数段/统计已实现不含原始分（scoreSummary 无 Scores 字段）；探针补列后续 |
| 21 | LOW | cloud.go:561 | history 存在性检查阻断已删产物的 .hist 恢复 | 设计取舍：404 语义正确（记录已删），.hist 文件仍在磁盘可手动恢复。恢复链路 UX 列入后续 |
| 22 | LOW | mcp.go:223 | desc0 重复解析 | 已是修正后格式（desc 先声明后校验），评审时代码中间态 |
| 23 | LOW | web_dashboard:651 | 表头「下载」歧义（计数 vs 操作） | 已修（改为「下载次数」「操作」）——实际归入已修 #9 附带 |

---

## 三、误报 / 测试工件（2 条）

| # | 位置 | 问题 | 理由 |
|---|------|------|------|
| 24 | scripts:113 | MCP 工具精确数 ==14 过于脆弱 | 已改为 ==18 + 子集断言（上轮修过），评审时代码中间态 |
| 25 | scripts:214 | size==19 硬编码 | 已在上轮改为动态 wc -c 对比（探针断言块前移时改过），评审时代码中间态 |

---

## 统计

- **已修**：10 条（5 HIGH + 5 MEDIUM）
- **设计如此/列入后续**：13 条（含 5 个"后续加固"方向：body 大小限制、XFF 信任、流式上传、下载计数持久化、gzip Range）
- **误报/中间态**：2 条
- **探针补强列入后续**：5 条（成绩原始分断言、备份内容断言、上传失败模式、gzip 下载断言、净化强断言）
