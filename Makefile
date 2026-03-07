.PHONY: dev migrate generate test

dev:
	docker-compose up -d

migrate:
	go run github.com/pressly/goose/v3/cmd/goose -dir kors-api/migrations postgres "postgres://kors:kors_dev_secret@localhost:5432/kors?sslmode=disable" up

generate:
	cd kors-api && go run github.com/99designs/gqlgen generate

test:
	go test ./...

test-graphql:
	powershell -NoProfile -Command "Invoke-RestMethod -Uri 'http://localhost/query' -Method Post -ContentType 'application/json' -Body '{\"query\": \"mutation { registerResourceType(input: { name: \\\"tool_final\\\", description: \\\"Final Tool\\\", jsonSchema: {\\\"type\\\": \\\"object\\\"}, transitions: {\\\"idle\\\": [\\\"in_use\\\"]} }) { success } }\"}'"

