# 🚚 GoFleet

> **Sistema Distribuído de Logística e Despacho em Tempo Real**

![Go](https://img.shields.io/badge/go-%3E%3D1.22-00ADD8?style=flat-square&logo=go)
![Architecture](https://img.shields.io/badge/arch-Microservices-326CE5?style=flat-square)
![Architecture](https://img.shields.io/badge/arch-Event--Driven-FF9800?style=flat-square)
![Architecture](https://img.shields.io/badge/arch-DDD-6B4EFF?style=flat-square)
![gRPC](https://img.shields.io/badge/comm-gRPC-2DAAE1?style=flat-square&logo=grpc)
![RabbitMQ](https://img.shields.io/badge/comm-RabbitMQ-FF6600?style=flat-square&logo=rabbitmq&logoColor=white)
![Postgres](https://img.shields.io/badge/db-PostgreSQL-316192?style=flat-square&logo=postgresql&logoColor=white)
![Redis](https://img.shields.io/badge/cache-Redis-DD0031?style=flat-square&logo=redis&logoColor=white)
![OpenTelemetry](https://img.shields.io/badge/obs-OpenTelemetry-7B42BC?style=flat-square&logo=opentelemetry)
![Prometheus](https://img.shields.io/badge/obs-Prometheus-E6522C?style=flat-square&logo=prometheus)
![Grafana](https://img.shields.io/badge/obs-Grafana-F46800?style=flat-square&logo=grafana&logoColor=white)
![Docker](https://img.shields.io/badge/infra-Docker-2496ED?style=flat-square&logo=docker&logoColor=white)
![Kubernetes](https://img.shields.io/badge/infra-Kubernetes-326CE5?style=flat-square&logo=kubernetes&logoColor=white)


O **GoFleet** é um backend de alta performance projetado para demonstrar padrões avançados de engenharia de software, incluindo **Distributed Tracing**, **Metrics Instrumentation** e **State Pattern**. O sistema orquestra a criação de pedidos, processamento assíncrono e geolocalização de motoristas.
O diferencial deste projeto é a **Observabilidade Completa**: Logs, Métricas e Traces são correlacionados automaticamente através de toda a malha de serviços.
---

## 🏗️ Arquitetura

O sistema é um monorepo composto por três microsserviços principais:

1.  **API Service (`cmd/api`)**: Gateway REST que recebe pedidos.
2.  **Worker Service (`cmd/worker`)**: Processador assíncrono que consome filas, gerencia regras de negócio e persistência.
3.  **Fleet Service (`cmd/fleet`)**: Microsserviço gRPC de alta performance para busca geoespacial (Redis).

### Fluxo de Observabilidade e Dados

```mermaid
graph LR

subgraph Observability_Pipeline
    direction TB

    API -.->|Traces (OTLP)| Jaeger
    Worker -.->|Traces (OTLP)| Jaeger
    Fleet -.->|Traces (OTLP)| Jaeger

    API -.->|Metrics (Pull)| Prometheus
    Worker -.->|Metrics (Pull)| Prometheus
    Fleet -.->|Metrics (Pull)| Prometheus

    API -.->|Logs (JSON)| DockerOutput
    Worker -.->|Logs (JSON)| DockerOutput
    Fleet -.->|Logs (JSON)| DockerOutput

    DockerOutput -.->|Tail| Promtail
    Promtail -.->|Push| Loki
end
```
Jaeger --> Grafana
Prometheus --> Grafana
Loki --> Grafana

---

## 🛠️ Stack Tecnológico

* **Linguagem**: Go 1.25
* **Web Framework**: Chi Router v5 (Leve e idiomático)
* **RPC**: gRPC + Protobuf (Comunicação interna otimizada)
* **Database**: PostgreSQL 18 (SQLC para queries Type-Safe)
* **Cache/Geo**: Redis + Go-Redis (GeoSpatial Indexing)
* **Observabilidade**:
* **Tracing**: OpenTelemetry (OTel) com Jaeger.
* **Logs**: (JSON estruturado) -> Promtail -> Loki
* **Grafana**: Visualização unificada.
* **Metrics**: Prometheus (Custom Registry & Decorators).

---

## 🚀 Como Executar

### Pré-requisitos

* Docker & Docker Compose
* Go 1.25+ (para desenvolvimento local)
* Make

### Quick Start

1. **Suba o ambiente completo:**
```bash
make docker-up

```

*Isso iniciará API, Worker, Fleet, DB, RabbitMQ, Redis, Jaeger, Prometheus e Grafana.*
2. **Acesse as interfaces:**
* **Grafana**: [http://localhost:3000](https://www.google.com/search?q=http://localhost:3000) (Login: `admin` / `admin`)
* **Jaeger UI**: [http://localhost:16686](https://www.google.com/search?q=http://localhost:16686)
* **Prometheus**: [http://localhost:9090](https://www.google.com/search?q=http://localhost:9090)
* **RabbitMQ Mgmt**: [http://localhost:15672](https://www.google.com/search?q=http://localhost:15672) (guest/guest)


3. 🔌 API Endpoints & Teste

### Criar Pedido

```bash
curl -X POST http://localhost:8000/api/v1/orders \
-H "Content-Type: application/json" \
-d '{"id":"pedido-01", "price": 100.0, "tax": 10.0}'

```

**O que acontece nos bastidores:**

1. API salva como `PENDING`.
2. RabbitMQ recebe evento.
3. Worker processa e busca motorista via gRPC.
4. Worker atualiza pedido para `DISPATCHED`.

---

### Verificar Resultado (Banco de Dados)

```bash
docker exec -it gofleet_db psql -U root -d gofleet -c "SELECT * FROM orders WHERE id = 'pedido-demo-01';"

```

## 👁️ Observabilidade (Tracing)

O sistema implementa **Distributed Tracing** com OpenTelemetry.
Para visualizar o caminho da requisição entre os microsserviços:

1. Acesse o **Jaeger UI**: [http://localhost:16686](https://www.google.com/search?q=http://localhost:16686)
2. Em "Service", selecione `gofleet-api`.
3. Clique em **Find Traces**.
4. Você verá o gráfico completo: `API -> RabbitMQ -> Worker -> gRPC -> Redis`.


## 🧠 Decisões de Design (Staff Engineer View)

### 1. Decorator Pattern para Observabilidade

Em vez de poluir os Use Cases com códigos de métricas, utilizamos o padrão **Decorator**.

* **Arquivo**: `internal/application/usecase/order/create_metrics.go`
* **Benefício**: O `CreateUseCase` foca puramente em regras de negócio. O `CreateOrderMetricsDecorator` envolve a execução e registra a latência e contagem no Prometheus, mantendo o princípio de responsabilidade única (SRP).

### 2. State Pattern no Domínio

O ciclo de vida do pedido (`PENDING` -> `DISPATCHED`) é gerenciado através do padrão **State**.

* **Arquivo**: `internal/domain/entity/states.go`
* **Benefício**: Elimina condicionais complexas (`if status == "PENDING"`) e garante que transições inválidas retornem erro (ex: tentar cancelar um pedido já entregue).

### 3. Propagação de Contexto (Distributed Tracing)

Implementamos a propagação de contexto manual no RabbitMQ.

* **Arquivo**: `internal/infra/event/consumer.go`
* **Benefício**: O TraceID gerado na API HTTP viaja nos headers da mensagem AMQP e é extraído pelo Worker. Isso permite visualizar no Jaeger a jornada completa da requisição, mesmo passando por filas assíncronas.

### 4. Interface Segregation nas Métricas

Definimos uma interface explícita para métricas.

* **Arquivo**: `pkg/metrics/metrics.go`
* **Benefício**: Permite trocar o provedor de métricas (ex: de Prometheus para Datadog) sem alterar uma linha de código nos Use Cases, apenas trocando a implementação injetada no `main.go`.

### 4. Correlação de Logs e Traces

Implementamos um Logger Wrapper (pkg/logger) usando Uber Zap.

* **Decisão**: Todos os logs são estruturados em JSON.
* **Mágica**: O logger verifica automaticamente se existe um context.Context com um Span ativo. Se houver, ele injeta trace_id e span_id no log.
* **Resultado**: No Grafana, você pode visualizar um Trace e clicar para ver "Logs for this Trace", unindo infraestrutura e aplicação.

---

## 🧠 Decisões Arquiteturais

1. **Redis para Geolocalização:** Utilizamos `GEOSEARCH` do Redis em vez de calcular distâncias no PostgreSQL (PostGIS) ou em memória no Go. Isso garante latência de sub-milissegundos na busca de motoristas e torna o serviço de frota *stateless*.
2. **Worker Pattern:** A criação do pedido é desacoplada da busca por motoristas. Se o serviço de mapas cair, o pedido é salvo e processado depois (Resiliência).
3. **SQLC:** Optamos por não usar ORM (GORM) para ter controle total das queries e performance máxima no acesso ao PostgreSQL.
4. **gRPC:** Comunicação binária entre Worker e Fleet Service para economizar banda e tempo de CPU em alto tráfego.

## 📂 Estrutura de Pastas

```text
.
├── cmd/                # Entrypoints (api, fleet, worker)
├── configs/            # Configuração via Viper
├── internal/
│   ├── application/    # Regras de Aplicação
│   │   ├── usecase/    # Lógica de Negócio + Decorators
│   │   └── port/       # Interfaces (Ports)
│   ├── domain/         # Core Domain (Entities, Events, States)
│   └── infra/          # Implementações (Adapters)
│       ├── database/   # Repositórios e SQLC
│       ├── event/      # RabbitMQ Consumer/Dispatcher
│       ├── grpc/       # Protobuf e Service Implementation
│       └── web/        # HTTP Handlers e Middlewares
├── pkg/                # Libs Compartilhadas (Metrics, OTel, Utils)
└── sql/                # Migrations e Queries

```

---

## 📊 Métricas Chave (Prometheus)

O sistema expõe métricas customizadas na porta `:2112` para evitar ruído na porta principal da aplicação.

* `app_usecase_total`: Contador de execuções por Use Case e Status.
* `app_usecase_duration_seconds`: Histograma de latência (P95, P99).
* `http_request_duration_seconds`: Latência dos endpoints REST.
* `grpc_request_duration_seconds`: Latência das chamadas internas gRPC.
* `goofleet_order_created_total`: Métrica de negócio (Contador de Pedidos).

---

## 🧪 Testes

Execute a suíte de testes unitários:

```bash
make test

```

Os testes de entidade garantem a integridade das regras de negócio (ex: validação de preço negativo ou ID vazio).
