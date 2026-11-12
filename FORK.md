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
| `common/number` 番号归一化 | 剥离 AI 修复模型追加的后缀（`-LTX`、`-BasicVSR++`、`-restored` / `__restored`）及后续工具标记（`_wm` / `.LTX_wm`），恢复 `ABP-030-LTX → ABP-030` 的匹配 | ✅ 是（PR） |
| 同步上游 `FC2DB` provider | FC2 官方详情页改为强制登录后，用 `fc2db.net` 兜底，恢复 FC2-PPV 番号刮削 | ✅ 上游已合并（#389），本分支同步 |
| `Dockerfile` | 支持通过 build-arg 注入 `VERSION` / `COMMIT`，保证镜像内 `-version` 显示分支版本 | ❌ 仅本分支 |
| 镜像默认 `-tags experimental` | 编入实验池 provider（AVBASE / ThePornDB / AV-LEAGUE），见下节 | ❌ 仅本分支 |
| `.github/workflows/docker.yml` | 推送到本仓库 GHCR 命名空间（上游固定为 `metatube-community`，fork 无权限） | ❌ 仅本分支 |
| `.github/workflows/release.yml` | tag 触发构建 `docker load` 可用的镜像包并发布到 Releases | ❌ 仅本分支 |
| `docker-compose.yaml` | 默认使用本分支镜像与本地 SQLite | ❌ 仅本分支 |
| `translate/baiduai` translator | 新增百度**大模型文本翻译**（AIT）引擎：端点 `fanyi-api.baidu.com/ait/api/aiTextTranslate`，与旧 `baidu` 引擎共用同一 APPID/密钥的 sign 鉴权与返回格式，参数键名一致（`baidu-app-id` / `baidu-app-key`，可选 `baidu-model-type=nmt|llm`，默认 `nmt`）。它是有独立免费额度的商品，通用翻译额度耗尽时可切换 | ❌ 仅本分支 |

## 问题背景

Jasna 等去马赛克工具在使用 LTX / BasicVSR 模型修复后会输出 `ABC-123-LTX.mp4`、
`ABC-123-BasicVSR++.mp4` 这类文件名，另一些工具写 `ABC-123__restored.mp4`。
这些后缀在业务上**都表示无码破解**（Plex 插件侧统一用 `u.png` 角标和「无码破解」类型标签），
分隔符 `-` / `_` / 空格以及连续重复都不影响识别。

工具还会把多个标记**叠加**在角标后面，例如 `JUR-110_restored_wm.mp4`、
`SONE-521_restored.LTX_wm.mp4`（`_wm` 是带水印压制版的标记，`.LTX` 用点号分隔）。
此时 `restored` 不再是最后一个词元，只认「结尾必须是角标」的规则会整体漏掉，
番号变成 `JUR-110_restored_wm` → provider 返回一堆无关结果。

MetaTube 原有的番号清洗逻辑只剥离 `-C`、`-UC`、`-ch`、`-cdN`、`-A~D`、`-hhbN`
等后缀，`-LTX` 被当作番号的一部分，导致所有 provider 都检索不到结果；
`-restored` 更会被末尾的 `A|B|C|D` 规则吃掉一个字母，变成 `ABC-123-restore`。

修复后 `number.Trim` 会在检索前循环剥掉 `ltx|restored|basicvsr(?:\+\+)?|wm`
这类尾部标记（顺序无关），直到剩下裸番号：

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
| `LTX-030` / `RESTORED-030` / `WM-030` | 原样保留（前缀不受影响） |

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

## 实验池 provider（`-tags experimental`）

上游把一部分稳定性未知的 provider 放在 `engine/register_exp.go`，只有 `-tags experimental`
才会编入。默认构建没有它们，于是**引用过这些 provider 的旧条目会永久刮削失败**：
Plex 侧日志报 `APIError: 404 <provider not found>`，刷新 5 毫秒就结束，元数据保持旧值。

本分支的镜像默认带 `-tags experimental`（`Dockerfile` 的 `ARG BUILD_TAGS=experimental`），
`docker.yml` / `release.yml` 也会显式传入，保证 CI 与本地一致。

