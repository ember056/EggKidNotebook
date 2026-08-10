# EggKidNotebook model services

This directory contains small OpenAI-compatible HTTP wrappers used by the
WeKnora backend:

- `POST /v1/embeddings` for `Qwen/Qwen3-Embedding-4B`
- `POST /v1/rerank` for `BAAI/bge-reranker-v2-m3`

On the remote server these services are intended to bind to the Docker network
gateway instead of the public interface, for example:

```bash
python deploy/model_services/model_api.py \
  --task embedding \
  --model Qwen/Qwen3-Embedding-4B \
  --host 172.18.0.1 \
  --port 18081

python deploy/model_services/model_api.py \
  --task rerank \
  --model BAAI/bge-reranker-v2-m3 \
  --host 172.18.0.1 \
  --port 18082
```

Then configure WeKnora built-in models with:

- `EMBEDDING_BASE_URL=http://172.18.0.1:18081/v1`
- `RERANK_BASE_URL=http://172.18.0.1:18082/v1`

Because these are private network targets, add `172.18.0.1` to
`SSRF_WHITELIST_EXTRA`.

For the remote server used by this project, after the weights are downloaded to
`/disk/user/lsb/models`, start both services with:

```bash
cd /disk/user/lsb/EggKidNotebook
bash deploy/model_services/start_remote_model_services.sh
```
