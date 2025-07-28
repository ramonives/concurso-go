package main

import (
	"context"
	"encoding/json"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"
	"go.opentelemetry.io/contrib/instrumentation/github.com/gorilla/mux/otelmux"
	"go.opentelemetry.io/contrib/instrumentation/net/http/otelhttp"
	"go.opentelemetry.io/otel/attribute"

	"concurso-go-app/internal/database"
	"concurso-go-app/internal/metrics"
	"concurso-go-app/internal/services"
	"concurso-go-app/internal/telemetry"
)

func main() {
	// Carregar variáveis de ambiente
	if err := godotenv.Load(); err != nil {
		log.Println("Arquivo .env não encontrado, usando variáveis do sistema")
	}

	// Inicializar telemetria
	cleanup, err := telemetry.InitTelemetry()
	if err != nil {
		log.Printf("Aviso: Erro ao inicializar telemetria: %v", err)
	} else {
		defer cleanup()
	}

	// Inicializar métricas
	metrics.InitMetrics()

	// Inicializar banco de dados
	if err := database.InitDB(); err != nil {
		log.Fatalf("Erro ao conectar ao banco: %v", err)
	}

	// Criar tabelas automaticamente na inicialização
	service := services.NewConcursoService()
	if err := service.CriarTabelas(); err != nil {
		log.Printf("Aviso: Erro ao criar tabelas na inicialização: %v", err)
	} else {
		log.Println("Tabelas verificadas/criadas na inicialização")
	}

	// Configurar rotas com middleware de telemetria
	r := mux.NewRouter()

	// Adicionar middleware de telemetria para todas as rotas
	r.Use(otelmux.Middleware("concurso-go-app"))

	// Endpoint para popular dados
	r.HandleFunc("/start", startHandler).Methods("POST")

	// Endpoint para extrair registros e enviar para Kafka
	r.HandleFunc("/extrair/{data}", extrairHandler).Methods("POST")

	// Endpoint para consumir registros do Kafka
	r.HandleFunc("/consumir/{data}", consumirHandler).Methods("POST")

	// Endpoint para limpar tópico Kafka
	r.HandleFunc("/limpar", limparKafkaHandler).Methods("POST")

	// Endpoint de health check
	r.HandleFunc("/health", healthHandler).Methods("GET")

	// Endpoint de métricas Prometheus
	r.Handle("/metrics", metrics.MetricsHandler()).Methods("GET")

	// Configurar servidor HTTP com instrumentação
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	// Criar servidor HTTP com instrumentação OpenTelemetry
	handler := otelhttp.NewHandler(r, "concurso-go-app")
	server := &http.Server{
		Addr:    ":" + port,
		Handler: handler,
	}

	// Canal para receber sinais de interrupção
	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// Iniciar servidor em goroutine
	go func() {
		log.Printf("Servidor iniciado na porta %s", port)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("Erro ao iniciar servidor: %v", err)
		}
	}()

	// Aguardar sinal de interrupção
	<-stop
	log.Println("Recebido sinal de interrupção, desligando servidor...")

	// Desligar servidor graciosamente
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := server.Shutdown(ctx); err != nil {
		log.Printf("Erro ao desligar servidor: %v", err)
	}

	log.Println("Servidor desligado com sucesso")
}

func healthHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	response := map[string]string{
		"status":  "healthy",
		"service": "concurso-go-app",
	}
	json.NewEncoder(w).Encode(response)
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.StartSpan(r.Context(), "start_handler")
	defer span.End()

	w.Header().Set("Content-Type", "application/json")

	service := services.NewConcursoService()

	// Popular dados (as tabelas são criadas automaticamente se não existirem)
	if err := service.PopularDados(); err != nil {
		span.RecordError(err)
		http.Error(w, `{"erro": "Erro ao popular dados: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"mensagem": "Dados populados com sucesso (3100 registros)",
	}
	json.NewEncoder(w).Encode(response)
}

func extrairHandler(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.StartSpan(r.Context(), "extrair_handler")
	defer span.End()

	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	data := vars["data"]

	// Validar formato da data
	if len(data) != 10 || data[4] != '-' || data[7] != '-' {
		span.SetAttributes(attribute.String("data.invalida", data))
		http.Error(w, `{"erro": "Formato de data inválido. Use YYYY-MM-DD"}`, http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("data", data))

	service := services.NewConcursoService()

	if err := service.ExtrairRegistros(data); err != nil {
		span.RecordError(err)
		http.Error(w, `{"erro": "Erro ao extrair registros: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"mensagem": "Registros extraídos e enviados para Kafka com sucesso",
		"data":     data,
	}
	json.NewEncoder(w).Encode(response)
}

func consumirHandler(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.StartSpan(r.Context(), "consumir_handler")
	defer span.End()

	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	data := vars["data"]

	// Validar formato da data
	if len(data) != 10 || data[4] != '-' || data[7] != '-' {
		span.SetAttributes(attribute.String("data.invalida", data))
		http.Error(w, `{"erro": "Formato de data inválido. Use YYYY-MM-DD"}`, http.StatusBadRequest)
		return
	}

	span.SetAttributes(attribute.String("data", data))

	service := services.NewConcursoService()

	if err := service.ConsumirRegistros(data); err != nil {
		span.RecordError(err)
		http.Error(w, `{"erro": "Erro ao consumir registros: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"mensagem": "Registros consumidos e processados com sucesso",
		"data":     data,
	}
	json.NewEncoder(w).Encode(response)
}

func limparKafkaHandler(w http.ResponseWriter, r *http.Request) {
	_, span := telemetry.StartSpan(r.Context(), "limpar_kafka_handler")
	defer span.End()

	w.Header().Set("Content-Type", "application/json")

	// Usar o service para limpar (que tem a lógica melhorada)
	service := services.NewConcursoService()

	if err := service.LimparTopicoKafka(); err != nil {
		span.RecordError(err)
		http.Error(w, `{"erro": "Erro ao limpar Kafka: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"mensagem": "Tópico Kafka limpo com sucesso",
	}
	json.NewEncoder(w).Encode(response)
}
