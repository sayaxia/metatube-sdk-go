# AGENTS.md — MetaTube (LTX fork) 服务端

> **任何智能体在改这个仓库之前，必须先读完本文件。**
> 改完之后必须同步更新本文件里受影响的部分，并在 [`AGENT_LOG.md`](AGENT_LOG.md) 追加一条记录。
>
> 最后更新：2026-11-16 · 维护者：SAYA

---

## 0. 这是什么

上游 `metatube-community/metatube-sdk-go` 的私有 fork，额外提供：

- 番号归一化（剥离 `-LTX` / `-U` / `__restored` / `-BasicVSR++` 等工具后缀）
- 实验池 provider（`-tags experimental`）
- 出站代理（Windows 跟随系统代理，Linux/Docker 跟随环境变量）
- 内置 Web 控制台 `/ui` + Windows 桌面面板 `metatube-panel.exe`
- 翻译引擎凭据文件 `translate.json` + `MT_TRANSLATE_JSON` 环境变量基线
- 请求日志 `/v1/logs`

完整差异、版本规则、Docker 说明见 [`FORK.md`](FORK.md)；面向使用者的说明见 `README.md` / `README.zh-CN.md`。

---

## 1. 铁律（不可协商）

1. **不擅自 push / 打 tag / 发版。** 本地 commit 可以，推到 GitHub、建 Release、更新镜像必须先拿到明确同意。
2. **不擅自加 token / 改鉴权。** 服务只在内网使用，鉴权策略由用户决定。
3. **提交前脱敏。** 真实番号、内网 IP/主机名、含用户名的绝对路径、token/密钥 一律不得进入提交内容。示例用 `<分类>/<任务>`、`<NAS-A>`、`~/...` 占位。
4. **不读不写不打印密钥。** `translate.json`、`panel.json`、`.env` 里是真实凭据；只在必要时确认字段是否存在，永不复制其值到文档、日志、提交。
5. **禁止 `refresh?force=1`。** 那是全库强制重刷（queueSize = 全库条数），会直接烧掉翻译额度，并可能把中文元数据覆盖成日文。只做文件扫描用不带 force 的 `refresh`。
6. **批量刷新前先确认翻译额度。** 额度耗尽时翻译会回退日文原文并写回 Plex（2026-11-12 事故根因）。
7. **改完必须验证**（见 §5），验证没过不许说"完成"。
8. **一个改动一个主题。** 不要顺手重构无关代码，不要把多个不相关的修复塞进同一个提交。

---

## 2. 仓库与分支

| 项 | 值 |
|---|---|
| 工作目录 | `C:/Script/metatube/metatube-server-ltx` |
| `origin` | `metatube-community/metatube-sdk-go`（**上游，只读，不要往上推**） |
| `mine` | `sayaxia/metatube-sdk-go`（**推这里**） |
| 主分支 | `main`，跟踪 `mine/main` |
| 当前版本 | 仓库根 `VERSION` = `4.0.0`（本地开发版本）；旧编号 `v1.4.0-ltx.N` 已停用 |
| 镜像 | `ghcr.io/sayaxia/metatube-server`，标签 `:v<版本>` `:latest` `:dev` |

**版本规则**（与上游彻底解耦，详见 §6）：

- 本地改动：PATCH +1 → `4.0.0` → `4.0.1` → `4.0.2` …
- 推送发版：MINOR +1、PATCH 归零 → `4.1.0`，tag **两点式** `v4.1`
- `:dev` 跟随 main push；`:v<版本>`（如 `:v4.1`，与 git tag 一字不差）与 `:latest` 只由 tag 触发；
  旧的 `:ltx` 自 v4.1 起停止更新
- 镜像 tag **带 `v` 前缀**（`:v4.1`），与 git tag 完全一致；二进制内部版本号不带（`4.1`）

**目录布局**（`C:/Script/metatube/` 下）：

