.PHONY: tidy test api worker compose-up

tidy:
	go mod tidy

test:
	go test ./internal/engine ./internal/xerr ./internal/tokenkit ./internal/lottery -count=1

api:
	go run ./app/gateway -f app/gateway/etc/luckygo-api.yaml

worker:
	go run ./app/worker -f app/worker/etc/worker.yaml

compose-up:
	docker compose -f deploy/docker-compose.yml up -d mysql redis
