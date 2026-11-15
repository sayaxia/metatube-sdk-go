# MetaTube SDK Go

[![Build Status](https://img.shields.io/github/actions/workflow/status/sayaxia/metatube-sdk-go/release.yml?branch=main&style=flat-square&logo=github-actions&label=release)](https://github.com/sayaxia/metatube-sdk-go/actions/workflows/release.yml)
[![Go Linter](https://img.shields.io/github/actions/workflow/status/metatube-community/metatube-sdk-go/linter.yml?style=flat-square&branch=main&logo=githubactions&label=golangci-lint)](https://github.com/metatube-community/metatube-sdk-go/actions/workflows/linter.yml)
[![Require Go Version](https://img.shields.io/badge/go-%3E%3D1.25-30dff3?style=flat-square&logo=go)](https://github.com/metatube-community/metatube-sdk-go/blob/main/go.mod)
[![GitHub License](https://img.shields.io/github/license/metatube-community/metatube-sdk-go?color=e4682a&logo=apache&style=flat-square)](https://github.com/metatube-community/metatube-sdk-go/blob/main/LICENSE)
[![Tag](https://img.shields.io/github/v/tag/metatube-community/metatube-sdk-go?color=%23ff8936&logo=fitbit&style=flat-square)](https://github.com/metatube-community/metatube-sdk-go/tags)

**English** | [简体中文](README.zh-CN.md)

Metadata Tube SDK in Golang.

> **This is a personal fork (`v1.4.0-ltx.N`)** of the upstream project, maintained at
> [sayaxia/metatube-sdk-go](https://github.com/sayaxia/metatube-sdk-go). It adds support for
> AI-restored releases whose file names carry a restoration suffix (`-LTX`, `-BasicVSR++`,
> `-restored` / `__restored`), e.g. `ABP-030-LTX` or `ABP-030__restored`. All of these
> suffixes denote the same uncensored release and are trimmed back to the bare number
> before any provider lookup. Stacked tool marks are handled too, including a watermark
> tag and dot separators (`JUR-110_restored_wm`, `SONE-521_restored.LTX_wm`).
> See [FORK.md](FORK.md) for the full delta, versioning scheme and Docker instructions.
> Upstream remains the source of truth for everything else.

## Fork highlights (since v1.4.0-ltx.8)

- **Built-in console at `/ui`** — recent searches grouped by title with friendly detail
  popups, per-request logs with secrets redacted, one-click clear, and an auto-translate
  toggle. Headless-friendly: open it in a browser against any server instance.
- **Windows desktop panel (`metatube-panel.exe`)** — a WebView2 window with a system tray
  icon that starts, stops and restarts the bundled service; closing or minimizing goes to
  the tray, and "Quit" shuts down both. The official logo is embedded as the app icon.
- **Outbound proxy** — follows the OS proxy by default (registry on Windows, env vars on
  Linux/Docker), overridable with `MT_PROXY_URL`; also fixes colly-based providers that
  previously bypassed the proxy.
- **Translation** — adds the Baidu LLM engine `baiduai`; credentials live next to the
  executable in `translate.json` or in `MT_TRANSLATE_JSON` (no Docker volume needed),
  hot-reloaded on change. A batch endpoint and UI toggle translate titles/cast/release date.
- **Downloads** — every tagged release ships Linux amd64/arm64 Docker images plus the two
  Windows executables. See [FORK.md](FORK.md) for details.

## Contents

- [MetaTube SDK Go](#metatube-sdk-go)
    - [Fork highlights](#fork-highlights-since-v140-ltx8)
    - [Contents](#contents)
    - [Features](#features)
    - [Installation](#installation)
    - [Credits](#credits)
    - [License](#license)

## Features

- Supported platforms
    - Linux
    - Darwin
    - Windows
    - BSD(s)
- Supported Databases
    - [SQLite](https://gitlab.com/cznic/sqlite)
    - [PostgreSQL](https://github.com/jackc/pgx)
- Image processing
    - Auto cropping
    - Badge support
    - Face detection
    - Image hashing
- RESTful API
- 20+ providers
- Text translation

## Installation

To install this package, you first need [Go](https://golang.org/) installed (**go1.25+ is required**), then you can use
the below Go command to install SDK.

```sh
go get -u github.com/metatube-community/metatube-sdk-go
```

## Credits

| Library														                                           | Description																						                                                                    |
|-----------------------------------------------------------------|------------------------------------------------------------------------------------------------------|
| [gocolly/colly](https://github.com/gocolly/colly)			            | Elegant Scraper and Crawler Framework for Golang													                                        |
| [gin-gonic/gin](https://github.com/gin-gonic/gin)			            | Gin is a HTTP web framework written in Go															                                             |
| [gorm.io/gorm](https://gorm.io/)								                        | The fantastic ORM library for Golang																                                                 |
| [esimov/pigo](https://github.com/esimov/pigo)				               | Fast face detection, pupil/eyes localization and facial landmark points detection library in pure Go |
| [robertkrimen/otto](https://github.com/robertkrimen/otto)       | A JavaScript interpreter in Go (golang)                                                              |
| [modernc.org/sqlite](https://gitlab.com/cznic/sqlite)		         | Package sqlite is a CGo-free port of SQLite/SQLite3												                                      |
| [corona10/goimagehash](https://github.com/corona10/goimagehash) | Go Perceptual image hashing package																                                                  |
| [antchfx/xpath](https://github.com/antchfx/xpath)			            | XPath package for Golang, supports HTML, XML, JSON document query									                           |
| [gen2brain/jpegli](https://github.com/gen2brain/jpegli)         | Go encoder/decoder for JPEG based on jpegli                                                          |

## License

[Apache-2.0 License](https://github.com/metatube-community/metatube-sdk-go/blob/main/LICENSE)
