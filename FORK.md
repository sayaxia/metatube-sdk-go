# MetaTube SDK Go — LTX Fork

本仓库是 [metatube-community/metatube-sdk-go](https://github.com/metatube-community/metatube-sdk-go)
的个人维护分支（fork），用于解决 AI 马赛克修复视频无法自动刮削的问题。

- 上游基线版本：**v1.4.0**（commit `fdadc9c`）
- 本分支版本号：**v1.4.0-ltx.N**（`N` 为本地修订号，与上游 tag 严格区分）
- 番号修复的上游 PR：[metatube-community/metatube-sdk-go#388](https://github.com/metatube-community/metatube-sdk-go/pull/388)
- Docker 镜像：`ghcr.io/sayaxia/metatube-server:<version>` / `ghcr.io/sayaxia/metatube-server:ltx`

## 改动内容

| 改动 | 说明 | 是否提交上游 |
| --- | --- | --- |
| `common/number` 番号归一化 | 剥离 AI 修复模型追加的后缀（`-LTX`、`-BasicVSR++`），恢复 `ABP-030-LTX → ABP-030` 的匹配 | ✅ 是（PR） |
| `Dockerfile` | 支持通过 build-arg 注入 `VERSION` / `COMMIT`，保证镜像内 `-version` 显示分支版本 | ❌ 仅本分支 |
| `.github/workflows/docker.yml` | 推送到本仓库 GHCR 命名空间（上游固定为 `metatube-community`，fork 无权限） | ❌ 仅本分支 |
| `.github/workflows/release.yml` | tag 触发构建 `docker load` 可用的镜像包并发布到 Releases | ❌ 仅本分支 |
| `docker-compose.yaml` | 默认使用本分支镜像与本地 SQLite | ❌ 仅本分支 |

## 问题背景

Jasna 等去马赛克工具在使用 LTX 模型修复后会输出 `ABC-123-LTX.mp4` 这类文件名。
MetaTube 原有的番号清洗逻辑只剥离 `-C`、`-UC`、`-ch`、`-cdN`、`-A~D`、`-hhbN`
等后缀，`-LTX` 被当作番号的一部分，导致所有 provider 都检索不到结果。

修复后 `number.Trim` 会在检索前把这类后缀去掉：

| 原始文件名 | 归一化结果 |
| --- | --- |
| `ABP-030-LTX.mp4` | `ABP-030` |
| `ABP-030_LTX.mkv` | `ABP-030` |
| `ABP-030-LTX-C.mp4` | `ABP-030` |
| `hhd800.com@SSIS-329-LTX-4K.mp4` | `SSIS-329` |
| `FC2-PPV-123456-LTX.mp4` | `FC2-123456` |
| `LTX-030` | `LTX-030`（前缀不受影响） |

## 安装

见 Releases 页面中的 `INSTALL.md`，或直接：

```bash
docker run -d --name metatube -p 8080:8080 \
  -v metatube-data:/data \
  ghcr.io/sayaxia/metatube-server:ltx \
  -dsn /data/metatube.db -port 8080
```

## 与上游同步

```bash
git remote add upstream https://github.com/metatube-community/metatube-sdk-go.git
git fetch upstream --tags
git rebase upstream/main   # 或 merge
```

同步后把版本号推进为 `v1.4.x-ltx.N`，打 tag 即自动产出新的 Release 资产。
