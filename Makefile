.PHONY: test backend-test frontend-test frontend-build build docker

backend-test:
	cd backend && go test ./...

frontend-test:
	cd frontend && npm test

frontend-build:
	cd frontend && npm ci --prefer-offline && npm run build

test: backend-test frontend-test

build: frontend-build
	cd backend && go build -o ../bin/zimaos-backup-manager ./cmd/server

docker:
	docker build -t zimaos-backup-manager:local .
