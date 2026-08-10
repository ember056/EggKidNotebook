#!/usr/bin/env bash
set -euo pipefail

PROJECT_DIR="${PROJECT_DIR:-/disk/user/lsb/EggKidNotebook}"
PYTHON="${PYTHON:-/disk/user/lsb/conda-envs/eggnb-models/bin/python}"
MODEL_HOST="${MODEL_HOST:-172.18.0.1}"
EMBEDDING_MODEL="${EMBEDDING_MODEL:-/disk/user/lsb/models/Qwen3-Embedding-4B}"
RERANK_MODEL="${RERANK_MODEL:-/disk/user/lsb/models/bge-reranker-v2-m3}"

cd "${PROJECT_DIR}"
mkdir -p logs /disk/user/lsb/huggingface

stop_from_pidfile() {
  local pidfile="$1"
  if [[ -f "${pidfile}" ]]; then
    local pid
    pid="$(cat "${pidfile}" 2>/dev/null || true)"
    if [[ -n "${pid}" ]] && kill -0 "${pid}" 2>/dev/null; then
      kill "${pid}" || true
    fi
  fi
}

stop_from_pidfile logs/embedding-qwen3-4b.pid
stop_from_pidfile logs/rerank-bge-v2-m3.pid
sleep 2

COMMON_ENV=(
  PYTHONUNBUFFERED=1
  HF_HOME=/disk/user/lsb/huggingface
  HF_ENDPOINT=https://hf-mirror.com
  HF_HUB_DISABLE_XET=1
  HF_HUB_ENABLE_HF_TRANSFER=0
  MODEL_HALF=true
)

nohup env "${COMMON_ENV[@]}" \
  CUDA_VISIBLE_DEVICES="${EMBEDDING_CUDA_VISIBLE_DEVICES:-0}" \
  MODEL_DEVICE="${EMBEDDING_MODEL_DEVICE:-cuda:0}" \
  EMBEDDING_BATCH_SIZE="${EMBEDDING_BATCH_SIZE:-4}" \
  EMBEDDING_NORMALIZE="${EMBEDDING_NORMALIZE:-true}" \
  "${PYTHON}" -u deploy/model_services/model_api.py \
  --task embedding --model "${EMBEDDING_MODEL}" --host "${MODEL_HOST}" --port "${EMBEDDING_PORT:-18081}" \
  > logs/embedding-qwen3-4b.log 2>&1 &
echo $! > logs/embedding-qwen3-4b.pid

nohup env "${COMMON_ENV[@]}" \
  CUDA_VISIBLE_DEVICES="${RERANK_CUDA_VISIBLE_DEVICES:-1}" \
  MODEL_DEVICE="${RERANK_MODEL_DEVICE:-cuda:0}" \
  RERANK_BATCH_SIZE="${RERANK_BATCH_SIZE:-8}" \
  RERANK_NORMALIZE="${RERANK_NORMALIZE:-true}" \
  "${PYTHON}" -u deploy/model_services/model_api.py \
  --task rerank --model "${RERANK_MODEL}" --host "${MODEL_HOST}" --port "${RERANK_PORT:-18082}" \
  > logs/rerank-bge-v2-m3.log 2>&1 &
echo $! > logs/rerank-bge-v2-m3.pid

echo "embedding pid=$(cat logs/embedding-qwen3-4b.pid)"
echo "rerank pid=$(cat logs/rerank-bge-v2-m3.pid)"