```
metatube-server-ltx/     本仓库（服务端）
metatube-plex-plugins/   Plex 插件仓库（姊妹项目）
_tools/                  build-win.sh 构建脚本、metatube-win/ exe 运行时、_subrule/ 离线回归测试
```

**Git 凭据**：本机 Git Credential Manager 已登录，`git push` 直接可用，**不要向用户索要 PAT**。

---

## 3. 目录地图

| 路径 | 作用 |
|---|---|
| `cmd/server/` | 服务端入口（Docker 镜像只构建这个单文件） |
| `cmd/panel/` | Windows 桌面面板（WebView2 + 托盘），Linux 下编译成一行提示的 stub |
| `cmd/panel/win.go` | 窗口、托盘、DPI、面板反代、HICON 管理 |
| `route/route.go` | 路由注册总表（公开 / 私有分界在这里） |
| `route/logs.go` | 请求日志中间件 + `/v1/logs`，查询串脱敏 |
| `route/translate.go` | `/v1/translate`、`/v1/translate/batch` |
| `route/translateconfig.go` | `/v1/translate/config`，`translate.json` 热重载 + 环境变量基线 |
| `route/panel.go` | `/v1/status`（版本、代理决策、provider 数） |
| `route/ui/index.html` | 内置控制台单文件（`go:embed`），改完要做 JS 语法检查 |
| `common/fetch/proxy*.go` | 出站代理：`proxy_windows.go` 读注册表，`proxy_unix.go` 读环境变量 |
| `internal/iconres/` | 应用图标：ICO 母版、解析器、`gen-icon.py`（可复现生成） |
| `cmd/{server,panel}/icon_windows_amd64.syso` | 编进 exe 的图标资源，仅 windows/amd64 自动链接 |
| `engine/init.go` | 启动时 `fetch.InstallDefaultTransport()`（colly 也走代理的关键） |
| `.github/workflows/` | `docker.yml`（main→`:dev`，tag→`:v<版本>`+`:latest`）、`release.yml`(tag→镜像+Linux 二进制+exe+Release)、`test.yml`、`linter.yml`、`codeql-analysis.yml` |
| `VERSION` | 版本号唯一来源（本地 `4.0.1` / 发布 `4.1.0`），Makefile、Dockerfile、CI、build-win.sh 都读它 |
| `scripts/version.sh` | `show` / `bump` / `release` / `verify`，发版与 CI 校验都走它 |
| `notes/<版本>.md` | 本轮发布说明（只写本轮），release.yml 缺文件直接失败 |

---

## 4. 本地开发与构建

本机是 Windows，Git Bash 环境下操作；Go 用系统 PATH 里的 `go`（go1.27.3，与旧便携版同版本）。

```bash
# 日常编译 + 静态检查
go build ./...
go vet ./route/... ./cmd/... ./internal/...
gofmt -l <改动过的文件>        # 应为空

# 单测
go test ./route/... ./common/fetch/... ./internal/iconres/... -count=1

# 模拟 Docker 构建（Linux 目标，必须过）
GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -tags experimental ./cmd/server
```

**控制台前端**（`route/ui/index.html`）必须做内联 JS 语法检查：

```bash
python - <<'PY'
import re, pathlib
s = pathlib.Path('route/ui/index.html').read_text(encoding='utf-8')
blocks = re.findall(r'<script[^>]*>(.*?)</script>', s, re.S)
pathlib.Path('/tmp/ui_inline.js').write_text('\n'.join(blocks), encoding='utf-8')
PY
node --check /tmp/ui_inline.js
```

**Windows 产物**（两个 exe，脚本在仓库外）：

```bash
bash C:/Script/metatube/_tools/build-win.sh
# 版本读仓库根 VERSION（本地如 4.0.1），commit 取当前 HEAD；产物在 C:/Script/metatube/_tools/metatube-win/
# 校验：C:/Script/metatube/_tools/metatube-win/metatube-server.exe -version
# panel 是 windowsgui 程序，没有 -version 参数（正常）
```

