# Contribuindo

## Rodar tudo localmente

```bash
make test           # backend (go test ./...)
make lint           # golangci-lint (go install github.com/golangci/golangci-lint/v2/cmd/golangci-lint@latest)
make front-test     # vitest
cd frontend && npx tsc -b && npx oxlint
```

Testes de integração do banco só rodam com `TEST_DATABASE_URL` apontando para um Postgres **descartável** com as migrations de `backend/migrations/` aplicadas (nunca o banco de uso):

```bash
docker compose exec -T postgres psql -U finassist -d postgres -c "CREATE DATABASE finassist_scratch"
for f in backend/migrations/*.sql; do docker compose exec -T postgres psql -q -U finassist -d finassist_scratch < $f; done
cd backend && TEST_DATABASE_URL=postgres://finassist:finassist@localhost:5432/finassist_scratch?sslmode=disable go test ./...
docker compose exec -T postgres psql -U finassist -d postgres -c "DROP DATABASE finassist_scratch"
```

## Regras

- **Commits pequenos**, um por passo lógico, com mensagem no formato `tipo: Descrição` (`feat`, `fix`, `refactor`, `docs`, `test`). Em refatoração, **mover e alterar não andam juntos**: primeiro o commit que só move, depois o que muda.
- **Tudo verde antes de abrir PR:** `gofmt -l .` sem saída, `go vet`, `make lint`, `go test ./...`, `tsc`, `oxlint`, `vitest`. O CI roda o mesmo.
- **Idioma:** identificadores em inglês; comentários, mensagens de erro, logs e textos da interface em português. Mensagens de erro em minúsculas.
- **Comentários** explicam o *porquê* em poucas linhas. Algoritmos e decisões longas vão para `docs/` (regras de cálculo, decisões), não para o código.
- **Migrations** novas: um arquivo numerado em `backend/migrations/`, idempotente (`IF NOT EXISTS`), com o comando de aplicar em bancos existentes no CHANGELOG e no README.
- **Segredos** nunca em log, resposta de API ou commit. Veja a seção de Configurações no README.
- **Sem abstração especulativa:** só extraia interface, tipo ou pacote com pelo menos dois usos reais.
