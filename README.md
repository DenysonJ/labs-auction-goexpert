# Auction (Leilão) — Fechamento automático com Goroutines

Sistema de leilões em Go onde cada leilão é **fechado automaticamente** depois de um
tempo configurável, sem intervenção manual. O agendamento do fechamento é disparado
no momento da criação do leilão, dentro de uma *goroutine* que não bloqueia a
requisição HTTP.

Desafio do curso **Go Expert** — Pós Full Cycle.

## Funcionalidades

- Criar leilão (`auction`) e registrar lances (`bid`).
- **Fechamento automático do leilão** após a duração definida em `AUCTION_DURATION`.
- Validação que impede lances em leilões já fechados.
- Consulta de leilões, lances e lance vencedor.

## Como o fechamento automático funciona

A implementação está em
[`internal/infra/database/auction/create_auction.go`](internal/infra/database/auction/create_auction.go):

1. Ao criar um leilão, além de inserir no MongoDB, dispara-se uma goroutine
   (`setAuctionClosed`) — por isso a thread principal/requisição **não é bloqueada**.
2. A goroutine aguarda, via `time.After`, a duração lida de `AUCTION_DURATION`.
3. Quando o tempo expira, faz o `UpdateOne` mudando o `status` do leilão para
   `Completed` (fechado).
4. A goroutine também observa o `context` da aplicação (`<-ctx.Done()`): no
   shutdown do servidor (`SIGINT`/`SIGTERM`) ela encerra de forma limpa, sem
   vazar goroutines.

O `context` de ciclo de vida da aplicação é criado no `main` com
`signal.NotifyContext` e injetado no repositório via construtor
(`NewAuctionRepository`), permitindo shutdown gracioso do servidor HTTP.

## Requisitos

- [Docker](https://docs.docker.com/get-docker/) e Docker Compose
- (Opcional, para rodar/testar fora do container) [Go 1.20+](https://go.dev/dl/)

## Variáveis de ambiente

Ficam em [`cmd/auction/.env`](cmd/auction/.env) (carregado via `godotenv` e também
pelo `docker-compose`):

| Variável                     | Descrição                                                                 | Exemplo                                                        |
| ---------------------------- | ------------------------------------------------------------------------- | ------------------------------------------------------------- |
| `AUCTION_DURATION`           | **Duração do leilão, em segundos.** Após esse tempo o leilão é fechado.    | `60`                                                          |
| `AUCTION_INTERVAL`           | Janela usada na validação de lances (não aceita lance após o fim).        | `20s`                                                        |
| `BATCH_INSERT_INTERVAL`      | Intervalo de flush do batch de lances.                                    | `20s`                                                        |
| `MAX_BATCH_SIZE`             | Tamanho máximo do batch de lances.                                        | `4`                                                          |
| `MONGO_INITDB_ROOT_USERNAME` | Usuário root do MongoDB.                                                   | `admin`                                                      |
| `MONGO_INITDB_ROOT_PASSWORD` | Senha root do MongoDB.                                                     | `admin`                                                      |
| `MONGODB_URL`                | URL de conexão com o MongoDB.                                             | `mongodb://admin:admin@mongodb:27017/auctions?authSource=admin` |
| `MONGODB_DB`                 | Nome do banco.                                                             | `auctions`                                                   |

> **`AUCTION_DURATION`** é o parâmetro do desafio. É lido em **segundos**
> (`strconv.Atoi`). Para ver o fechamento rápido durante os testes manuais,
> reduza para, por exemplo, `AUCTION_DURATION=20`.
>
> Dentro do Docker, o host do Mongo é `mongodb` (nome do serviço). Para rodar a
> aplicação ou os testes **fora** do container, use `localhost`:
> `mongodb://admin:admin@localhost:27017/auctions?authSource=admin`.

## Como rodar (Docker / Docker Compose)

Na raiz do projeto:

```bash
docker compose up -d --build
```

Isso sobe dois serviços:

- `app` — a API em `http://localhost:8080`
- `mongodb` — MongoDB em `localhost:27017`

Para acompanhar os logs e parar:

```bash
docker compose logs -f app
docker compose down
```

## Testando o fechamento automático

### Teste automatizado (cenário do desafio)

O teste em
[`internal/infra/database/auction/create_auction_test.go`](internal/infra/database/auction/create_auction_test.go)
comprova o cenário:

| Passo | Ação                                  | Resultado esperado                       |
| ----- | ------------------------------------- | ---------------------------------------- |
| 1     | Criar um leilão                       | Leilão criado com status **Active**      |
| 2     | Aguardar o tempo de `AUCTION_DURATION`| —                                        |
| 3     | Verificar o status do leilão          | Alterado automaticamente para **Completed** |

O teste usa um `AUCTION_DURATION` curto e um banco isolado (`auctions_test`),
que é removido ao final. Ele sobe contra um MongoDB real.

**Com o Mongo rodando** (via Docker):

```bash
docker compose up -d mongodb
go test ./internal/infra/database/auction/... -run TestCreateAuction_AutomaticClose -v
```

Saída esperada:

```
--- PASS: TestCreateAuction_AutomaticClose (1.25s)
PASS
```

> Se o MongoDB não estiver acessível, o teste faz **skip** (não falha), com uma
> mensagem indicando como subir o banco.

Para rodar toda a suíte:

```bash
go test ./...
```

### Teste manual (via API)

Com `AUCTION_DURATION` baixo (ex.: `20`) e o ambiente no ar:

```bash
# 1. Criar um leilão
curl -X POST http://localhost:8080/auction \
  -H "Content-Type: application/json" \
  -d '{"product_name":"Playstation 5","category":"Games","description":"A brand new console","condition":1}'

# 2. Consultar logo em seguida (status 0 = Active)
curl http://localhost:8080/auction/<AUCTION_ID>

# 3. Aguardar mais que AUCTION_DURATION e consultar de novo (status 1 = Completed)
curl http://localhost:8080/auction/<AUCTION_ID>
```

`status`: `0 = Active (aberto)`, `1 = Completed (fechado)`.

## Endpoints

| Método | Rota                          | Descrição                     |
| ------ | ----------------------------- | ----------------------------- |
| POST   | `/auction`                    | Cria um leilão                |
| GET    | `/auction`                    | Lista leilões                 |
| GET    | `/auction/:auctionId`         | Busca leilão por id           |
| GET    | `/auction/winner/:auctionId`  | Lance vencedor do leilão      |
| POST   | `/bid`                        | Registra lance(s)             |
| GET    | `/bid/:auctionId`             | Lista lances de um leilão     |
| GET    | `/user/:userId`               | Busca usuário por id          |

## Estrutura

```
cmd/auction/              # entrypoint (main), .env e shutdown gracioso
internal/
  entity/                 # entidades de domínio (auction, bid, user)
  usecase/                # casos de uso
  infra/
    database/             # repositórios MongoDB (fechamento automático aqui)
    api/web/controller/   # handlers HTTP (gin)
configuration/            # conexão com Mongo, logger
```
