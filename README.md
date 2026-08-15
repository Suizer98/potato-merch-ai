# protobuf-ai-potato

Go gRPC AI chat backend with protobuf streaming. Docker-first so you can scale replicas later.

Live demo (grpcui): https://protobuf-ai-potato.onrender.com

## Layout

```
api/proto/chat/v1/chat.proto   # ChatService contract
cmd/server                     # gRPC server entrypoint
internal/llm                   # mock + openai-compatible (openai, groq, gemini) providers
internal/session               # in-memory session history
internal/server                # Chat RPC handlers
crm/                           # Twenty CRM seed (compose lives at repo root)
store/                         # Potato Merch storefront (Vite + React)
Dockerfile                     # buf generate + static binary (local chat)
Dockerfile.grpcui              # browser gRPC UI only
Dockerfile.render              # Render all-in-one: chat + grpcui
docker-compose.yml             # chat + grpcui + CRM + store
scripts/render-entrypoint.sh   # starts gRPC then grpcui
```

## Quick start

### Docker (everything)

```bash
cp .env.example .env
docker compose up --build
```

- gRPC server: `:50051`
- Browser UI (grpcui): [http://localhost:8080](http://localhost:8080)
- CRM (Twenty): [http://localhost:3000](http://localhost:3000)
- Merch store: [http://localhost:3001](http://localhost:3001)

Default chat provider is `mock` unless you set `LLM_PROVIDER` in `.env`.

### Groq

```bash
# .env
LLM_PROVIDER=groq
GROQ_API_KEY=gsk_...
GROQ_MODEL=openai/gpt-oss-120b
docker compose up --build
```

### Gemini

```bash
# .env
LLM_PROVIDER=gemini
GEMINI_API_KEY=AIza...
GEMINI_MODEL=gemini-3.5-flash
docker compose up --build
```

### OpenAI

```bash
# .env
LLM_PROVIDER=openai
OPENAI_API_KEY=sk-...
docker compose up --build
```

### Local without Docker

```bash
# needs: go 1.22+, buf
make run
```

## gRPC surface

- `Chat(ChatRequest) returns (stream ChatChunk)` — stream token deltas
- `ListSessions` — list in-memory sessions
- standard gRPC health + reflection enabled

## Scaling path

1. Chat containers are mostly stateless: provider calls + in-memory history per process.
2. Redis is already running for CRM; next step is wire `internal/session` to that Redis so any chat replica can continue a session.
3. `docker compose up --build --scale chat=2` scales chat replicas (history sharing not wired yet).
4. Put a load balancer / ingress in front of `:50051` (or use Kubernetes Deployment + Service).

## Useful commands

```bash
make docker-up
make docker-down
make docker-scale
make generate
```

## Deploy on Render (UI in front)

Use `Dockerfile.render` so the public URL is **grpcui** (HTTP). gRPC stays inside the container.

1. Render → Web Service → Docker
2. Dockerfile path: `Dockerfile.render`
3. Env vars:

```text
LLM_PROVIDER=groq
GROQ_API_KEY=gsk_...
GROQ_MODEL=openai/gpt-oss-120b
```

Leave `PORT` alone (Render sets it). Do **not** point `GRPC_ADDR` at Render's public port — the entrypoint keeps gRPC on `127.0.0.1:50051`.

4. After deploy, open: `https://<your-service>.onrender.com`

Browser → grpcui (HTTP) → localhost gRPC chat inside the same container.
