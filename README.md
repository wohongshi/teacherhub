# teacherhub —— 教师工作台（重写版）

**本地 WorkBuddy 插件 + 轻云端**，零账号体系：

```
老师的电脑（纯本地）                     云端（一台便宜 VPS 即可）
┌─────────────────────────┐            ┌──────────────────────────┐
│ WorkBuddy 插件（SKILL）   │            │ teacherhub serve          │
│  ├ teacherhub CLI        │  统一key   │  ├ ima 贡献统计（按人）    │
│  ├ 配置 WebUI（一键配置） │ ─────────► │  ├ 工作台产物库            │
│  └ 实验模板+参数配方      │  检索/推送  │  ├ 相似检索（先复用后生成）│
│ 生成在本地，产物自动推云端 │            │  └ 统计页 /stats          │
└─────────────────────────┘            └──────────────────────────┘
        ima 知识库 → 用 WorkBuddy 自带的 ima 插件（无需本插件操心）
```

- **云端不接手任何账号**：没有注册/登录/绑定，全员共用一把访问 key（不外传给学生）。
- **配置 WebUI**：`teacherhub ui` 弹本地配置页（云端地址 / 教师姓名 / 访问 key），只存本机。
- **生成工作台的规矩**：先 `search` 检索云端有无类似产物 → 有就发链接复用 → 没有才本地生成 → 生成完 `push` 自动上云。
- **贡献统计**：老师往 ima 知识库上新资料时报一条 `contrib`，云端按人聚合，谁贡献了什么一目了然。

## 目录

```
teacherhub/
├── main.go / cloud.go / ui.go     # 单二进制双角色：serve（云端）+ CLI（本地）
├── scripts/probe_teacherhub.sh    # 云端探针（14 断言）
├── plugins/workbuddy/teacherhub/  # WorkBuddy 插件包（SKILL + 四平台 CLI + 模板）
└── README.md
```

## 云端部署（云服务器，一条命令）

```bash
# 上传 teacherhub 二进制后：
./teacherhub serve --addr :8096 --data /var/lib/teacherhub --key 你定的统一key
# 开机自启（systemd）参考：
#   ExecStart=/opt/teacherhub/teacherhub serve --addr :8096 --data /var/lib/teacherhub --key XXX
# 验收：curl "http://IP:8096/api/ping?key=XXX" → {"ok":true}
```

- 放行端口 8096（安全组/防火墙）；有域名建议挂 HTTPS 反代（nginx/caddy）。
- 换 key：重启时换 `--key` 即全员换锁。
- 数据全在 `--data` 目录（store.json + artifacts/），备份这一个目录=备份全部。
- 统计页：`http://地址:8096/stats?key=XXX`（可收藏/投屏）。

## 教师端（WorkBuddy 插件）

把 `plugins/workbuddy/teacherhub/` 拷进 `~/.workbuddy/skills/`（Windows 是 `C:\Users\你\.workbuddy\skills\`），重启 WorkBuddy。
首次运行 `teacherhub ui`（或让 WorkBuddy 里的助手帮你跑 `teacherhub config`）填三样配置，之后全部动嘴完成：

- 「帮我做个钾与水反应的实验工作台」→ 检索 → 复用/生成 → 自动推送 → 发链接
- 「我往知识库加了《银镜反应》」→ 报贡献一条
- 「最近大家贡献了啥」→ 统计

## 检索相似度说明

中文友好的二元切片重叠：查询词与「标题×2 + 标签 + 关键词 + 简介」比对，≥0.3 命中，按相似度排序；查询串完整出现在标题里直接 1.0。同一个实验换个说法搜（「钠和水」vs「钠与水反应」）也能命中。

## 开发

```bash
go build ./... && go vet ./...
go build -o /tmp/teacherhub-bin . && bash scripts/probe_teacherhub.sh   # 14 断言
```
