# AGENT_LOG — MetaTube (LTX fork) 服务端

**追加式日志。任何智能体改完代码，都要在文件末尾追加一条，不删改旧条目。**

- 一条记录 = 一次完整改动（一个主题），写完再开始下一个主题。
- 字段固定：改动 / 文件 / 验证 / 部署·发版 / 待办·风险。
- 日志里**不写**真实番号、内网 IP、密钥值、含用户名的绝对路径。

格式模板：

```markdown
## YYYY-MM-DD HH:MM · <智能体名> · <一句话标题>
- 改动：
- 文件：
- 验证：
- 部署/发版：
- 待办/风险：
```

---

## 2026-11-15 ~ 11-16 · WorkBuddy · 内置控制台 + 桌面面板 + 应用图标

- 改动：
  - 新增内置 Web 控制台 `/ui`（单文件前端）：总览只显示"最近搜索"（同番号聚合一行）、点详情出可读弹窗、请求日志独立页、搜索后可选自动翻译标题/演员/发售日期。
  - 新增请求日志中间件与 `/v1/logs`（内存环形缓冲 500 条，单响应体捕获上限 4MB，查询串里的 token/appid/key 自动打码）。
  - 新增 Windows 桌面面板 `metatube-panel.exe`（纯 Go WebView2 + Win32 子类化 + 托盘 + 高 DPI + 面板反代），负责拉起/停止/重启服务、一键退出。
  - 新增翻译引擎凭据管理 `/v1/translate/config`：`translate.json` 存密钥、mtime 热重载、PUT 合并写回、GET 只回打码值。
  - 新增 `MT_TRANSLATE_JSON` 环境变量基线（容器不挂 volume 时的方案），优先级 `query > 文件 > 环境变量 > 空`。
  - 新增应用图标：官方 logo → 9 尺寸 ICO → `windres` 生成 `.syso` 编进两个 exe；面板窗口/托盘改用同一枚图标，删掉手绘占位图。
  - **明确删除**：实时 API 用量百分比、额度电池、校准、peers 汇总整套代码（用户判断"只看本机没有意义，大头在 Docker 端"）。
- 文件：
  `route/logs.go`、`route/translate.go`、`route/translateconfig.go`(+`_test.go`)、`route/panel.go`、`route/route.go`、
  `route/ui/index.html`、`cmd/panel/{main,win,main_other}.go`、`cmd/panel/icon_windows_test.go`、
  `internal/iconres/{icon.go,icon_test.go,gen-icon.py,metatube.ico,metatube.png}`、
  `cmd/{panel,server}/icon_windows_amd64.syso`、`FORK.md`、`README*.md`、`.github/workflows/release.yml`
- 验证：gofmt / `go vet` / route+iconres+panel 单测 / Windows 编译 / Linux 交叉编译 / 内联 JS `node --check` 全过；解析 exe PE 资源确认 9 尺寸图标齐全、四角 alpha=0。
- 部署/发版：`v1.4.0-ltx.9` 已推送，Release 8 个资产（2 exe + 双架构镜像包 + 校验 + compose + INSTALL），GHCR `:ltx` 与 `:1.4.0-ltx.N` 同 digest。
- 待办/风险：
  - NAS Docker 端**需手动** `docker-compose pull && docker-compose up -d` 才切到新镜像。
  - 旧面板进程要退出重启才能看到新界面/图标。
  - Windows exe 在 CI 里由 `ubuntu-latest` 交叉编译（`windows-latest` 会无日志卡死），不要改回。

---

## 2026-11-12 · WorkBuddy · 全库重刷事故与翻译回退

- 改动：定位"2889 条标题/简介变日文"的根因——DeepL 免费额度耗尽后翻译返回原文，插件把原文写回 Plex。服务端配合修复百度系引擎 `error_code` 数字解码失败问题；前端补错误提示与"测试连接"。
- 文件：`translate/baidu/baidu.go`、`translate/baiduai/baiduai.go`、`route/ui/index.html`。
- 验证：批量翻译接口实测返回中文；`go build` / `go vet` 通过。
- 部署/发版：随 `v1.4.0-ltx.8` 发布。
- 待办/风险：翻译额度是硬约束，批量刷新前必须先确认额度；Butler 的"每三天更新元数据 + 维护期间刷新全部内容"已建议关闭。

---

## 2026-11-16 22:40 · WorkBuddy · 建立多智能体交接文档

- 改动：新增本文件与 [`AGENTS.md`](AGENTS.md)，明确铁律、目录地图、验证清单、发版流程、文档更新义务与多智能体协作规则。
- 文件：`AGENTS.md`、`AGENT_LOG.md`。
- 验证：内容与当前代码/CI 实际状态逐项核对（tag、workflow、脚本路径、测试命令）。
- 部署/发版：无，仅文档。
- 待办/风险：后续智能体必须保持本文件与代码同步，否则以本文件为准追责到"最近一条日志"。
