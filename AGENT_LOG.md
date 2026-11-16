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

---

## 2026-11-16 22:33 · WorkBuddy · 项目目录迁移到 C:/Script/metatube

- 改动：仓库从 WorkBuddy 会话工作区 `metatube/` 迁到 `C:/Script/metatube/metatube-server-ltx`；
  姊妹插件仓库迁到同级 `metatube-plex-plugins`；构建脚本、exe 运行时、离线回归测试统一放到 `_tools/`。
  `build-win.sh` 改为指向新仓库路径，并改用系统 PATH 的 `go`（与旧便携版同为 go1.27.3，不再依赖 `tools/go`）。
- 文件：`AGENTS.md`（工作目录、目录布局、构建命令）、本日志；仓库外 `_tools/build-win.sh`、`_tools/_subrule/*.py`。
- 验证：`git log` / `git status` 完整、remote 未变；`bash _tools/build-win.sh` 产出两个 exe，
  `metatube-server.exe -version` = `v1.4.0-ltx.9-71ca427`；插件三个回归测试在新路径下全过（75/30/3）。
- 部署/发版：无，仅目录与文档调整。
- 待办/风险：旧工作区遗留 `tools/go`（268 MB 便携 Go）未删，择机清理；
  若还有脚本写死旧路径，需同步修正。

## 2026-11-16 23:05 · WorkBuddy · 版本号体系独立（4.x）+ CI 三平台产物 + 本轮说明

- 改动：
  - 版本号与上游彻底解耦：仓库根新增 `VERSION`（当前 `4.0.0`）作为唯一来源。本地改动 PATCH +1
    （4.0.1、4.0.2 …），推送发版 MINOR +1、PATCH 归零 → `4.1.0`，tag 为**两点式** `v4.1`；
    发布后本地再改即 4.1.1、4.1.2 …，下次发版 4.2。旧编号 `v1.4.0-ltx.N` 停用。
  - 新增 `scripts/version.sh`：`show` / `bump` / `release` / `verify`。`release` 会校验
    `notes/<版本>.md` 存在，缺失时生成模板并中止；默认只做本地提交，`--tag` 才打 tag，
    `--push` 才推送（铁律：不擅自推送）。
  - 新增 `notes/` 目录：每个发布版本一个文件，只写本轮改动；`release.yml` 在 Prepare 阶段
    调 `version.sh verify` 校验 tag / VERSION / notes 三者自洽，不自洽直接失败。
  - `release.yml` 增加 `linux` 矩阵 job（amd64/arm64），编译静态二进制并打成
    `metatube-server-<版本>-linux-<arch>-bin.tar.gz`（含 LICENSE、VERSION、`.sha256`）；
    Release 说明改为「标题 + 本轮改动（notes）+ 资产表 + GHCR」，不再硬编码历史功能清单。
  - `docker.yml`：发布版标签改为 `:<版本>` + `:latest`，**去掉 `:ltx`**；`:dev` 版本串改为
    `dev-<VERSION 文件>-<短 sha>`。仓库 `docker-compose.yaml` 的镜像同步改成 `:latest`。
  - `Makefile` / `_tools/build-win.sh` 的版本号改为读 `VERSION` 文件（git describe 只作兜底）。
  - 新增 `.gitattributes`：`*.sh` 与 `VERSION` 强制 LF，`.syso`/`.ico`/`.png` 标记二进制。
  - 文档：`README.md` / `README.zh-CN.md` 头部说明、版本号章节、镜像标签表、离线安装、
    构建与发版流程全部改写；`FORK.md` 新增「版本号与发版」章节并同步镜像标签；
    `AGENTS.md` §2/§3/§4/§6/§9 同步。
- 文件：`VERSION`、`.gitattributes`、`scripts/version.sh`、`notes/README.md`、`notes/4.1.md`、
  `.github/workflows/{release,docker}.yml`、`Makefile`、`docker-compose.yaml`、
  `README.md`、`README.zh-CN.md`、`FORK.md`、`AGENTS.md`、本日志；仓库外 `_tools/build-win.sh`。
