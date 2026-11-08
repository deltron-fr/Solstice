.PHONY: backend-run backend-test backend-build sqlc migrate-up frontend-dev frontend-check frontend-build onchain-build onchain-test test

backend-run:
	cd backend && go run -buildvcs=false ./cmd/api

backend-test:
	cd backend && go test ./...

backend-build:
	cd backend && mkdir -p bin && go build -buildvcs=false -o bin/solstice-api ./cmd/api

sqlc:
	cd backend && sqlc generate


migrate-up:
	cd backend && goose -dir migrations sqlite3 "$${SOLSTICE_DATABASE_DSN:-file:solstice.db}" up

frontend-dev:
	cd frontend && npm run dev

frontend-check:
	cd frontend && npm run check

frontend-build:
	cd frontend && npm run build

onchain-build:
	cd onchain && anchor build

onchain-test:
	cd onchain && anchor test

test: backend-test frontend-check
