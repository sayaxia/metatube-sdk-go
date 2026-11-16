# metatube-server-ltx

本仓库是 [metatube-community/metatube-sdk-go](https://github.com/metatube-community/metatube-sdk-go)
的个人维护分支（fork），用于解决 AI 马赛克修复视频无法自动刮削的问题。

- 上游基线版本：**v1.4.0**（commit `fdadc9c`），另已同步 `main` 上尚未发版的 `FC2DB` provider（上游提交 `59ac7f2`）
- 项目名：**metatube-server-ltx**，版本号与上游彻底解耦：**`v4.N`**（规则见下节）
- Docker 镜像：`ghcr.io/sayaxia/metatube-server:4.1`（固定版本）/ `:latest`（最新发布）
- 番号修复的上游 PR：[metatube-community/metatube-sdk-go#388](https://github.com/metatube-community/metatube-sdk-go/pull/388)

## 版本号与发版

`MAJOR` 固定为 4（承接上游 v1.4.0 的基线），`MINOR` / `PATCH` 按下面的节奏走：

| 阶段 | 版本号 | 说明 |
| --- | --- | --- |
| 本地改动 | `4.0.1`、`4.0.2` … | 每改一轮 PATCH +1，只存在于本地 |
| 推送发版 | `v4.1`、`v4.2` … | MINOR +1、PATCH 归零，打**两点式** tag |

发布后本地基线变成 `4.1.0`，继续改就是 `4.1.1`、`4.1.2` …，下次发布即 `4.2`。

- **唯一来源**：仓库根 `VERSION`。`scripts/version.sh` 提供 `show` / `bump` / `release` / `verify`
  四个子命令，`release` 会自动校验 `notes/<版本>.md` 是否存在。
- **tag**：`v4.1`（两点式，不带 PATCH）；镜像 tag 不带 `v`。
- **镜像标签**：`:<版本>` + `:latest`；旧 `:ltx` 自 v4.1 起**停止更新**（历史镜像仍可拉取）。
- **发布说明**：只写本轮改动，放在 `notes/<版本>.md`；CI 缺这个文件会直接失败，
  不会出现"沿用上版说明"的情况。
- **发布产物**：Linux amd64/arm64 裸二进制包、Linux amd64/arm64 的 `docker load` 镜像包、
  Windows x64 服务端与面板，外加 `docker-compose.yml` / `INSTALL.md` / `.sha256`。

> **改动本仓库前请先读 [`AGENTS.md`](AGENTS.md)**（协作规约、目录地图、验证清单、发版流程，
> 含多智能体协作规则），改完在 [`AGENT_LOG.md`](AGENT_LOG.md) 末尾追加一条记录。
> 本文件面向使用者，`AGENTS.md` 面向维护者。

## 改动内容

| 改动 | 说明 | 是否提交上游 |
| --- | --- | --- |
| `common/number` 番号归一化 | 剥离 AI 修复模型追加的后缀（`-LTX`、`-BasicVSR++`、`-restored` / `__restored`）及后续工具标记（`_wm` / `.LTX_wm`），恢复 `ABP-030-LTX → ABP-030` 的匹配 | ✅ 是（PR） |
| 同步上游 `FC2DB` provider | FC2 官方详情页改为强制登录后，用 `fc2db.net` 兜底，恢复 FC2-PPV 番号刮削 | ✅ 上游已合并（#389），本分支同步 |
| `Dockerfile` | 支持通过 build-arg 注入 `VERSION` / `COMMIT`，保证镜像内 `-version` 显示分支版本 | ❌ 仅本分支 |
| 镜像默认 `-tags experimental` | 编入实验池 provider（AVBASE / ThePornDB / AV-LEAGUE），见下节 | ❌ 仅本分支 |
| `.github/workflows/docker.yml` | 推送到本仓库 GHCR 命名空间（上游固定为 `metatube-community`，fork 无权限）；发布版打 `:<版本>` + `:latest` | ❌ 仅本分支 |
| `.github/workflows/release.yml` | tag 触发：编译 Linux amd64/arm64 裸二进制、导出两个架构的 `docker load` 镜像包、交叉编译 Windows x64 两个 exe，按 `notes/<版本>.md` 生成本轮说明并发布 Releases | ❌ 仅本分支 |
| `VERSION` / `scripts/version.sh` / `notes/` | 独立版本号体系（`4.x`，发布 = MINOR+1）与每轮发布说明；tag 为两点式 `v4.1` | ❌ 仅本分支 |
| `docker-compose.yaml` | 默认使用本分支镜像与本地 SQLite | ❌ 仅本分支 |
| `translate/baiduai` translator | 新增百度**大模型文本翻译**（AIT）引擎：端点 `fanyi-api.baidu.com/ait/api/aiTextTranslate`，与旧 `baidu` 引擎共用同一 APPID/密钥的 sign 鉴权与返回格式，参数键名一致（`baidu-app-id` / `baidu-app-key`，可选 `baidu-model-type=nmt|llm`，默认 `nmt`）。它是有独立免费额度的商品，通用翻译额度耗尽时可切换 | ❌ 仅本分支 |
| `route/translateconfig.go` | 控制台的翻译引擎凭据落在服务端 `translate.json`（热重载、密钥回打码值），暴露私有接口 `GET/PUT /v1/translate/config`；顺带把 `error_code` 改成同时接受字符串与数字，`<engine>: <错误原文>` 不再被解码失败吃掉 | ❌ 仅本分支 |

## 问题背景

Jasna 等去马赛克工具在使用 LTX / BasicVSR 模型修复后会输出 `ABC-123-LTX.mp4`、
`ABC-123-BasicVSR++.mp4` 这类文件名，另一些工具写 `ABC-123__restored.mp4`。
这些后缀在业务上**都表示无码破解**（Plex 插件侧统一用 `u.png` 角标和「无码破解」类型标签），
分隔符 `-` / `_` / 空格以及连续重复都不影响识别。

工具还会把多个标记**叠加**在角标后面，例如 `JUR-110_restored_wm.mp4`、
`ABC-123_restored.LTX_wm.mp4`（`_wm` 是带水印压制版的标记，`.LTX` 用点号分隔）。
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
| `ABC-123_restored.LTX_wm.mp4` | `ABC-123` |
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
# 验证
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
  ghcr.io/sayaxia/metatube-server:latest \
  -dsn /data/metatube.db -port 8080
```

### 群晖 DSM 6 注意事项

DSM 6 的 Docker 套件停留在 **18.09.8 + docker-compose 1.24**，有四个坑已经在本分支处理：

| 问题 | 处理 |
| --- | --- |
| 没有 `docker compose` 子命令 | 用 `docker-compose`（v1） |
| compose v1 缺 `version:` 会按 legacy 格式解析而报错 | Release 里的 `docker-compose.yml` 已带 `version: "3.8"` |
| Docker 18.09 不认 OCI index，pull 会报 `no matching manifest` / `unsupported media type` | 本分支所有镜像均以 Docker manifest list 发布（`oci-mediatypes=false`），18.09 可正常 pull |
| GUI 只能导入未压缩 `.tar`，且注册表搜不到 ghcr.io | 离线包先 `gunzip -c x.tar.gz > x.tar` 再导入，或 SSH 里 `docker load -i x.tar.gz` |


## 百度大模型翻译引擎（`baiduai`）

通用文本翻译（旧 `baidu` 引擎）的免费额度用尽时，可切换到
[大模型文本翻译 API](https://api.fanyi.baidu.com/doc/21)（独立商品、独立额度）：

1. 在百度翻译开放平台**开通「大模型文本翻译API」服务**（需要个人/企业认证）；
2. Plex 插件里把翻译引擎从 `baidu` 改成 `baiduai`，参数表**原样保留**
   （`baidu-app-id=...` / `baidu-app-key=...` 与旧引擎共用，不用改）；
3. 模型二选一，两种方式都行：
   - **Plex 设置里的开关** `BaiduAI: use LLM translation`（默认关闭）：
     关 = `nmt` 机器翻译，开 = `llm` 大模型翻译；
   - **参数表强制指定** `baidu-model-type=llm|nmt` 追加到 `translation_engine_parameters`，
     优先级高于开关（适合脚本化/临时切换）。

   两个模型**各占一个独立免费额度池**（控制台的「剩余免费额度」按模型分标签显示）：
   `nmt` 为个人认证 100 万字符/月；`llm` 在定价页写作「100 万字符**测试额度**（认证后
   自动发放）」、未标周期，平台通用规则是免费额度每月 1 日 0 点刷新
   （[doc/49](https://api.fanyi.baidu.com/doc/49)）。超出后两者同价 49 元/百万字符。
   实际剩余量以控制台为准。

   插件侧只需这两项，后端 `baiduai` 引擎本身已支持该参数，不必改后端。

验证（NAS 上直接调）：

```bash
curl "http://127.0.0.1:8080/v1/translate?q=%E6%B5%8B%E8%AF%95&to=zh&engine=baiduai&baidu-app-id=APPID&baidu-app-key=KEY"
```

## 出站代理（默认跟随系统设置）

`common/fetch` 用的是 hashicorp `cleanhttp`，它**故意清空** `Transport.Proxy`，所以
`HTTP_PROXY` / `HTTPS_PROXY` 环境变量对它无效，以前只能逐个 provider 注入
`MT_<TYPE>_PROVIDER_<NAME>__PROXY=...`（34 个源就要写 34 行）。

现在改为**默认跟随操作系统的代理设置**，与浏览器行为一致：你在「设置 → 网络和 Internet →
代理」里开着，server 就走；你关掉，server 就直连。程序**只读取，从不写入**该设置，
也从不替你开或关代理。

| 变量 | 作用 |
|---|---|
| （不设置） | 跟随系统代理，默认行为 |
| `MT_PROXY_URL` | 钉死一个代理覆盖所有 provider，例如 `http://127.0.0.1:7897` |
| `MT_SYSTEM_PROXY=0` | 仅本次进程忽略系统代理（等价 `MT_NO_PROXY=1`） |

优先级：`MT_<TYPE>_PROVIDER_<NAME>__PROXY` > `MT_PROXY_URL` > 系统代理 > 直连。

解析规则：

- **Windows**：只读注册表
  `HKCU\Software\Microsoft\Windows\CurrentVersion\Internet Settings` 的 `ProxyEnable` /
  `ProxyServer`，`ProxyEnable=0` 即直连；绕过列表取 `ProxyOverride`（`<local>`、`192.168.*`
  等按 Windows 语义匹配）。**刻意不看环境变量** —— Windows 上的 `HTTPS_PROXY` 往往是
  某个进程临时注入的（例如本次会话注入的 `127.0.0.1:13149`），跟着它走会让 provider 从
  错误地区出网。需要压过系统代理时用 `MT_PROXY_URL`。
- **Linux / macOS / Docker**：依次读 `HTTPS_PROXY` / `HTTP_PROXY` / `ALL_PROXY`，
  绕过列表读 `NO_PROXY`。
- `AutoConfigURL`（PAC 脚本）无法在没有 JS 引擎的情况下求值，若检测到会在日志里明说，而不是
  默默按直连处理。

**每 10 秒复查一次**，所以运行中改系统代理即时生效，不用重启。

还有一条容易漏的通道：一部分 provider 基于 colly，它的 collector 没有设 `Transport`，
请求会落到 `http.DefaultTransport` —— 而那个默认读 `HTTPS_PROXY` 等环境变量。结果是同一个
进程里两批 provider 从两个不同出口出去（实测：fetch 那批走系统代理正常，colly 那批走了
某个被注入的代理，整批抓空）。`fetch.InstallDefaultTransport()` 在启动时把同一个选择器挂到
`http.DefaultTransport` 上，保证所有 provider 走同一个决定；provider 级的
`MT_<TYPE>_PROVIDER_<NAME>__PROXY` 仍然优先。

启动日志会写明当前判定和来源：

```text
[ENGINE] ... Outbound proxy: http://127.0.0.1:7897 (via Windows Internet Settings; 22 bypass rule(s))
[ENGINE] ... Outbound proxy: direct (system proxy off)
[ENGINE] ... Outbound proxy: direct (system proxy ignored: MT_SYSTEM_PROXY=0)
[ENGINE] ... Outbound proxy: direct (PAC script configured but unsupported: http://...)
```

运行中开关代理、换代理地址或端口，会在 10 秒内生效，并打一行变更日志，便于区分
「代理断了」和「站点挂了」：

```text
[ENGINE] ... Outbound proxy changed: http://127.0.0.1:7897 (via Windows Internet Settings; 22 bypass rule(s)) -> direct (system proxy off)
```

注意 **Linux / Docker 没有热切换**：代理在容器里只能通过环境变量给，而环境变量在进程启动时
固化，改宿主的 env 对已运行的容器无效，只能重建容器；或者干脆让出口在容器外面（网关分流），
容器侧永远直连。

> 实测（2026-11-15）：DMM/FANZA 在这台机器上**直连必然被地区封锁**
> （跳到 `special.fanza.jp/not-available-in-your-region`），必须经系统代理 `127.0.0.1:7897`
> 才能取到数据。同一台机器上 `127.0.0.1:13149`（会话注入的环境变量代理）出口同样被封锁。
> Docker / NAS 侧的直连不通也属同类问题。

交叉编译 Windows 单文件（纯 Go，无需 CGO / mingw）：

```bash
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -tags experimental \
  -o metatube-server.exe ./cmd/server

# 桌面面板（WebView2 窗口 + 托盘 + 服务托管），GUI 子系统，无控制台窗口
CGO_ENABLED=0 GOOS=windows GOARCH=amd64 go build -trimpath -tags experimental \
  -ldflags "-w -s -H windowsgui" -o metatube-panel.exe ./cmd/panel
```

## 桌面面板（`metatube-panel.exe`）

面板把同一个控制台页面装进一个真正的桌面窗口，并负责起停后端服务。双击 `start.bat` 时优先
启动它；没有它（或设了 `MT_NO_PANEL=1`）才回到「控制台里直接跑服务端」的老路子。

| 行为 | 说明 |
|---|---|
| 窗口 | WebView2 窗口（需要系统已装 Edge WebView2 运行时，Win10/11 一般自带），标题 `MetaTube 控制台`，使用官方 logo 图标 |
| 关窗 / 最小化 | 都只是**收进托盘**，服务照常跑；点托盘图标恢复窗口 |
| 托盘右键 | 打开面板 / 启动服务 / 停止服务 / 重启服务 / 退出 |
| 退出 | 面板与服务一起关。左栏底部也有「退出面板与服务」按钮 |
| 服务托管 | 面板用 `--port <service_port> --dsn data/metatube.db --request-timeout 60s` 起 `metatube-server.exe`，输出写进 `metatube-service.log`；端口被外部实例占用时标成「外部实例」，不抢不停 |
| 单实例 | 再开一次只会把已有窗口叫到前台 |

面板自己也跑一个小 HTTP 服务（默认 `127.0.0.1:8918`），页面、控制接口和 `/v1` 代理都由它提供，
所以**服务停了页面还在**——这正是能「重启服务」的前提。

```text
GET  /              控制台页面（就是 /ui 那份 HTML）
GET  /panel/info    面板版本、后端地址、服务状态（页面每 2.5 秒轮询）
GET  /panel/config  读后端地址 / 是否有 token / 服务端口与程序路径
POST /panel/config  {backend, token, token_set, service_port} → 存 panel.json 并立即生效
POST /panel/service/{start|stop|restart}
POST /panel/exit    退出面板并关服务
POST /panel/focus   把窗口叫到前台
ANY  /v1/*          反向代理到 backend，并把 token 以 Bearer 注入
```

因为 `/v1` 是代理，页面里**不用填地址也不用填 token**：token 只存在 `panel.json`，由面板附加到
每个转发请求上。配置在面板里点右上角齿轮改（也可以直接编辑 `panel.json`）：

```json
{
  "backend": "http://127.0.0.1:8080",
  "token": "",
  "service_exe": "metatube-server.exe",
  "service_port": 8080,
  "panel_port": 8918,
  "auto_start": true
}
```

- `auto_start` 只在 `backend` 指向本机同端口（`127.0.0.1:<service_port>`）时才真的去起服务；
  指向 NAS 时不碰本机进程。
- token 只在后端是本机时才会同时传给服务端（`--token`），保证「面板能进、别人不能进」一致。

页面被面板托管时会自动认出来（探测 `/panel/info`）：左栏底部多出「本机服务」控制块，顶部只显示
**连上的后端**，设置收进右上角齿轮里。用浏览器直接打开 `/ui` 时没有这些控制项，行为与以前一致。

### 应用图标

两个 exe 都内嵌官方 logo（`logo-vertical-dark.png`，16/20/24/32/40/48/64/128/256 共 9 个尺寸），
资源管理器、任务栏、Alt-Tab 和托盘用的是同一枚图标；之前启动时用代码画出来的占位图已移除。
四角按 `ICON_CORNER_RADIUS = 0.25` 裁成圆角（4x 超采样后写进 ICO 像素，不依赖系统圆角），
所以 16px 托盘图标也有 4px 圆角、不会看成尖角，256px 下 Meta/Tube 仍完整不被切到；
想改圆角大小改这个常量再跑一次生成脚本。

资源由 `internal/iconres/gen-icon.py` 生成，链路是：

```text
官方 logo → metatube.png (256px master) → metatube.ico (9 尺寸，32bpp DIB)
                                       └→ windres -F pe-x86-64
                                          → cmd/{panel,server}/icon_windows_amd64.syso
```

`.syso` 是带 `.rsrc` 段的 COFF 对象；Go 只在 `windows/amd64` 下自动链接包目录里的 `.syso`，
其它目标直接忽略，所以 `go build` 不需要额外步骤，Linux / Docker 构建也不受影响。面板运行时
通过 `go:embed` 读同一份 ICO，用 `CreateIconFromResourceEx` 生成 16 / 32 / 48 三个 HICON，
分别给窗口小图标、托盘和窗口大图标。

换 logo 时重新生成资源再构建两个 exe：

```bash
python internal/iconres/gen-icon.py --download          # 从上游重新拉官方图
python internal/iconres/gen-icon.py --source new-logo.png
```

Windows 会缓存 exe 图标，替换文件后如果资源管理器还显示旧的，重启资源管理器或注销一次即可。

### 高 DPI

窗口先用 `SetProcessDpiAwarenessContext(PER_MONITOR_AWARE_V2)` 声明自己按物理像素绘制
（失败时依次回退到 `SetProcessDpiAwareness` / `SetProcessDPIAware`），再按 `GetDpiForSystem`
把窗口尺寸从 DIP 换算成物理像素。

这两步缺一不可，否则 4K 屏开 175% 缩放时会得到两种坏结果：不声明 DPI 时整窗被系统位图放大，
字全糊；只声明不换算时窗口会缩成 1380 物理像素（看起来只有一半大）。页面的
`-webkit-font-smoothing` 也从 `antialiased` 改成 `auto`——Windows 上强制灰度抗锯齿会把字压细。

## 内置控制台（`/ui`）

服务端自带一个单文件控制台，编译时嵌进二进制（`route/ui` + `go:embed`），所以 Windows 版
仍然是**一个 exe**：不用另装前端、不联网也能打开。

```text
http://127.0.0.1:8080/ui
```

面板用的是同一份页面，所以两处看到的界面一致；用浏览器直接打开 `/ui` 时不带服务控制块。

五个视图：

| 视图 | 作用 |
|---|---|
| 状态 | 版本、引擎、运行时长、DB 驱动、平台、源数量；**出站代理**单独用一张卡显示当前判定，直连时标黄提醒。10 秒自动刷新，改系统代理能立刻看到。页面底部是**最近搜索**，一次搜索一行（见下） |
| 搜索 | 按番号或 URL 查询。两种模式：**聚合搜索**（一次请求，服务端并发打全部源）与**逐源探测**（每个源单独发请求，列出成功/失败、耗时、条数、错误原因）。带**自动翻译**开关 |
| 源健康 | 全量探测每个 provider，用来区分「站点挂了」「地区封锁」「解析失败」 |
| 翻译 | 直接调 `/v1/translate`，验证引擎和凭据是否可用 |
| 日志 | 最近 500 条请求的完整列表，可按「仅错误 / 慢请求 ≥1s / 仅元数据」筛选并用番号、源或路径过滤。放在最后一项 |

支持直接带参数打开，便于把诊断结果发给别人：

```text
/ui?q=ABC-123&mode=probe     # 打开即跑逐源探测
/ui#providers                 # 直接进源健康页
/ui#logs                      # 直接进请求日志
```

### 最近搜索：一次查询 = 一行

总览页不显示原始请求行，而是把同一个番号的请求**折叠成一次查询**：

```text
ABC-123  [成功]  3 条结果 · 3 个来源 · 5 次请求      12 秒前   6.12 s
[AVSOX] [DMM] [JavBus]
```

- 合并键是番号（大写归一化），间隔超过 10 分钟的重复搜索算两次
- 逐源探测产生的 34 条请求会合成一条，不再刷满整页
- 点开是一条友好详情：这次查询找到的作品（封面 + 标题 + 番号 + 发售日期，点开进该源的完整详情），
  以及每个来源的表现（正常 / HTTP 码 / 条数 / 耗时 / 错误原因）
- 弹窗右上角可切到**原始 JSON** 核对字段

服务端为此在 `/v1/logs` 的响应里多带一个 `sessions` 数组（始终全量重建，不受 `since` 影响）：

```json
{"q":"ABC-123","ok":true,"items":3,"ms":6120,"requests":5,
 "providers":["AVSOX","DMM","JavBus"],
 "sources":[{"provider":"DMM","status":200,"ms":3400,"items":1},
            {"provider":"FC2","status":404,"error":"Not Found"}],
 "results":[{"provider":"JavBus","id":"...","number":"ABC-123","title":"...","release":"2024-01-05"}]}
```

### 请求日志

每条请求都在内存里留一条记录（环形缓冲，最多 500 条，不落盘、重启即清），字段面向展示而不是
导出：时间、方法、路径、匹配到的路由、业务分类、provider / id / 番号、状态码、耗时、响应字节数、
来源 IP、是否带 token，以及**搜索命中条数**、**结果摘要**和**错误信息**。

`requestLogger` 中间件为此在响应写入时按需截留一小段 body：成功的 `/search` 留最多 4 MiB 用来
数结果条数并记下前 10 条结果的 provider / 番号 / 标题（全源聚合的响应也放得下），出错时留最多
2 KiB 用来取出 `error.message`，其他响应（尤其是图片）不占内存。

控制台把每条记录翻成一句人话，例如「搜索 ABC-123 · 6 条结果 · 4.55 s」、
「影片详情 javbus/ABC-123 · 1.4 KB」、「翻译 baidu · 52003: UNAUTHORIZED USER」，
路径里的百分号编码也会还原成原文。

`/v1/logs`（私有，需 token）支持增量拉取：

```text
GET  /v1/logs?since=<seq>&limit=200   # 只取比 seq 新的记录，控制台每 3 秒跟随一次
DELETE /v1/logs                        # 清空，seq 保持单调递增，不会让前端重复拉到旧记录
```

`/ui`、`/v1/status`、`/v1/logs`、`/favicon.ico` 这四个路径**不记录**，否则控制台自己的轮询会把
真正的业务请求淹掉。

配套的 `/v1/status` 是**私有**接口（`/v1/status`，需 token），返回控制台用的运行时信息：

```json
{"data":{"app":"metatube","version":"4.1","uptime_seconds":7,
         "db_driver":"sqlite","go_version":"go1.27.3","goos":"windows","goarch":"amd64",
         "num_cpu":16,"proxy":"http://127.0.0.1:7898 (via Windows Internet Settings; 24 bypass rule(s))",
         "proxy_active":true,"movie_providers":34,"actor_providers":3,"modules":97}}
```

### 批量翻译

控制台的自动翻译用它，一次搜索只产生一条请求：

```text
POST /v1/translate/batch?engine=BaiduAI&to=zh&from=auto
{"texts":["…","…"]}

{"data":{"items":[{"source":"…","translated_text":"…"},{"source":"…","error":"…"}]}}
```

一次最多 40 条、每条最多 2000 字符，重复文本自动去重（同一个演员会出现在多个结果里），
引擎凭据与单条接口一样从 query 参数传。

### 翻译引擎凭据

上面两个接口的凭据由调用方自己带：Metadata agent 就是这么做的（偏好设置里
`translation_engine_parameters` 写成 `baidu-app-id=…,baidu-app-key=…`，随请求进 query）。
控制台没有 agent 的上下文，所以它把凭据放在**服务端**的一份小文件里，开页面不用重填：

```json
{
  "engine": "baiduai",
  "baidu-app-id": "<APPID>",
  "baidu-app-key": "<密钥>",
  "baidu-model-type": "nmt"
}
```

- 位置：可执行文件同目录的 `translate.json`，用 `MT_TRANSLATE_CONFIG` 可覆盖路径
- 容器里不想挂 volume 时，把**整份 JSON 放进环境变量** `MT_TRANSLATE_JSON` 即可（见下方）
- 键名就是各引擎结构体的 json 标签（`deepl-api-key`、`google-api-key`、`openai-api-key`…），
  不认识的键自动忽略，换引擎不用改代码
- 改完**不用重启**：每次请求按 mtime 判断是否重读，写坏的文件保留上一份可用值
- 优先级：请求 query > `translate.json` > `MT_TRANSLATE_JSON` > 空；
  `MT_TRANSLATE_ENGINE` 能压过文件里的 `engine`
- 界面 PUT 仍然写文件；有文件时文件覆盖环境变量的同名键，容器重建后自动回落到环境变量基线
- 界面入口：右上角齿轮 →「翻译引擎」，选引擎、填字段、点「测试」当场验一次链路
- `GET/PUT /v1/translate/config` 是私有接口（要 token）；密钥只回打码值（`••••1dCJ`），
  输入框留空表示沿用已保存的值，PUT 只合并本次填的键，换引擎不会清掉另一个引擎的字段
- 防白用：只有**带着 token** 的请求（或服务端本来就没设 token）才会套用文件里的凭据；
  自己带凭据的调用不受影响。请求日志里 `baidu-app-id` / `baidu-app-key` 一律记成 `***`
- 上报的错误是引擎原文，例如 `54000: appid is empty`、`52003: UNAUTHORIZED USER`——
  `translate/baidu*` 里 `error_code` 同时接受字符串和数字（AIT 端点回的是数字）

容器里不想挂 volume 保存凭据时，把同一份 JSON 原样塞进环境变量（密钥写进 `.env` 并 gitignore）：

```yaml
environment:
  - MT_TRANSLATE_JSON=${MT_TRANSLATE_JSON:-}
```

```text
.env:
MT_TRANSLATE_JSON={"engine":"baiduai","baidu-app-id":"<APPID>","baidu-app-key":"<KEY>"}
```

容器重建后 `/translate.json` 消失也没关系，环境变量就是基线；不需要在 entrypoint 里生成文件。

### 自动翻译开关

搜索页的「自动翻译」勾上后，搜索完成会调一次批量翻译，把**标题**和**演员名**换成中文，并把
**发售日期**写成「2024年1月5日」这种长格式。译文缓存在浏览器 localStorage（最多 900 条），
同一段文本不会重复计费；关掉开关只是不再自动调用，已缓存的译文仍会显示。

没配凭据时勾开会直接提示去设置里填，翻译失败也会把引擎的原话弹出来，不会静默停留在日文原文。

判定是否要翻的依据是「含假名或拉丁字母，且不含中文」——日文标题里常有汉字，只按汉字判断会漏翻。

页面本身（`/ui`）无鉴权，因为它不含数据；所有数据请求走 `/v1/*`，设了 `--token` 时页面顶部
的 token 输入框要填对，否则连接失败。

## 与上游同步

```bash
git remote add upstream https://github.com/metatube-community/metatube-sdk-go.git
git fetch upstream --tags
git rebase upstream/main   # 或 merge
```

同步上游只涉及代码，版本号继续按本项目节奏推进（本地 `4.1.x` → 发布 `4.2`），不再跟随上游 tag 命名。
