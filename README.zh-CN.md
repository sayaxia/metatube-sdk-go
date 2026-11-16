# metatube-server-ltx（MetaTube Server 个人分支）

[![Build Status](https://img.shields.io/github/actions/workflow/status/sayaxia/metatube-sdk-go/release.yml?branch=main&style=flat-square&logo=github-actions&label=release)](https://github.com/sayaxia/metatube-sdk-go/actions/workflows/release.yml)
[![Require Go Version](https://img.shields.io/badge/go-%3E%3D1.25-30dff3?style=flat-square&logo=go)](https://github.com/sayaxia/metatube-sdk-go/blob/main/go.mod)
[![GitHub License](https://img.shields.io/github/license/sayaxia/metatube-sdk-go?color=e4682a&logo=apache&style=flat-square)](https://github.com/sayaxia/metatube-sdk-go/blob/main/LICENSE)
[![Tag](https://img.shields.io/github/v/tag/sayaxia/metatube-sdk-go?color=%23ff8936&logo=fitbit&style=flat-square)](https://github.com/sayaxia/metatube-sdk-go/tags)

> **这是 [metatube-community/metatube-sdk-go](https://github.com/metatube-community/metatube-sdk-go) 的个人分支（fork），
> 已独立成项目 `metatube-server-ltx`，版本号自成一系（`v4.N`，不再跟随上游的 `v1.4.0-ltx.N`）**，维护于
> [sayaxia/metatube-sdk-go](https://github.com/sayaxia/metatube-sdk-go)。
> 相对上游增加：识别带 AI 修复后缀的文件名（如 `ABP-030-LTX.mp4`、`ABP-030__restored.mp4`），
> 内置 Web 控制台、Windows 桌面面板、出站代理跟随、翻译引擎与凭据热重载等（详见下文）。
> 其余功能与上游一致，**上游始终是唯一事实来源**。
> 完整差异、版本规则与 Docker 说明见 [FORK.md](FORK.md)。

English version: [README.md](README.md)

## 目录

- [这个分支解决什么问题](#这个分支解决什么问题)
- [分支新增功能](#分支新增功能)
- [版本号与发版](#版本号与发版)
- [特性](#特性)
- [安装](#安装)
    - [作为 Go SDK 使用](#作为-go-sdk-使用)
    - [Docker 部署（推荐）](#docker-部署推荐)
    - [离线安装](#离线安装)
    - [Linux 裸二进制（不装 Docker）](#linux-裸二进制不装-docker)
    - [Windows 桌面版](#windows-桌面版)
- [番号后缀归一化](#番号后缀归一化)
- [构建](#构建)
- [与上游同步](#与上游同步)
- [致谢](#致谢)
- [许可](#许可)

## 分支新增功能

- **内置控制台 `/ui`**：总览按番号聚合显示最近搜索（一行一个番号），点开有友好的详情弹窗；
  请求日志可读、查询串中的密钥自动打码、支持一键清空；提供自动翻译开关，
  浏览器打开任意实例的 `/ui` 即可使用。
- **Windows 桌面面板 `metatube-panel.exe`**：WebView2 窗口 + 系统托盘，自动启动/停止/重启
  服务；关闭窗口与最小化都进托盘，右键退出会一并关闭服务。窗口与托盘用官方 logo 图标，
  并声明 DPI 感知保证字体清晰。可与 `metatube-server.exe` 放同目录独立运行。
- **出站代理**：默认跟随系统代理（Windows 读注册表、Linux/Docker 读环境变量），
  可用 `MT_PROXY_URL` 覆盖；同时修复 colly 系 provider 绕过代理导致抓空的问题。
- **翻译**：新增百度大模型引擎 `baiduai`；凭据放在可执行文件同目录的 `translate.json`
  或 `MT_TRANSLATE_JSON` 环境变量（容器无需挂 volume），改动热重载；
  提供批量翻译接口与自动翻译开关（标题/演员/发售日期）。
- **发版资产**：每个 tag 同时提供 Linux amd64/arm64 裸二进制包、Linux amd64/arm64 的
  `docker load` 镜像包，以及 Windows x64 的服务端与桌面面板。

## 版本号与发版

本分支版本号**与上游彻底解耦**，`MAJOR` 固定为 4（承接上游 v1.4.0 的基线）：

| 阶段 | 版本号 |
| --- | --- |
| 本地改动 | `4.0.1`、`4.0.2` …（PATCH 递增，只在本地） |
| 推送 GitHub 发版 | `v4.1`、`v4.2` …（MINOR 递增、PATCH 归零） |

发布后再改本地就是 `4.1.1`、`4.1.2` …，下次推送即 `4.2`。

- 版本号唯一来源是仓库根的 `VERSION` 文件，`scripts/version.sh` 负责推进：
  `bump`（本地改动 +1）、`release`（升到下一个发布版本并提交）。
- 镜像：`ghcr.io/sayaxia/metatube-server-ltx`（**带 `-ltx` 后缀**，与上游 `metatube-server` 区分），
  标签为 `:v4.1`（本轮版本号，**带 v**）+ `:latest`；旧的 `:ltx` 自 v4.1 起**停止更新**。
- 每次发版的更新说明只写本轮改动，放在 `notes/<版本>.md`；CI 缺这个文件会直接失败。

## 这个分支解决什么问题

Jasna 等去马赛克工具在使用 **LTX** 模型修复后，会把模型名写进输出文件名：

```
修复前：ABP-030.mp4
修复后：ABP-030-LTX.mp4
```

除了 `-LTX`，同类工具还会写 `_LTX`、`-BasicVSR++`、`-restored` / `__restored`，
并可能再叠加一层工具标记，如 `JUR-110_restored_wm.mp4`、`SONE-521_restored.LTX_wm.mp4`
（`_wm` 为带水印压制版，`.LTX` 用点号分隔）。
这些后缀**都是无码破解**的标记：Plex 插件侧统一判为 `u.png` 角标（带中文字幕则 `uc.png`）
并写同一个「无码破解」类型标签，本质上是同一个版别，只是拼写不同。

MetaTube 在检索前会对文件名做番号归一化，但原逻辑只剥离 `-C`、`-UC`、`-ch`、`-cdN`、`-A`~`-D`、`-hhbN`
等已知后缀，`-LTX` 被当成番号的一部分，于是所有 provider 都在搜 `ABP-030-LTX` → 返回空结果 → 自动刮削失败；
`-restored` 还会被末尾的 `A|B|C|D` 规则吃掉一个 `d`，变成 `ABP-030-restore`。

本分支在归一化阶段把 `ltx|restored|basicvsr(?:\+\+)?|wm` 一并**循环剥离**，
因此叠加顺序不影响结果：`ABP-030-LTX`、`ABP-030__restored`、
`JUR-110_restored_wm` 都会被正确识别为裸番号。
该修复已作为 PR 提交上游：[#388](https://github.com/metatube-community/metatube-sdk-go/pull/388)；
上游合并后即可直接用官方镜像，无需继续使用本分支。

> **另外**：FC2 官方把付费作品详情页改成了「必须登录」，
> 未登录访问会 302 到登录页并最终 404，导致**所有** `FC2-PPV-*` 番号刮不到（与 `-LTX` 无关）。
> 本分支已同步上游的 `FC2DB` provider（数据源 `fc2db.net`）作为兜底，
> 现象与细节见 [FORK.md](FORK.md#fc2-番号刮不到2026-11-起)。

## 特性

- 支持平台
    - Linux
    - Darwin
    - Windows
    - BSD(s)
- 支持的数据库
    - [SQLite](https://gitlab.com/cznic/sqlite)
    - [PostgreSQL](https://github.com/jackc/pgx)
- 图像处理
    - 自动裁剪
    - 角标（Badge）支持
    - 人脸检测
    - 图像哈希
- RESTful API
- 20+ 数据源（provider）
- 文本翻译

## 安装

### 作为 Go SDK 使用

需要先安装 [Go](https://golang.org/)（**要求 go1.25 及以上**）：

```sh
go get -u github.com/metatube-community/metatube-sdk-go
```

如需使用本分支的修复，在 `go.mod` 里替换成本目录的源码路径即可：

```
replace github.com/metatube-community/metatube-sdk-go => /path/to/metatube-server-ltx
```

> 不要写成 `=> github.com/sayaxia/metatube-sdk-go v4.1`：本分支版本号是 4.x，
> 而 Go 要求 v2 及以上的模块路径带 `/v4` 后缀，直接按版本号替换会被 `go` 拒绝。

### Docker 部署（推荐）

镜像已发布到本仓库的 GHCR 命名空间，**公开可拉取，无需登录**：

```bash
docker run -d --name metatube -p 8080:8080 \
  -v metatube-data:/data \
  ghcr.io/sayaxia/metatube-server-ltx:latest \
  -dsn /data/metatube.db -port 8080
```

或使用 `docker-compose.yaml`（默认已指向本分支镜像，并把 SQLite 落到 `./data`）：

```yaml
services:
  metatube:
    image: ghcr.io/sayaxia/metatube-server-ltx:latest
    container_name: metatube
    ports:
      - "8080:8080"
    restart: unless-stopped
    volumes:
      - ./data:/data
    command: -dsn /data/metatube.db -port 8080
```

```bash
docker compose pull && docker compose up -d
```

镜像标签说明：

| 标签 | 含义 |
| --- | --- |
| `latest` | 最新一次发布版本（跟随 tag，不跟 `main`） |
| `v4.1` | 固定版本号（与 git tag 一致），生产环境建议锁定 |
| `dev` | `main` 上每次提交的构建，仅供自测 |

`docker-compose.yaml` 中的上游写法（如需回到官方）：`ghcr.io/metatube-community/metatube-server:latest`。

### 离线安装

Releases 页面提供 `docker load` 可用的镜像包，适用于无法访问 GHCR 的内网环境：

```bash
# amd64（<VER> 换成实际版本号，如 4.1）
gunzip -c metatube-server-<VER>-linux-amd64.tar.gz | docker load
# arm64
gunzip -c metatube-server-<VER>-linux-arm64.tar.gz | docker load

docker images | grep metatube-server-ltx   # metatube-server-ltx:v4.1
```

加载出的镜像名称为 `metatube-server-ltx:v4.1`（版本号带 v），把 compose 里的 `image` 换成它即可。
每个镜像包都附带 `.sha256` 文件，下载后可校验：

```bash
sha256sum -c metatube-server-<VER>-linux-amd64.tar.gz.sha256
```

### Linux 裸二进制（不装 Docker）

Release 里另有 `metatube-server-<VER>-linux-amd64-bin.tar.gz` 与 `-arm64-bin.tar.gz`，
解压即得静态编译的服务端（含 `LICENSE`、`VERSION`）：

```bash
tar -xzf metatube-server-<VER>-linux-amd64-bin.tar.gz
./metatube-server -version          # 显示 <VER>
./metatube-server -dsn ./metatube.db -port 8080
```

### Windows 桌面版

不想装 Docker 时，从 Release 下载 `metatube-server.exe` 和 `metatube-panel.exe`，放同一目录：

- 双击 `metatube-panel.exe` 启动图形面板：自动托管服务，可开关/重启；关闭窗口与最小化进托盘，
  右键退出会一并关服务。需要系统已装 Edge WebView2 运行时（Win10/11 一般自带）。
- `metatube-server.exe` 是无界面服务本体，也可单独命令行运行。

首次启动在面板设置里填后端地址与 access token 即可连接；翻译凭据写同目录 `translate.json`
或用 `MT_TRANSLATE_JSON` 环境变量。浏览器打开 `http://127.0.0.1:8080/ui` 访问内置控制台。

## 番号后缀归一化

修复后的 `number.Trim` 行为：

`number.Trim` 识别的尾部词表为 `ltx|restored|basicvsr(?:\+\+)?|wm`，大小写不敏感，
分隔符 `-` / `_` / 空格及其连续重复都支持；从尾部**循环剥离**，多个标记叠加、顺序颠倒都能剥干净。
只剥尾部，番号前缀里的同名字符串不受影响；`wm` 单独出现（如 `ABP-030_wm.mp4`）也会被剥掉。

| 原始文件名 | 归一化结果 |
| --- | --- |
| `ABP-030-LTX.mp4` | `ABP-030` |
| `ABP-030_LTX.mkv` | `ABP-030` |
| `ABP-030-LTX-C.mp4` | `ABP-030` |
| `hhd800.com@SSIS-329-LTX-4K.mp4` | `SSIS-329` |
| `FC2-PPV-123456-LTX.mp4` | `FC2-123456` |
| `ABP-030-BasicVSR++.mp4` | `ABP-030` |
| `ABP-030__restored.mp4` | `ABP-030` |
| `ABP-030-restored.mp4` | `ABP-030` |
| `ABP-030_LTX__restored.mp4` | `ABP-030` |
| `SONE-521_restored.LTX_wm.mp4` | `SONE-521` |
| `JUR-110_restored_wm.mp4` | `JUR-110` |
| `MISM-399_restored_wm.mp4` | `MISM-399` |
| `ABP-030_wm.mp4` / `ABP-030_LTX_wm.mp4` | `ABP-030` |
| `LTX-030.mp4` / `RESTORED-030.mp4` / `WM-030.mp4` | 原样保留（前缀不受影响） |

验证方式：

```bash
curl 'http://127.0.0.1:8080/v1/movies/search?q=ABP-030-LTX&provider=javbus'
# 修复前返回 404；修复后返回 id 为 ABP-030 的正常结果
```

如果你的刮削流程是 MoviePilot 之类的上层应用，它只负责把文件名交给 MetaTube，
**无需修改上层配置**，替换 MetaTube 镜像即生效。

## 构建

版本号由仓库根的 `VERSION` 文件决定（本地开发如 `4.0.1`，发布时如 `4.1.0`）：

```bash
# 本地编译（版本号自动读 VERSION）
VERSION=$(tr -d ' \r\n' < VERSION)
go build -trimpath \
  -ldflags "-X github.com/metatube-community/metatube-sdk-go/internal/version.Version=$VERSION" \
  -o metatube-server ./cmd/server

./metatube-server -version
```

```bash
# 构建镜像
VERSION=$(tr -d ' \r\n' < VERSION)
docker build -t metatube-server-ltx:$VERSION \
  --build-arg VERSION=$VERSION \
  --build-arg COMMIT=$(git rev-parse --short HEAD) .
```

发版流程（`scripts/version.sh` 会做版本校验，CI 会拒绝不自洽的 tag）：

```bash
bash scripts/version.sh bump                 # 本地改动：4.0.0 → 4.0.1
bash scripts/version.sh release --tag        # 发布：4.0.1 → 4.1.0，本地打 tag v4.1
git push mine main && git push mine v4.1     # 需用户明确同意后再推
```

推 tag 后 GitHub Actions 会自动：

1. 构建 `linux/amd64` 与 `linux/arm64` 镜像并推送到 `ghcr.io/sayaxia/metatube-server`
   （标签 `:v<版本>` 与 `:latest`）；
2. 编译 Linux amd64/arm64 裸二进制、导出两个架构的 `docker load` 镜像包、交叉编译
   Windows x64 的两个 exe，连同 `docker-compose.yml`、`INSTALL.md`、`.sha256` 发布到 Releases；
3. Release 说明只取 `notes/<版本>.md` 的**本轮**内容。

## 与上游同步

```bash
git remote add upstream https://github.com/metatube-community/metatube-sdk-go.git
git fetch upstream --tags
git rebase upstream/main        # 或 git merge upstream/main
```

同步上游只涉及代码，版本号继续按本项目自己的节奏推进（本地 `4.1.x` → 发布 `4.2`），
不再跟随上游 tag 命名。
若上游已合并 [#388](https://github.com/metatube-community/metatube-sdk-go/pull/388)，
本分支即可退役，直接切回官方镜像。

## 致谢

| 库 | 说明 |
| --- | --- |
| [gocolly/colly](https://github.com/gocolly/colly) | Go 语言的爬虫框架 |
| [gin-gonic/gin](https://github.com/gin-gonic/gin) | Go 的 HTTP Web 框架 |
| [gorm.io/gorm](https://gorm.io/) | Go 的 ORM 库 |
| [esimov/pigo](https://github.com/esimov/pigo) | 纯 Go 的快速人脸检测与五官定位库 |
| [robertkrimen/otto](https://github.com/robertkrimen/otto) | Go 的 JavaScript 解释器 |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite) | SQLite 的无 CGo 移植版 |
| [corona10/goimagehash](https://github.com/corona10/goimagehash) | Go 的感知图像哈希库 |
| [antchfx/xpath](https://github.com/antchfx/xpath) | 支持 HTML / XML / JSON 查询的 XPath 库 |
| [gen2brain/jpegli](https://github.com/gen2brain/jpegli) | 基于 jpegli 的 JPEG 编解码器 |

## 许可

[Apache-2.0 License](https://github.com/metatube-community/metatube-sdk-go/blob/main/LICENSE)