**改图标**：改 `internal/iconres/gen-icon.py` 里的常量（`ICON_CORNER_RADIUS` 等）→ 重跑
`python internal/iconres/gen-icon.py` → 用 `windres` 重新生成两份 `.syso` → 重建 exe。

---

## 5. 提交前验证清单

| 检查 | 命令 | 通过标准 |
|---|---|---|
| 格式 | `gofmt -l` 改动文件 | 无输出 |
| 静态检查 | `go vet` 相关包 | 无输出 |
| 单测 | `go test` 相关包 | 全 PASS |
| Windows 编译 | `go build ./...` | 成功 |
| Linux 编译 | `GOOS=linux GOARCH=amd64 go build ./...` | 成功 |
| 前端 JS | `node --check` | 成功 |
| 脱敏 | `git diff` 肉眼过一遍 + grep 内网 IP/真实番号/密钥前缀 | 无命中 |
| 误提交 | `git status --short` | 无 `translate.json` / `panel.json` / `*.exe` / `data/` |

---

## 6. 发版流程（需用户明确同意后才执行）

版本号唯一来源是仓库根 [`VERSION`](VERSION)，用 [`scripts/version.sh`](scripts/version.sh) 推进：

```bash
# 1. 本地每改完一轮（PATCH +1）
bash scripts/version.sh bump

# 2. 正常提交改动（禁止 git add -A，只加自己改的文件）
git add <文件...> && git commit -m "feat(scope): ..."

# 3. 发版前必须先做的事（缺一项都算没完成）
#    a. notes/<下个版本>.md 写本轮更新说明（只写本轮，不带历史）
#    b. README.md / README.zh-CN.md 把本轮改动**合并**进「分支新增功能」，不是追加"本轮新增"小节
#    c. FORK.md 同步受影响的章节

# 4. 定版（写 VERSION=4.1.0 + 提交 + 本地打 tag，默认不推送）
bash scripts/version.sh release --tag

# 5. 推（用户同意后）
git push mine main && git push mine v4.1
```

推 tag 后 `release.yml` 会跑：Prepare（`version.sh verify` 校验 tag/VERSION/notes 自洽）→
Image amd64/arm64 → Linux binary amd64/arm64 → Windows binaries → Publish Release；
`docker.yml` 同时推 `:v<版本>` 与 `:latest`。

发版后必须核验（三项都做）：

1. `actions/runs` 全部 success（Image×2 / Linux×2 / Windows / Publish Release / Publish Docker Image）
2. Release 资产齐全：Linux amd64/arm64 裸二进制 + 两个镜像包（各带 `.sha256`）+ 2 个 exe
   + `docker-compose.yml` + `INSTALL.md`
3. GHCR `:latest` 与 `:v<版本>` 指向同一 digest

已知 CI 坑：

- **`windows-latest` runner 会无日志卡死**（实测挂 20+ 分钟）。Windows exe 一律在 `ubuntu-latest`
  交叉编译（`CGO_ENABLED=0 GOOS=windows`），`.syso` 会自动链接，`-H windowsgui` 也正常。
- **`VERSION` 环境变量会让 `-version` 开关失效**（`cmd/server/main.go` 的判断）：CI 里跑
  `./metatube-server -version` 必须 `env -u VERSION`，否则进程会直接起服务并挂住。
- golangci-lint 比 `go vet` 严：注意未使用的常量、`w.ResponseWriter.Status()` 这类可简化的选择器。
- 网络：本机 DNS 可能被 Clash fake-ip 接管导致 GitHub TLS 失败；联网排查见 §8。

---

## 7. 运行时配置

