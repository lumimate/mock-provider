# mock-provider

A minimal OpenAI-compatible mock provider for latency testing across the gateway → bridge → new-api pipeline.

No authentication. Returns fixed mock responses instantly.

## Endpoints

| Endpoint | Method | Description |
|---|---|---|
| `/v1/chat/completions` | POST | Chat completion (streaming & non-streaming) |
| `/v1/embeddings` | POST | Embedding (returns 1536-dim vector) |
| `/v1/models` | GET | Model list |
| `/health` | GET | Health check |

## Run

```bash
go run . -port 8199
```

## Build & Run with Docker

```bash
docker build -t mock-provider .
docker run -p 8199:8199 mock-provider
```

## Configure in new-api

1. Add a new channel (type: OpenAI) with base URL `http://127.0.0.1:8199`
2. Set any API key (it won't be validated)
3. Add a model name like `mock-latency-test`
4. Send requests to the model through the gateway to measure end-to-end latency