| provider | 类型 | 优先级 | 说明 |
| --- | --- | --- | --- |
| `AVBASE` | movie | `1000 - 4` | 番号覆盖广，素人/无码配信常有演员和简介；排在 FANZA（1001）之后、JavBus（995）之前 |
| `ThePornDB` | movie ×2 + actor | — | `ThePornDBMovie` / `ThePornDBScene` / `ThePornDBActor` |
| `AV-LEAGUE` | actor | `1000` | 仅演员，不参与番号检索 |
| `MadouQu` | movie | `0` | `Priority = 0` 表示默认停用，即使编入也不会出现（需要 `MT_MOVIE_PROVIDER_MADOUQU__PRIORITY` 才启用） |

编入后的实际差异（`/v1/providers`，2026-11 实测）：

```
默认构建: movie 31 / actor 1  (Gfriends)
实验构建: movie 34 / actor 3  (+AVBASE, +ThePornDBMovie, +ThePornDBScene,
                               +AV-LEAGUE, +ThePornDBActor)
```

只增不减，默认池的 31 个全部保留；优先级也不冲突，FANZA 依旧排在 AVBASE 前面，
所以正常新片的搜索结果顺序不变。想让某个 provider 的优先级生效或失效，用环境变量
`MT_MOVIE_PROVIDER_<NAME>__PRIORITY` 覆盖即可，不必重新构建。

需要退回纯上游构建时：

```bash
docker build --build-arg BUILD_TAGS= -t metatube-server:plain .
# 或本地
make server            # 无 tag
make server-experimental  # 带实验池
```

### 排查「刷新瞬间结束」

```bash
curl -s 'http://127.0.0.1:8080/v1/movies/AVBASE/<id>'    # 该 provider 是否存在且有数据
docker logs metatube 2>&1 | grep 'Disable movie provider'  # 被停用的 provider 会在这里出现
```

`Disable movie provider: MadouQu` 属于预期输出（`Priority = 0`），不是故障。

## 安装

见 Releases 页面中的 `INSTALL.md`，或直接：

```bash
docker run -d --name metatube -p 8080:8080 \
  -v metatube-data:/data \
  ghcr.io/sayaxia/metatube-server:ltx \
  -dsn /data/metatube.db -port 8080
```

### 群晖 DSM 6 注意事项

DSM 6 的 Docker 套件停留在 **18.09.8 + docker-compose 1.24**，有四个坑已经在本分支处理：

| 问题 | 处理 |
| --- | --- |
| 没有 `docker compose` 子命令 | 用 `docker-compose`（v1） |
| compose v1 缺 `version:` 会按 legacy 格式解析而报错 | Release 里的 `docker-compose.yml` 已带 `version: "3.8"` |
| Docker 18.09 不认 OCI index，pull 会报 `no matching manifest` / `unsupported media type` | 自 `v1.4.0-ltx.4` 起镜像以 Docker manifest list 发布（`oci-mediatypes=false`），18.09 可正常 pull |
| GUI 只能导入未压缩 `.tar`，且注册表搜不到 ghcr.io | 离线包先 `gunzip -c x.tar.gz > x.tar` 再导入，或 SSH 里 `docker load -i x.tar.gz` |


## 百度大模型翻译引擎（`baiduai`）

通用文本翻译（旧 `baidu` 引擎）的免费额度用尽时，可切换到
[大模型文本翻译 API](https://api.fanyi.baidu.com/doc/21)（独立商品、独立额度）：

1. 在百度翻译开放平台**开通「大模型文本翻译API」服务**（需要个人/企业认证）；
2. Plex 插件里把翻译引擎从 `baidu` 改成 `baiduai`，参数表**原样保留**
   （`baidu-app-id=...` / `baidu-app-key=...` 与旧引擎共用，不用改）；
3. 可选 `baidu-model-type=llm` 走大模型模式；默认 `nmt`（机器翻译模式，
   个人认证每月 100 万免费字符）。

验证（NAS 上直接调）：

```bash
curl "http://127.0.0.1:8080/v1/translate?q=%E6%B5%8B%E8%AF%95&to=zh&engine=baiduai&baidu-app-id=APPID&baidu-app-key=KEY"
```

## 与上游同步

```bash
git remote add upstream https://github.com/metatube-community/metatube-sdk-go.git
git fetch upstream --tags
git rebase upstream/main   # 或 merge
```

同步后把版本号推进为 `v1.4.x-ltx.N`，打 tag 即自动产出新的 Release 资产。
