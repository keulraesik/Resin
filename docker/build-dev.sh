#!/bin/sh
set -eu

script_dir="$(CDPATH= cd "$(dirname "$0")" && pwd)"
repo_root="$(CDPATH= cd "${script_dir}/.." && pwd)"

IMAGE_NAME="${IMAGE_NAME:-resin}"
DOCKERFILE_PATH="${DOCKERFILE_PATH:-docker/Dockerfile}"
CONTEXT_DIR="${CONTEXT_DIR:-${repo_root}}"

NPM_REGISTRY="${NPM_REGISTRY:-https://registry.npmmirror.com}"
GOPROXY="${GOPROXY:-https://goproxy.cn,direct}"
ALPINE_MIRROR="${ALPINE_MIRROR:-https://mirrors.aliyun.com/alpine}"

usage() {
  cat <<EOF
Usage:
  sh docker/build-dev.sh [options]

Options:
  -f, --file PATH       Dockerfile path. Default: docker/Dockerfile
  -c, --context PATH    Docker build context. Default: repository root
  -i, --image NAME      Image name. Default: resin
  -h, --help            Show this help

Environment:
  IMAGE_NAME            Image name, same as --image
  DOCKERFILE_PATH       Dockerfile path, same as --file
  CONTEXT_DIR           Build context, same as --context
  NPM_REGISTRY          Default: https://registry.npmmirror.com
  GOPROXY               Default: https://goproxy.cn,direct
  ALPINE_MIRROR         Default: https://mirrors.aliyun.com/alpine
EOF
}

while [ "$#" -gt 0 ]; do
  case "$1" in
    -f|--file)
      [ "$#" -ge 2 ] || { echo "Missing value for $1" >&2; exit 1; }
      DOCKERFILE_PATH="$2"
      shift 2
      ;;
    -c|--context)
      [ "$#" -ge 2 ] || { echo "Missing value for $1" >&2; exit 1; }
      CONTEXT_DIR="$2"
      shift 2
      ;;
    -i|--image)
      [ "$#" -ge 2 ] || { echo "Missing value for $1" >&2; exit 1; }
      IMAGE_NAME="$2"
      shift 2
      ;;
    -h|--help)
      usage
      exit 0
      ;;
    *)
      echo "Unknown option: $1" >&2
      usage >&2
      exit 1
      ;;
  esac
done

case "${DOCKERFILE_PATH}" in
  /*) dockerfile_abs="${DOCKERFILE_PATH}" ;;
  *) dockerfile_abs="${repo_root}/${DOCKERFILE_PATH}" ;;
esac

case "${CONTEXT_DIR}" in
  /*) context_abs="${CONTEXT_DIR}" ;;
  *) context_abs="${repo_root}/${CONTEXT_DIR}" ;;
esac

if [ ! -f "${dockerfile_abs}" ]; then
  echo "Dockerfile not found: ${dockerfile_abs}" >&2
  exit 1
fi

if [ ! -d "${context_abs}" ]; then
  echo "Docker build context not found: ${context_abs}" >&2
  exit 1
fi

command -v docker >/dev/null 2>&1 || { echo "docker is required" >&2; exit 1; }
command -v git >/dev/null 2>&1 || { echo "git is required" >&2; exit 1; }

commit="$(git -C "${repo_root}" rev-parse --short=12 HEAD)"
dirty_status="$(
  git -C "${repo_root}" status --porcelain --untracked-files=all \
    | sed '/^?? \.dev\//d' \
    | sed '/^?? data\//d'
)"
dirty=""
if [ -n "${dirty_status}" ]; then
  dirty="-dirty"
fi

tag="dev-${commit}${dirty}"
build_time="$(date -u +%Y-%m-%dT%H:%M:%SZ)"

echo "Building Docker image"
echo "  image:      ${IMAGE_NAME}"
echo "  tag:        ${tag}"
echo "  dockerfile: ${dockerfile_abs}"
echo "  context:    ${context_abs}"
echo "  goproxy:    ${GOPROXY}"
echo "  npm:        ${NPM_REGISTRY}"
echo "  alpine:     ${ALPINE_MIRROR}"

DOCKER_BUILDKIT="${DOCKER_BUILDKIT:-1}" docker build \
  -f "${dockerfile_abs}" \
  -t "${IMAGE_NAME}:${tag}" \
  -t "${IMAGE_NAME}:dev" \
  --build-arg VERSION="${tag}" \
  --build-arg GIT_COMMIT="${commit}" \
  --build-arg BUILD_TIME="${build_time}" \
  --build-arg NPM_REGISTRY="${NPM_REGISTRY}" \
  --build-arg GOPROXY="${GOPROXY}" \
  --build-arg ALPINE_MIRROR="${ALPINE_MIRROR}" \
  "${context_abs}"

echo
echo "Built:"
echo "  ${IMAGE_NAME}:${tag}"
echo "  ${IMAGE_NAME}:dev"
