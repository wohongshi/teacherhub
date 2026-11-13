# ocr 代码评审处置表 · teacherhub 增强十（2026-11-13）

评审范围：`a272a60..HEAD`（3 提交：评分/备份管理/移动底栏/搜索推荐/美化 + 2 fix），5 文件 +251/-14。
ocr 会话 1cb4c08c，12 条意见 / 9m26s / ~297 万 token。
**分诊：已修 11 · 设计如此 1 · 误报 0**（本轮评审质量高，无一条站不住脚）。

## 已修（11）

| # | 位置 | 级别 | 问题 | 修法 |
|---|------|------|------|------|
| 1 | probe:166 | test·med | 限一评断言不严：不验证"未计分"、缺 stars 越界回归 | 补三断言：越界 400、越界未计分（rate 0/num 0）、重复评分不计分（5.0/1 人） |
| 2 | probe:168 | test·low | BN 空值退化测错路由、200 空体也过 | BN 空守卫 + `%{size_download}` 断言非空 |
| 3 | probe:167 | test·high | 备份鉴权/路径穿越无回归保护 | 补无 key 拒 401 断言 + `--path-as-is` 穿越断言。**注意**：curl 会折叠 `..`（finding-classes #7 坑），必须 `--path-as-is`；Go ServeMux 对 `..` 回 **307** 清洗重定向而非 404，断言口径 = 3xx/404 且响应体不含 `"key"`（不泄露即安全） |
| 4 | probe:113 | test·low | MCP 工具数数断言不验名字集合 | 断言含 {rate_workbench, list_backups} 集合 + 补 list_backups 无 key 拒 |
| 5 | portal.go:218 | bug·med | `.noticebar` CSS display:flex 被 JS `style.display='block'` 内联覆盖，图标错位 | JS 改 `display='flex'` |
| 6 | cloud.go rateH | bug·low | 先占去重位后校验存在性：拼错 id 永久占坑 + 200/ok:false | 重构 applyRating：先定位→计分→save→再记去重；不存在 404 并退还去重位 |
| 7 | mcp.go | maintain·low | rate/list_backups 与 REST 完全重复 | 抽 applyRating / listBackups 共享，REST 与 MCP 同一份实现 |
| 8 | admin.go:115 | bug·low | 环比用浏览器时区拼日期键索引服务端数据，跨时区/跨零点错 | 服务端 ping 响应加 `yesterday` 值，前端直用 |
| 9 | cloud.go | security·med | CF-Connecting-IP 裸信任（LAN 直连可伪造绕限评）+ boardLikes 无限涨 | clientIP() 统一：net.ParseIP 校验、非法回落 RemoteAddr、XFF 取首跳；boardLikes 超 2 万整体清空 |
| 10 | portal.go:289 | bug·low | rank `i%3` 让第 4 名戴金牌 | 改 `i`，金银铜类只存 r0/r1/r2，其余回落灰 |
| 11 | portal.go:207 | bug·med | 移动底栏 z-index 70 遮住 .totop/.themebtn/FAB | @media ≤700px 内 FAB 组上移（bottom 70/126/182） |

## 设计如此（1）

| # | 位置 | 级别 | 意见 | 理由 |
|---|------|------|------|------|
| 12 | cloud.go rateH | perf·med | 持全局锁内 st.save() 阻塞磁盘 I/O | **全仓统一模式**：contrib/notice/board/delete/hidden/restore/rate 所有写 handler 都在锁内持久化（单机一致性取舍）。单改 rateH 反而行为不一致；低并发教学场景（几十人）影响可控。若优化需全仓统一改锁外写盘/去抖，另行立项 |

## 附注：评审修复过程中探针抓到的自伤

重构 applyRating 时埋了死锁（already 分支 `return` 在 `st.mu.Lock()` 后未解锁，二评必挂死）——**探针第二轮跑直接卡住**暴露，改 `defer st.mu.Unlock()` 修复。教训与 finding-classes #7 同款：断言要落在行为上，回归必须在重构后立刻跑。

## 回归

- `probe_teacherhub` **63/63 全绿**（57→63，新增 6 条全部为本轮意见的回归保护）
- go build / go vet 干净
- 公网部署后验证：公告横幅 flex 正常、评分 404/越界 400 口径正确
