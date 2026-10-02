.PHONY: run build test lint front-dev front-test front-build compose-up compose-down compose-logs db-shell backup restore secret-key

run:
	docker compose up postgres redis -d
	cd backend && set -a && . ../.env && set +a && go run ./cmd/main.go

build:
	cd backend && go build -o finassist ./cmd/main.go

test:
	cd backend && go test ./... -v

test-coverage:
	cd backend && go test ./... -coverprofile=coverage.out
	cd backend && go tool cover -html=coverage.out

lint:
	cd backend && golangci-lint run ./...

compose-up:
	docker compose up -d --build
	docker compose logs -f app

compose-down:
	docker compose down

compose-reset:
	docker compose down -v

compose-logs:
	docker compose logs -f

logs:
	docker compose logs -f app

logs-evolution:
	docker compose logs -f evolution

db-shell:
	docker compose exec postgres psql -U finassist -d finassist
# O front precisa de Node 22+ (com nvm: "nvm use 22"). Em desenvolvimento, o Vite roda em
# http://localhost:5173 e repassa /api ao app Go em 127.0.0.1:3000 (suba o app antes).
front-dev:
	cd frontend && npm install && npm run dev

front-test:
	cd frontend && npm install && npm test && npm run lint

front-build:
	cd frontend && npm install && npm run build

deps:
	cd backend && go mod tidy
	cd backend && go mod download

help:
	@echo ""
	@echo "Comandos disponíveis:"
	@echo "  make run              Sobe infra local + roda app Go direto"
	@echo "  make build            Compila o binário"
	@echo "  make test             Roda os testes"
	@echo "  make test-coverage    Testes com relatório de cobertura"
	@echo "  make lint             Verifica estilo e bugs"
	@echo "  make compose-up       Sobe tudo no Docker"
	@echo "  make compose-down     Derruba os containers"
	@echo "  make compose-reset    Derruba tudo + apaga volumes"
	@echo "  make compose-logs     Logs de todos os serviços"
	@echo "  make logs             Logs apenas da aplicação"
	@echo "  make logs-evolution   Logs apenas da Evolution API"
	@echo "  make db-shell         Abre o psql no container"
	@echo "  make deps             Baixa e organiza dependências"
	@echo "  make front-dev        Front em modo desenvolvimento (Vite)"
	@echo "  make front-test       Testes e lint do front"
	@echo "  make front-build      Build de produção do front"
	@echo ""
# Gera a chave mestra dos segredos em ./secrets/app_secret_key (600, fora do git). Depois, no .env:
# APP_SECRET_KEY_FILE=/run/secrets/app_secret_key  (e remova APP_SECRET_KEY). Não sobrescreve uma chave existente.
secret-key:
	@test ! -e secrets/app_secret_key || (echo "secrets/app_secret_key já existe; não vou sobrescrever (perderia os segredos salvos)" && exit 1)
	@mkdir -p secrets && umask 077 && openssl rand -base64 32 > secrets/app_secret_key && echo "criado secrets/app_secret_key. Defina no .env: APP_SECRET_KEY_FILE=/run/secrets/app_secret_key"

# Backup na hora, além do diário (arquivo 600 em ./backups, fora do git).
backup:
	@umask 077 && docker compose exec -T postgres pg_dump -U finassist -d finassist -Fc --no-owner > backups/finassist-$$(date -u +%Y%m%d-%H%M%S)-manual.dump && ls -l backups | tail -1

# Restaura um backup: make restore FILE=backups/finassist-AAAAMMDD-HHMMSS.dump  (SUBSTITUI os dados atuais)
restore:
	@test -n "$(FILE)" || (echo "uso: make restore FILE=backups/<arquivo>.dump" && exit 1)
	docker compose exec -T postgres pg_restore -U finassist -d finassist --clean --if-exists --no-owner < $(FILE)
