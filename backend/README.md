# Pact backend

The backend is a Go `net/http` service with explicit configuration, JSON logging, SQLite persistence, goose migrations, and sqlc-generated query code. Its structure keeps startup wiring in `cmd/api` and implementation in `internal/`.

```sh
go mod download
sqlc generate
go test ./...
go run -buildvcs=false ./cmd/api
```

The application embeds and applies goose migrations at startup. SQLite uses `github.com/ncruces/go-sqlite3/driver`; it is pure Go and does not use CGO or `modernc.org/libc` as the application's driver. Goose migrations define runtime state; keep `sql/schema.sql` synchronized as the sqlc schema snapshot. `price` and `total_amount` are integer currency minor units (for example, kobo for NGN), never floating point.

Implemented HTTP operations are health, user create/read, and listing create/list/read. The remaining registered order and listing-update routes return a consistent `501` JSON response until their workflows are built. The blockchain package defines the future RPC verification boundary and explicitly returns not implemented for chain reads.