| 文件 / 变量 | 说明 |
|---|---|
| `translate.json`（exe 同目录） | 翻译引擎凭据，键名 = 引擎结构体 json 标签（`deepl-api-key` 等）；mtime 热重载 |
| `MT_TRANSLATE_CONFIG` | 覆盖 `translate.json` 路径 |
| `MT_TRANSLATE_JSON` | 整份 JSON 内联，作**基线**（文件同名键覆盖它），容器不挂 volume 时用 |
| `MT_TRANSLATE_ENGINE` | 压过文件里的 `engine` |
| `MT_PROXY_URL` / `MT_SYSTEM_PROXY` | 全局代理 / 本进程忽略系统代理 |
| `MT_<TYPE>_PROVIDER_<NAME>__PROXY` | 单 provider 代理，优先级最高 |
| `TOKEN` | 可选访问令牌，内网默认不设 |
| `panel.json` | 面板配置（后端地址、token、端口） |

服务端默认端口 8080；`start.bat` 拉起面板，面板托管服务并常驻托盘。

---

## 8. 已知坑

**代理**

- `colly` 是第二条网络通道：它的 client 没有自定义 Transport，会落到 `http.DefaultTransport`，
  直接读 `HTTPS_PROXY` 环境变量，绕过 `fetch.Proxy()`。所以启动时必须调
  `fetch.InstallDefaultTransport()`，否则 FANZA/DMM、SOD 等一批 provider 全部抓空，
  症状是"只有 JavBus 有结果"。
- Windows 上**刻意不读环境变量**（会话注入的代理会把 provider 带到被封地区），只读注册表
  `ProxyEnable`/`ProxyServer`，10s 复查一次。Linux/Docker 才读环境变量。
- 测"DMM 直连"必须 `curl --noproxy '*'`，否则测的是 `HTTPS_PROXY`。

**Plex 侧**

- 列表接口只回前 2 个 Genre；XML 不含 unavailable/accessible 字段。
- 判断是否在重刷：日志 grep `queueSize=`；中断重刷只能重启 Plex。

**构建**

- `.syso` 只在 `windows/amd64` 自动链接；`Makefile` 的 `server` 目标用单文件 `cmd/server/main.go`，
  连包目录都不扫，所以 Docker 镜像天然不含 Windows 资源。
- 改 `DefaultPrefs.json` 之类的枚举属于插件仓库，不在这里。

---

## 9. 文档更新义务

**每次改动**都要按下面这张表更新文档，缺一项视为未完成。

| 改了什么 | 必更文档 |
|---|---|
| 任何改动 | `AGENT_LOG.md` 追加一条（格式见文件头） |
| 新增/删除接口、路由 | 本文件 §3 目录地图、`FORK.md` 对应章节 |
| 新增环境变量 / 配置项 | 本文件 §7、`FORK.md` 对应章节 |
| 构建方式、CI、发版流程 | 本文件 §4/§5/§6、`README*.md` |
| 版本号规则、发布产物、镜像标签 | 本文件 §2/§6、`FORK.md`「版本号与发版」、`README*.md`；本轮说明写 `notes/<版本>.md` |
| 用户可见行为 | `README.md` + `README.zh-CN.md` |
| 平台差异（Windows / Docker） | 本文件 §8，并确认 Docker 侧不受影响 |

---

## 10. 多智能体协作规则

- **开工前**：`git status --short` → 读 `AGENT_LOG.md` 最后 3 条 → 确认工作区是否有人未提交的改动。
- **禁止** `git add -A` / `git checkout .` / `git reset --hard` / `git stash`：会吞掉别人未提交的工作。
  只 `git add` 自己改的文件。
- **禁止** 在别人的未提交改动上"顺手格式化"，会制造巨大噪声 diff。
- 提交信息用 `feat(scope):` / `fix(scope):` / `ci(scope):` / `docs(scope):` 前缀，正文写清"为什么"。
- 同时有智能体在改插件仓库时，**先确认那边没在写同名逻辑**（翻译/角标规则两边都有实现）。
