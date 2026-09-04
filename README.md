# Distributed Job Queue Service

A deployable PostgreSQL-backed job queue with a Go API, independently scalable
workers, an operations dashboard, and dependency auditing as a reference
workload.

```text
Browser -> Dashboard -> API -> PostgreSQL <- Worker 1..N
```

## Start the complete application

Docker Compose is the supported delivery path:

```bash
cp .env.example .env
docker compose up --build
```

Open the dashboard at [http://localhost:3000](http://localhost:3000). The API is
available at `http://localhost:8080`, including `/health`, `/ready`, and
`/metrics`.

Compose starts PostgreSQL, applies every migration, starts the API, starts two
workers, and serves the dashboard. Job state remains in the `pg_data` volume
across restarts. Use `docker compose down -v` only when you intentionally want
to delete that local history.

## Run components locally

Start only the database and migrations:

```bash
docker compose up -d postgres migrate
export DATABASE_URL='postgres://postgres:postgres@localhost:5432/jobqueue?sslmode=disable'
```

Then use separate terminals:

```bash
go run ./cmd/api
go run ./cmd/worker
go run ./cmd/worker
```

Run the dashboard in development mode:

```bash
cd web
npm install
npm run dev
```

## Submit work

```bash
curl -i -X POST http://localhost:8080/api/jobs \
  -H 'Content-Type: application/json' \
  -H 'Idempotency-Key: demo-1' \
  -d '{"type":"demo","payload":{"durationMs":1000},"maxAttempts":5}'
```

The API returns `202 Accepted` immediately. Supported public job types are:

- `demo`: controlled success, delay, transient failure, or permanent failure
- `dependency_audit`: scans supported manifests in a public GitHub repository

## Verification

```bash
go test -race ./...
DATABASE_URL="$DATABASE_URL" go test -race -tags=integration ./internal/store/postgres

cd web
npm run typecheck
npm run lint
npm test
npm run build
```

See [architecture](docs/architecture.md), [reliability guarantees](docs/reliability.md),
[test strategy](docs/testing.md), and the [demo runbook](docs/demo.md).
