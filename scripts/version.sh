#!/usr/bin/env bash
# metatube-server-ltx 版本号工具
#
# 版本体系（与上游彻底解耦）：
#   - 版本号形如 MAJOR.MINOR.PATCH，MAJOR 固定 4（承接上游 v1.4.0 的基线）。
#   - 本地开发：每改一轮 PATCH 加一 → 4.0.0 → 4.0.1 → 4.0.2 …
#   - 发布：MINOR 加一、PATCH 归零 → 4.1，打 tag v4.1（镜像 tag 不带 v）。
#   - 发布后本地基线变成 4.1.0，继续 4.1.1 / 4.1.2 …，下次发布即 4.2。
#
# 用法：
#   scripts/version.sh show                   # 当前版本 + 下个开发/发布版本
#   scripts/version.sh bump [--to 4.0.3]      # 本地改动后升 PATCH（写 VERSION，不提交）
#   scripts/version.sh release [--dry-run]    # 升到下一个发布版本（MINOR+1），提交 VERSION
#   scripts/version.sh release --tag          # 再在本地打 tag（不推送）
#   scripts/version.sh release --tag --push   # 提交 + 打 tag + 推送（需用户明确同意）
#   scripts/version.sh verify v4.1            # 校验 tag / VERSION / notes 是否自洽（CI 用）
#
# 铁律：默认绝不推送。--push 只在用户明确要求「推送 / 发版」那一轮才用。
set -euo pipefail

repo="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
version_file="$repo/VERSION"
remote="${REMOTE:-mine}"

die() { echo "[version] $*" >&2; exit 1; }
info() { echo "[version] $*"; }

read_version() {
  [ -f "$version_file" ] || die "缺少 $version_file"
  tr -d ' \r\n' < "$version_file"
}

check_semver() {
  [[ "$1" =~ ^[0-9]+\.[0-9]+\.[0-9]+$ ]] || die "版本号必须形如 MAJOR.MINOR.PATCH：$1"
}

# 下一个发布版本：MINOR+1，PATCH 归零（4.0.2 → 4.1.0；4.1.3 → 4.2.0）
next_release() {
  local major minor patch
  IFS=. read -r major minor patch <<<"$1"
  echo "${major}.$((minor + 1)).0"
}

# 下一个本地开发版本：PATCH+1
next_dev() {
  local major minor patch
  IFS=. read -r major minor patch <<<"$1"
  echo "${major}.${minor}.$((patch + 1))"
}

# tag 形如 v4.1（两点式，不带 PATCH）
tag_of() {
  local major minor patch
  IFS=. read -r major minor patch <<<"$1"
  echo "v${major}.${minor}"
}

cmd_show() {
  local cur; cur="$(read_version)"
  check_semver "$cur"
  local rel; rel="$(next_release "$cur")"
  info "当前版本: $cur"
  info "本地下一改动: $(next_dev "$cur")"
  info "下次发布版本: ${rel%.*}  (VERSION 记为 $rel，tag $(tag_of "$rel"))"
}

cmd_bump() {
  local to=""
  while [ $# -gt 0 ]; do
    case "$1" in
      --to) to="${2:-}"; [ -n "$to" ] || die "--to 需要版本号"; shift 2 ;;
      *) die "未知参数: $1" ;;
    esac
  done
  local cur; cur="$(read_version)"
  check_semver "$cur"
  [ -n "$to" ] || to="$(next_dev "$cur")"
  check_semver "$to"
  printf '%s\n' "$to" > "$version_file"
  info "版本号: $cur → $to（只改文件，未提交）"
}

cmd_release() {
  local dry_run=0 do_tag=0 do_push=0
  while [ $# -gt 0 ]; do
    case "$1" in
      --dry-run) dry_run=1; shift ;;
      --tag) do_tag=1; shift ;;
      --push) do_tag=1; do_push=1; shift ;;
      *) die "未知参数: $1" ;;
    esac
  done

  local cur; cur="$(read_version)"
  check_semver "$cur"
  local rel; rel="$(next_release "$cur")"
  local tag; tag="$(tag_of "$rel")"
  local note="$repo/notes/${rel%.*}.md"

  # 本轮发布说明必须存在：只写本轮改动，不允许把历史内容带进来。
  if [ ! -f "$note" ]; then
    [ "$dry_run" = 1 ] && { info "缺少 $note（dry-run 不创建）"; } || {
      mkdir -p "$repo/notes"
      cat > "$note" <<EOF
<!-- 只写本轮（${tag}）的改动，一条一行，格式：- **标题**：一句话说明。
     历史版本内容不要复制进来，正文不要加标题（CI 会自动补）。写完后重跑本命令。 -->
- 
EOF
      die "已生成模板 $note，填写本轮改动后重跑"
    }
  fi

  info "发布版本: $cur → $rel  (tag $tag)"
  if [ "$dry_run" = 1 ]; then
    info "dry-run：不做任何写入"
    return 0
  fi

  printf '%s\n' "$rel" > "$version_file"
  git -C "$repo" add VERSION "notes/${rel%.*}.md"
  git -C "$repo" commit -q -m "chore(release): ${tag}

VERSION 升到 ${rel}（发布版本 MINOR 递增、PATCH 归零），并附本轮更新说明 notes/${rel%.*}.md。"

  if [ "$do_tag" = 1 ]; then
    git -C "$repo" tag -a "$tag" -m "metatube-server-ltx ${tag}"
    info "已创建本地 tag $tag"
  fi

  if [ "$do_push" = 1 ]; then
    git -C "$repo" push "$remote" HEAD
    git -C "$repo" push "$remote" "$tag"
    info "已推送 main 与 $tag"
  else
    cat <<EOF

[version] 本地已完成，未推送。确认后执行：
  git -C "$repo" push $remote HEAD
  git -C "$repo" push $remote $tag
（或重跑 scripts/version.sh release --tag --push）
EOF
  fi
}

cmd_verify() {
  local tag="${1:-}"
  [ -n "$tag" ] || die "用法: version.sh verify v4.1"
  [[ "$tag" =~ ^v([0-9]+)\.([0-9]+)$ ]] || die "tag 必须形如 v4.1：$tag"
  local ver="${tag#v}"
  local cur; cur="$(read_version)"
  check_semver "$cur"
  case "$cur" in
    "$ver".*) ;;
    *) die "VERSION=$cur 与 tag $tag 不匹配（应为 ${ver}.0）" ;;
  esac
  [ -f "$repo/notes/${ver}.md" ] || die "缺少本轮说明 notes/${ver}.md"
  info "OK: tag=$tag VERSION=$cur notes/${ver}.md"
}

case "${1:-}" in
  show) shift; cmd_show "$@" ;;
  bump) shift; cmd_bump "$@" ;;
  release) shift; cmd_release "$@" ;;
  verify) shift; cmd_verify "$@" ;;
  *) sed -n '2,26p' "${BASH_SOURCE[0]}" | sed 's/^# \{0,1\}//'; exit 1 ;;
esac
