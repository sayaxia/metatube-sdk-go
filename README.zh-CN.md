# MetaTube SDK Go（LTX 分支）

[![Build Status](https://img.shields.io/github/actions/workflow/status/sayaxia/metatube-sdk-go/release.yml?branch=main&style=flat-square&logo=github-actions&label=release)](https://github.com/sayaxia/metatube-sdk-go/actions/workflows/release.yml)
[![Require Go Version](https://img.shields.io/badge/go-%3E%3D1.25-30dff3?style=flat-square&logo=go)](https://github.com/sayaxia/metatube-sdk-go/blob/main/go.mod)
[![GitHub License](https://img.shields.io/github/license/sayaxia/metatube-sdk-go?color=e4682a&logo=apache&style=flat-square)](https://github.com/sayaxia/metatube-sdk-go/blob/main/LICENSE)
[![Tag](https://img.shields.io/github/v/tag/sayaxia/metatube-sdk-go?color=%23ff8936&logo=fitbit&style=flat-square)](https://github.com/sayaxia/metatube-sdk-go/tags)

> **这是 [metatube-community/metatube-sdk-go](https://github.com/metatube-community/metatube-sdk-go) 的个人分支（fork，版本号 `v1.4.0-ltx.N`）**，维护于
> [sayaxia/metatube-sdk-go](https://github.com/sayaxia/metatube-sdk-go)。
> 相对上游只增加一项能力：识别带 AI 修复后缀的文件名（如 `ABP-030-LTX.mp4`）。
> 其余功能与上游完全一致，**上游始终是唯一事实来源**。
> 完整差异、版本规则与 Docker 说明见 [FORK.md](FORK.md)。

English version: [README.md](README.md)

## 目录

- [这个分支解决什么问题](#这个分支解决什么问题)
- [特性](#特性)
- [安装](#安装)
    - [作为 Go SDK 使用](#作为-go-sdk-使用)
    - [Docker 部署（推荐）](#docker-部署推荐)
    - [离线安装](#离线安装)
- [番号后缀归一化](#番号后缀归一化)
- [构建](#构建)
- [与上游同步](#与上游同步)
- [致谢](#致谢)
- [许可](#许可)

## 这个分支解决什么问题

Jasna 等去马赛克工具在使用 **LTX** 模型修复后，会把模型名写进输出文件名：

```
修复前：ABP-030.mp4
修复后：ABP-030-LTX.mp4
```

MetaTube 在检索前会对文件名做番号归一化，但原逻辑只剥离 `-C`、`-UC`、`-ch`、`-cdN`、`-A`~`-D`、`-hhbN`
等已知后缀，`-LTX` 被当成番号的一部分，于是所有 provider 都在搜 `ABP-030-LTX` → 返回空结果 → 自动刮削失败。

本分支在归一化阶段把这类"修复模型后缀"一并剥离，`ABP-030-LTX` 会被正确识别为 `ABP-030`。
该修复已作为 PR 提交上游：[#388](https://github.com/metatube-community/metatube-sdk-go/pull/388)；
上游合并后即可直接用官方镜像，无需继续使用本分支。

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

如需使用本分支的修复，可在 `go.mod` 中替换为本地源码或本仓库路径：

```
replace github.com/metatube-community/metatube-sdk-go => github.com/sayaxia/metatube-sdk-go v1.4.0-ltx.1
```

### Docker 部署（推荐）

镜像已发布到本仓库的 GHCR 命名空间，**公开可拉取，无需登录**：

```bash
docker run -d --name metatube -p 8080:8080 \
  -v metatube-data:/data \
  ghcr.io/sayaxia/metatube-server:ltx \
  -dsn /data/metatube.db -port 8080
```

或使用 `docker-compose.yaml`（默认已指向本分支镜像，并把 SQLite 落到 `./data`）：

```yaml
services:
  metatube:
    image: ghcr.io/sayaxia/metatube-server:ltx
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
| `ltx` | 本分支最新构建（跟随 `main`） |
| `1.4.0-ltx.1` | 固定版本，生产环境建议锁定 |

`docker-compose.yaml` 中的上游写法（如需回到官方）：`ghcr.io/metatube-community/metatube-server:latest`。

### 离线安装

Releases 页面提供 `docker load` 可用的镜像包，适用于无法访问 GHCR 的内网环境：

```bash
# amd64
gunzip -c metatube-server-1.4.0-ltx.1-linux-amd64.tar.gz | docker load
# arm64
gunzip -c metatube-server-1.4.0-ltx.1-linux-arm64.tar.gz | docker load

docker images | grep metatube-server   # metatube-server:1.4.0-ltx.1
```

加载出的镜像名称为 `metatube-server:1.4.0-ltx.1`，把 compose 里的 `image` 换成它即可。
每个镜像包都附带 `.sha256` 文件，下载后可校验：

```bash
sha256sum -c metatube-server-1.4.0-ltx.1-linux-amd64.tar.gz.sha256
```

## 番号后缀归一化

修复后的 `number.Trim` 行为：

| 原始文件名 | 归一化结果 |
| --- | --- |
| `ABP-030-LTX.mp4` | `ABP-030` |
| `ABP-030_LTX.mkv` | `ABP-030` |
| `ABP-030-LTX-C.mp4` | `ABP-030` |
| `hhd800.com@SSIS-329-LTX-4K.mp4` | `SSIS-329` |
| `FC2-PPV-123456-LTX.mp4` | `FC2-123456` |
| `LTX-030.mp4` | `LTX-030`（前缀不受影响） |

验证方式：

```bash
curl 'http://127.0.0.1:8080/v1/movies/search?q=ABP-030-LTX&provider=javbus'
# 修复前返回 404；修复后返回 id 为 ABP-030 的正常结果
```

如果你的刮削流程是 MoviePilot 之类的上层应用，它只负责把文件名交给 MetaTube，
**无需修改上层配置**，替换 MetaTube 镜像即生效。

## 构建

```bash
# 本地编译（注入分支版本号）
go build -trimpath \
  -ldflags "-X github.com/metatube-community/metatube-sdk-go/internal/version.Version=1.4.0-ltx.1" \
  -o metatube-server ./cmd/server

./metatube-server -version
```

```bash
# 构建镜像
docker build -t metatube-server:1.4.0-ltx.1 \
  --build-arg VERSION=1.4.0-ltx.1 \
  --build-arg COMMIT=$(git rev-parse --short HEAD) .
```

打 tag（形如 `v1.4.0-ltx.2`）并推送后，GitHub Actions 会自动：

1. 构建 `linux/amd64` 与 `linux/arm64` 镜像并推送到 `ghcr.io/sayaxia/metatube-server`；
2. 导出两个架构的 `docker load` 镜像包，附 `docker-compose.yml`、`INSTALL.md` 与 `.sha256`，发布到 Releases。

## 与上游同步

```bash
git remote add upstream https://github.com/metatube-community/metatube-sdk-go.git
git fetch upstream --tags
git rebase upstream/main        # 或 git merge upstream/main
```

同步后把版本号推进为 `v1.4.x-ltx.N`，打 tag 即可产出对应的新 Release。
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
