# Concurso Go App

Aplicação Go para processamento de dados de concursos com integração OpenTelemetry e Tempo para observabilidade.

## 🚀 Funcionalidades

- **Processamento de Dados**: População e extração de dados de concursos
- **Integração Kafka**: Produção e consumo de mensagens
- **Observabilidade**: Rastreamento distribuído com OpenTelemetry
- **Visualização**: Interface Grafana para análise de traces

## 🏗️ Arquitetura

```
┌─────────────────┐    ┌──────────────┐    ┌─────────────┐
│   Concurso App  │───▶│    Kafka     │───▶│   Database  │
│   (Go + OTEL)   │    │              │    │   (MySQL)   │
└─────────────────┘    └──────────────┘    └─────────────┘
         │
         ▼
┌─────────────────┐    ┌─────────────┐
│   OpenTelemetry │───▶│    Tempo    │
│   (Traces)      │    │ (Backend)   │
└─────────────────┘    └─────────────┘
                              │
                              ▼
                       ┌─────────────┐
                       │   Grafana   │
                       │ (UI)        │
                       └─────────────┘
```

## 📋 Pré-requisitos

- Go 1.21+
- Docker e Docker Compose
- MySQL (via Docker)
- jq (para formatação JSON nos testes)

## 🛠️ Instalação

1. **Clone o repositório**:
```bash
git clone <repository-url>
cd concurso-go-app
```

2. **Configure as variáveis de ambiente**:
```bash
cp env.example .env
# Edite o arquivo .env conforme necessário
```

3. **Instale as dependências**:
```bash
go mod tidy
```

## 🚀 Execução

### Opção 1: Usando o script de teste (Recomendado)

```bash
./teste-telemetria.sh
```

Este script irá:
- Iniciar todos os serviços (Kafka, Tempo, Grafana)
- Testar todos os endpoints da aplicação
- Gerar traces para visualização

### Opção 2: Execução manual

1. **Iniciar serviços de infraestrutura**:
```bash
cd docker
docker-compose up -d
cd ..
```

2. **Executar a aplicação**:
```bash
go run cmd/main.go
```

3. **Testar endpoints**:
```bash
# Health check
curl http://localhost:8080/health

# Popular dados
curl -X POST http://localhost:8080/start

# Extrair registros
curl -X POST http://localhost:8080/extrair/2024-01-15

# Consumir registros
curl -X POST http://localhost:8080/consumir/2024-01-15

# Limpar Kafka
curl -X POST http://localhost:8080/limpar
```

## 📊 Observabilidade

### URLs de Acesso

- **Aplicação**: http://localhost:8080
- **Grafana**: http://localhost:3000
- **Tempo**: http://localhost:3200

### Visualizando Traces no Grafana

1. Acesse http://localhost:3000
2. Vá em **Explore**
3. Selecione **Tempo** como datasource
4. Digite a query: `{service.name="concurso-go-app"}`
5. Clique em **Run Query**

### Visualizando Traces no Tempo

1. Acesse http://localhost:3200
2. Use a query: `{service.name="concurso-go-app"}`
3. Explore os traces detalhados

## 🔧 Configuração

### Variáveis de Ambiente

| Variável | Descrição | Padrão |
|----------|-----------|--------|
| `DB_HOST` | Host do MySQL | localhost |
| `DB_PORT` | Porta do MySQL | 3306 |
| `DB_USER` | Usuário do MySQL | root |
| `DB_PASSWORD` | Senha do MySQL | root |
| `DB_NAME` | Nome do banco | mentoria_db |
| `KAFKA_BROKER` | Broker do Kafka | localhost:9092 |
| `KAFKA_TOPIC` | Tópico principal | concurso |
| `KAFKA_ERROR_TOPIC` | Tópico de erros | concurso_erros |
| `API_PORT` | Porta da API | 8080 |
| `OTEL_SERVICE_NAME` | Nome do serviço | concurso-go-app |
| `TEMPO_ENDPOINT` | Endpoint do Tempo | localhost:4317 |

## 📁 Estrutura do Projeto

```
concurso-go-app/
├── cmd/
│   └── main.go              # Ponto de entrada da aplicação
├── internal/
│   ├── database/
│   │   └── mysql.go         # Configuração do banco
│   ├── kafka/
│   │   ├── consumer.go      # Consumidor Kafka
│   │   └── producer.go      # Produtor Kafka
│   ├── models/
│   │   ├── concurso.go      # Modelos de dados
│   │   ├── kafka.go         # Modelos Kafka
│   │   └── logs.go          # Modelos de logs
│   ├── services/
│   │   └── concurso.go      # Lógica de negócio
│   └── telemetry/
│       └── telemetry.go     # Configuração OpenTelemetry
├── docker/
│   ├── docker-compose.yml   # Serviços Docker
│   ├── tempo.yaml          # Configuração Tempo
│   └── grafana-datasources.yaml # Configuração Grafana
├── logs/                    # Logs da aplicação
├── go.mod                   # Dependências Go
├── go.sum                   # Checksums das dependências
├── env.example              # Exemplo de variáveis de ambiente
├── teste.sh                 # Script de teste original
├── teste-telemetria.sh      # Script de teste com telemetria
└── README.md               # Este arquivo
```

## 🔍 Endpoints da API

| Método | Endpoint | Descrição |
|--------|----------|-----------|
| GET | `/health` | Health check da aplicação |
| POST | `/start` | Popula dados iniciais |
| POST | `/extrair/{data}` | Extrai registros para Kafka |
| POST | `/consumir/{data}` | Consome registros do Kafka |
| POST | `/limpar` | Limpa tópico Kafka |

## 🐛 Troubleshooting

### Problemas comuns

1. **Erro de conexão com Tempo**:
   - Verifique se o container está rodando: `docker ps | grep tempo`
   - Verifique os logs: `docker logs concurso_tempo`

2. **Traces não aparecem no Grafana**:
   - Aguarde alguns segundos após gerar traces
   - Verifique se o datasource Tempo está configurado corretamente
   - Use a query correta: `{service.name="concurso-go-app"}`

3. **Erro de conexão com Kafka**:
   - Verifique se o Zookeeper e Kafka estão rodando
   - Verifique os logs: `docker logs concurso_kafka`

### Logs úteis

```bash
# Logs da aplicação
docker logs concurso-go-app

# Logs do Tempo
docker logs concurso_tempo

# Logs do Grafana
docker logs concurso_grafana

# Logs do Kafka
docker logs concurso_kafka
```

## 🤝 Contribuição

1. Fork o projeto
2. Crie uma branch para sua feature (`git checkout -b feature/AmazingFeature`)
3. Commit suas mudanças (`git commit -m 'Add some AmazingFeature'`)
4. Push para a branch (`git push origin feature/AmazingFeature`)
5. Abra um Pull Request

## 📄 Licença

Este projeto está sob a licença MIT. Veja o arquivo `LICENSE` para mais detalhes.