- 验证：`bash -n scripts/version.sh` 通过；`version.sh show` 输出 `4.0.0 → 4.0.1 / 发布 4.1`；
  `version.sh verify v4.1` 正确拒绝（VERSION 不匹配），`verify v4.0` 亦拒绝（缺 notes/4.0.md）；
  `mingw32-make -n server` 确认 ldflags 注入 `Version=4.0.0`；两个 workflow 用 PyYAML 解析通过
  （jobs: prepare/image/linux/windows/release）；`go build ./...`、`GOOS=linux go build ./...` 通过。
- 部署/发版：**未推送、未打 tag**（用户要求先不推）。首次按新体系发版应为 `v4.1`
  （`notes/4.1.md` 已写好）。NAS 上的 compose 引用 `:ltx`，发版后需改成 `:latest` 或固定版本。
- 待办/风险：
  - 发 4.1 后 `:ltx` 停在旧镜像，任何仍引用它的部署不会自动更新。
  - 本次改动与之前两个 docs 提交（`71ca427`、`bf2c2e9`）都还在本地。
  - 插件仓库版本号按用户要求**暂不动**。

## 2026-11-16 23:05 · WorkBuddy · 镜像 tag 改为带 v（`v4.1`）

- 改动：Docker 镜像 tag 从 `<版本>`（`4.1`）改为 **`v<版本>`（`v4.1`）**，与 git tag 一字不差；
  `:latest` 不变。
  - `docker.yml`：Get Version 增加 `tagv` 输出（tag 触发时=`${GITHUB_REF_NAME}` 即 `v4.1`，
    main 触发时=`dev`），发布构建的 tags 用 `tagv`。
  - `release.yml`：本地 `docker load` 镜像名、`docker save`、compose/INSTALL/RELEASE_NOTES
    里的 GHCR 地址统一用 `TAG`（`v4.1`）；资产文件名仍用不加 v 的 `4.1`。
  - 顺带修掉一处旧 bug：main 分支 `build-args` 里写的是 `VERSION=dev-${{ version }}`，而
    `version` 本身已是 `dev-4.0.1-<sha>`，会变成 `dev-dev-4.0.1-<sha>`；现改为直接引用。
  - `Dockerfile`：注释与 `image.title` label 改为 `metatube-server-ltx`。
  - 文档同步：`README.md` / `README.zh-CN.md` / `FORK.md` / `AGENTS.md` / `notes/4.1.md` /
    `scripts/version.sh` 注释，全部改成「镜像 tag 带 v，二进制内部版本号不带」。
- 验证：提取 `release.yml` 的生成脚本本地实跑（`VERSION=4.1 TAG=v4.1`）→ compose
  `image: ghcr.io/sayaxia/metatube-server:v4.1`、INSTALL 里离线导入后镜像名
  `metatube-server:v4.1`、RELEASE_NOTES 显示「镜像标签 `v4.1` 与 `latest`」；
  两个 workflow PyYAML 解析通过。
- 部署/发版：仍未推送、未打 tag。

## 2026-11-16 23:10 · WorkBuddy · 镜像改名 metatube-server-ltx + 临时目录约定

- 改动：
  - **镜像仓库改名**：`ghcr.io/sayaxia/metatube-server` → **`ghcr.io/sayaxia/metatube-server-ltx`**，
    与上游同名镜像彻底区分（避免 `docker pull` 误拉上游版本）。
    - `docker.yml`：`env.IMAGE` 改新名；发布标签仍是 `:v<版本>` + `:latest`，`:dev` 不变。
    - `release.yml`：`docker build/load/save` 的本地镜像名、GHCR 地址、compose/INSTALL/RELEASE_NOTES
      全部换成新名（离线包导入后是 `metatube-server-ltx:v4.1`）。
    - `docker-compose.yaml` 默认镜像改新名。
    - `Dockerfile`：`image.source` label 指向 fork 仓库。
  - **临时目录约定**（用户要求）：新增 `C:/Script/metatube/temp/`（仓库外），排障/试运行产生的脚本、
    日志 dump、截图一律放这里；确认要保留的再挪回项目合适位置；**每次 push 前清一遍**。
    已写入 AGENTS.md 铁律第 9 条、提交前验证清单新增一行、目录布局图，以及 `temp/README.md`；
    `交接提示词.md` 同步（布局、硬约束第 9 条、附二目录表、当前基线刷新到 4.x + 新镜像名）。
