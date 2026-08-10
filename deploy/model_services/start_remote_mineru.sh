#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="${PROJECT_DIR:-/disk/user/lsb/EggKidNotebook}"
MINERU_API="${MINERU_API:-/disk/user/lsb/conda-envs/eggnb-mineru/bin/mineru-api}"
MINERU_HOST="${MINERU_HOST:-172.18.0.1}"
MINERU_PORT="${MINERU_PORT:-18080}"

cd "${PROJECT_DIR}"
mkdir -p logs /disk/user/lsb/huggingface /disk/user/lsb/modelscope_cache

if [[ -f logs/mineru-api.pid ]]; then
  old_pid="$(cat logs/mineru-api.pid 2>/dev/null || true)"
  if [[ -n "${old_pid}" ]] && kill -0 "${old_pid}" 2>/dev/null; then
    kill "${old_pid}" || true
  fi
fi

nohup env \
  PYTHONUNBUFFERED=1 \
  HF_ENDPOINT="${HF_ENDPOINT:-https://hf-mirror.com}" \
  HF_HOME="${HF_HOME:-/disk/user/lsb/huggingface}" \
  MODELSCOPE_CACHE="${MODELSCOPE_CACHE:-/disk/user/lsb/modelscope_cache}" \
  CUDA_VISIBLE_DEVICES="${MINERU_CUDA_VISIBLE_DEVICES:-2}" \
  "${MINERU_API}" --host "${MINERU_HOST}" --port "${MINERU_PORT}" \
  > logs/mineru-api.log 2>&1 &

echo $! > logs/mineru-api.pid
echo "mineru-api pid=$(cat logs/mineru-api.pid)"
