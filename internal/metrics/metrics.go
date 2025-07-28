package metrics

import (
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	// Contadores de requisições
	RequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "concurso_requests_total",
			Help: "Total de requisições por endpoint",
		},
		[]string{"endpoint", "method", "status"},
	)

	// Duração das requisições
	RequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "concurso_request_duration_seconds",
			Help:    "Duração das requisições em segundos",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"endpoint", "method"},
	)

	// Métricas de negócio
	RegistrosProcessados = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "concurso_registros_processados_total",
			Help: "Total de registros processados",
		},
		[]string{"operacao", "status"},
	)

	// Métricas de Kafka
	KafkaMessagesSent = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "concurso_kafka_messages_sent_total",
			Help: "Total de mensagens enviadas para Kafka",
		},
	)

	KafkaMessagesReceived = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "concurso_kafka_messages_received_total",
			Help: "Total de mensagens recebidas do Kafka",
		},
	)

	// Métricas de banco de dados
	DatabaseOperations = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "concurso_database_operations_total",
			Help: "Total de operações no banco de dados",
		},
		[]string{"operation", "table"},
	)
)

// InitMetrics inicializa e registra todas as métricas
func InitMetrics() {
	// Registrar métricas
	prometheus.MustRegister(
		RequestsTotal,
		RequestDuration,
		RegistrosProcessados,
		KafkaMessagesSent,
		KafkaMessagesReceived,
		DatabaseOperations,
	)
}

// MetricsHandler retorna o handler para o endpoint /metrics
func MetricsHandler() http.Handler {
	return promhttp.Handler()
}

// RecordRequest registra uma requisição
func RecordRequest(endpoint, method, status string, duration float64) {
	RequestsTotal.WithLabelValues(endpoint, method, status).Inc()
	RequestDuration.WithLabelValues(endpoint, method).Observe(duration)
}

// RecordRegistroProcessado registra um registro processado
func RecordRegistroProcessado(operacao, status string) {
	RegistrosProcessados.WithLabelValues(operacao, status).Inc()
}

// RecordKafkaMessageSent registra uma mensagem enviada para Kafka
func RecordKafkaMessageSent() {
	KafkaMessagesSent.Inc()
}

// RecordKafkaMessageReceived registra uma mensagem recebida do Kafka
func RecordKafkaMessageReceived() {
	KafkaMessagesReceived.Inc()
}

// RecordDatabaseOperation registra uma operação no banco
func RecordDatabaseOperation(operation, table string) {
	DatabaseOperations.WithLabelValues(operation, table).Inc()
}