- 文件：`.github/workflows/{docker,release}.yml`、`docker-compose.yaml`、`Dockerfile`、`README.md`、
  `README.zh-CN.md`、`FORK.md`、`AGENTS.md`、`notes/4.1.md`、本日志；仓库外 `temp/README.md`、`交接提示词.md`。
- 验证：提取 `release.yml` 生成脚本本地实跑（`VERSION=4.1 TAG=v4.1 OWNER=sayaxia`）→ compose
  `image: ghcr.io/sayaxia/metatube-server-ltx:v4.1`，INSTALL 离线导入名与 `docker pull` 行全部为新名，
  RELEASE_NOTES 含「镜像改名」条目；两个 workflow PyYAML 解析通过；全仓 grep 无旧名残留
  （仅剩 README 里指向上游官方镜像的那一行，属正常引用）。
- 部署/发版：未推送、未打 tag。**NAS 上的 compose 仍写 `metatube-server:ltx`，发 4.1 之后必须改成
  `metatube-server-ltx:latest` 或 `metatube-server-ltx:v4.1`，否则拉不到（新仓库名此前不存在）。**

## 2026-11-16 23:15 · WorkBuddy · 本地 README 与 GitHub README 分离

- 改动（用户要求）：
  - 新增**仓库外** `C:/Script/metatube/本地README.md`：按轮次记录「改了什么 / 动了哪些文件 /
    哪些进了 GitHub README、哪些没进及原因 / 验证 / 撤回或遗留」，并回填了近期各轮
    （4.x 版本体系、镜像 v 标签、镜像改名、目录迁移、控制台+面板+图标、MT_TRANSLATE_JSON、
    11-12 重刷事故）以及「试过又否掉」的方案（exe 事后注入图标、windows-latest、`:ltx` 标签、
    PMS 1.40 插件侧换封面）。
  - 规矩：**本地加过又删掉的内容不进 GitHub README**；GitHub README/FORK 只写成品当前真实能力；
    本地 README 绝不上传（也不 `git add -f`）。
  - `AGENTS.md`：铁律新增第 10 条；§6 发版前必做 b 项补「只写最终保留的能力」；§9 文档义务表加一行。
  - `.gitignore`：加 `本地README.md` / `LOCAL_README.md` / `*.local.md` 防呆（已用 check-ignore 验证）。
  - 顺带修 README.zh-CN 里一个**不可用的示例**：`replace ... => github.com/sayaxia/metatube-sdk-go v4.1`
    违反 Go 主版本路径规则（v2+ 需 `/v4` 后缀），`go` 会直接拒绝；改为本地源码路径替换并注明原因。
- 文件：`AGENTS.md`、`.gitignore`、`README.zh-CN.md`、本日志；仓库外 `本地README.md`、`交接提示词.md`。
- 验证：`git check-ignore -v 本地README.md` 命中；文档改动无代码影响；`go build ./...` 未受影响（未动代码）。
- 部署/发版：未推送、未打 tag。用户会自行重新拉镜像、重建 NAS 容器，不代为操作。

## 2026-11-16 23:15 · WorkBuddy · 首次按 4.x 体系发布 v4.1（已推送）

