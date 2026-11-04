# MetaTube SDK Go — LTX Fork

本仓库是 [metatube-community/metatube-sdk-go](https://github.com/metatube-community/metatube-sdk-go)
的个人维护分支（fork），用于解决 AI 马赛克修复视频无法自动刮削的问题。

- 上游基线版本：**v1.4.0**（commit `fdadc9c`），另已同步 `main` 上尚未发版的 `FC2DB` provider（上游提交 `59ac7f2`）
- 本分支版本号：**v1.4.0-ltx.N**（`N` 为本地修订号，与上游 tag 严格区分）
- 番号修复的上游 PR：[metatube-community/metatube-sdk-go#388](https://github.com/metatube-community/metatube-sdk-go/pull/388)
- Docker 镜像：`ghcr.io/sayaxia/metatube-server:<version>` / `ghcr.io/sayaxia/metatube-server:ltx`

## 改动内容

| 改动 | 说明 | 是否提交上游 |
| --- | --- | --- |
| `common/number` 番号归一化 | 剥离 AI 修复模型追加的后缀（`-LTX`、`-BasicVSR++`），恢复 `ABP-030-LTX → ABP-030` 的匹配 | ✅ 是（PR） |
| 同步上游 `FC2DB` provider | FC2 官方详情页改为强制登录后，用 `fc2db.net` 兜底，恢复 FC2-PPV 番号刮削 | ✅ 上游已合并（#389），本分支同步 |
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

## FC2 番号刮不到（2026-11 起）

现象：`FC2-PPV-xxxxxxx` 在 MetaTube 里返回 404，日志中 `FC2` provider 报 `Not Found`，
但作品本身并未下架。

原因：FC2 官方把付费作品详情页改成了「必须登录」。未登录访问
`https://adult.contents.fc2.com/article/<id>/` 会被 302 到
`fc2.com/ja/login.php?ref=payarticle`，最终落到 `error.fc2.com/other/`（HTTP 404）。
官方 `api/v2/videos/<id>/sample` 接口仍然公开，可见作品还在，只是页面不再对匿名游客开放。

影响范围是所有 FC2-PPV 番号，而不是某个编号；与番号归一化、`-LTX` 后缀无关。

解决：同步上游 PR
[#389 `Feature(provider): add FC2DB provider`](https://github.com/metatube-community/metatube-sdk-go/pull/389)
（上游提交 `59ac7f2`）。新增的 `FC2DB` provider 抓取 `fc2db.net`，优先级 `1000 - 3`，
排在 `FC2`、`fc2hub`、`FC2PPVDB` 之后，只在前面全部失败时兜底，因此不会改变已有的正常结果。

```bash
# 验证（v1.4.0-ltx.2 起）
curl 'http://127.0.0.1:8080/v1/movies/search?q=FC2-PPV-4985867'
# {"data":[{"id":"4985867","number":"FC2-4985867","provider":"FC2DB", ...}]}
```

顺带记录：`fc2ppvdb.com` 在部分网络下 TLS 握手失败（Cloudflare 边缘不可达），
`javten.com`（`fc2hub` 的数据源）对非浏览器请求返回 403，所以 `FC2DB` 目前是最可靠的兜底源。

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