- 改动：把 6 个本地提交推上 GitHub，并按新版本体系发布第一个版本。
  - `scripts/version.sh release --tag`：`VERSION` 4.0.0 → **4.1.0**，本地打 tag **`v4.1`**
    （commit `c199514`），随后推 `main`（`3a3ef01..c199514`）与 tag。
  - 插件仓库同步推送 `main`（`86b2c28..f1acd5f`），其中 `b70e3a7` 改了 bundle，
    自动产出插件 Release **`v2026.1116.1517.0-f1acd5f`**。
- 发版核验（全部实测）：
  - CI 全绿：Test / Linter / CodeQL / Image amd64 / Image arm64 / Linux binary amd64+arm64 /
    Windows binaries / Prepare / Publish Release / Publish Docker Image(tag) 均 success。
  - Release `v4.1`（`metatube-server-ltx v4.1`）共 **12 个资产**：4 个 Linux tar.gz（amd64/arm64 ×
    裸二进制/镜像包）+ 4 个 `.sha256` + `metatube-server.exe`(46.87 MB) + `metatube-panel.exe`(7.96 MB)
    + `docker-compose.yml` + `INSTALL.md`；说明正文只含本轮 `notes/4.1.md` 内容。
  - 下载 `metatube-server-4.1-linux-amd64.tar.gz` 实算 SHA-256 与 Release 提供的 `.sha256` **一致**
    （`02662930…`）；解包读 `manifest.json` → 镜像名 **`metatube-server-ltx:v4.1`**，
    label `image.title=metatube-server-ltx` / `image.version=4.1`。
  - 下载 `metatube-server.exe` 实跑 `env -u VERSION ./metatube-server.exe -version` →
    **`v4.1-c199514`**（版本与提交号都对）。
  - GHCR：`:v4.1` 与 `:latest` 同一 digest **`sha256:593bd2a8…`**，manifest list 含 linux/amd64 +
    linux/arm64（Docker v2 manifest list，DSM 18.09 可拉）；`:dev` 单独 digest（main 构建）。
  - 包是私有的：匿名 token 拉 manifest 返回 DENIED，需带 GitHub token 换 GHCR token 才能查。
- 部署/发版：服务端与插件**均已推送**，服务端 Release `v4.1` 是当前线上版本；
  旧编号 `v1.4.0-ltx.N` 与 `:ltx` 标签正式停用（历史镜像未删除）。
- 待办/风险：
  - NAS 上 compose 仍写 `metatube-server:ltx`，**必须改成 `metatube-server-ltx:v4.1`（或 `:latest`）**
    并重建容器；旧容器名 `metatube` 要先 `docker rm -f`。
  - 插件工作区那 6 个「三层翻译保底」文件仍未提交、未部署，本轮刻意没碰。
  - 下次本地改动从 `4.1.1` 起（`version.sh bump`），下次发布为 `v4.2`。

## 2026-11-16 23:54 · Doubao · 核对线上容器：已运行 v4.1（无代码改动）

- 改动：无代码改动。核对 NAS 容器实际运行版本，关闭此前「容器还没换到 v4.1」的遗留项。
- 文件：本日志；仓库外 `交接提示词.md`、`本地README.md` 的状态描述同步修正。
- 验证：`GET http://<NAS-A>:8080/v1/status` 返回
  `version=v4.1-c199514ef0f6c5bd90831c7f0e29719e718cd320`（= tag `v4.1`）、`goos=linux`、`goarch=amd64`；
  容器 uptime 连续增长，推算约 23:46 启动。GHCR `:v4.1` 与 `:latest` 同一 digest `sha256:593bd2a8…`，
  `:dev` 单独 digest。
- 部署/发版：无，仅核对。
- 待办/风险：本机无法确认容器所用 image digest（SSH 到 <NAS-A> 用默认密钥被拒、本机无 docker CLI），
  版本串带完整 commit 可认定跑的是 v4.1 镜像。若 NAS 上的 compose 还留着旧镜像名 `metatube-server:ltx`，
  下次 `up -d` 会拉不到新版，建议一并改成 `metatube-server-ltx:v4.1`（或 `:latest`）。