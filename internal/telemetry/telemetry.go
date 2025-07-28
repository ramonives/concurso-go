package telemetry

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"go.opentelemetry.io/otel"
	"go.opentelemetry.io/otel/exporters/otlp/otlptrace/otlptracegrpc"
	"go.opentelemetry.io/otel/propagation"
	"go.opentelemetry.io/otel/sdk/resource"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	semconv "go.opentelemetry.io/otel/semconv/v1.17.0"
	"go.opentelemetry.io/otel/trace"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
)

var tracer trace.Tracer

// InitTelemetry inicializa o OpenTelemetry com exportação para Tempo
func InitTelemetry() (func(), error) {
	ctx := context.Background()

	// Obter configurações do ambiente
	serviceName := os.Getenv("OTEL_SERVICE_NAME")
	if serviceName == "" {
		serviceName = "concurso-go-app"
	}

	tempoEndpoint := os.Getenv("TEMPO_ENDPOINT")
	if tempoEndpoint == "" {
		tempoEndpoint = "localhost:4317" // Endpoint padrão do Tempo
	}

	// Configurar o resource com informações do serviço
	res, err := resource.New(ctx,
		resource.WithAttributes(
			semconv.ServiceName(serviceName),
			semconv.ServiceVersion("1.0.0"),
		),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create resource: %w", err)
	}

	// Configurar o exporter OTLP para Tempo
	conn, err := grpc.DialContext(ctx, tempoEndpoint,
		grpc.WithTransportCredentials(insecure.NewCredentials()),
		grpc.WithBlock(),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create gRPC connection to collector: %w", err)
	}

	exporter, err := otlptracegrpc.New(ctx, otlptracegrpc.WithGRPCConn(conn))
	if err != nil {
		return nil, fmt.Errorf("failed to create trace exporter: %w", err)
	}

	// Configurar o TracerProvider
	tp := sdktrace.NewTracerProvider(
		sdktrace.WithBatcher(exporter,
			sdktrace.WithBatchTimeout(5*time.Second),
			sdktrace.WithMaxExportBatchSize(1000),
		),
		sdktrace.WithResource(res),
	)

	// Configurar propagação de contexto
	otel.SetTextMapPropagator(propagation.NewCompositeTextMapPropagator(
		propagation.TraceContext{},
		propagation.Baggage{},
	))

	// Registrar o TracerProvider global
	otel.SetTracerProvider(tp)

	// Criar tracer para este serviço
	tracer = tp.Tracer(serviceName)

	log.Printf("OpenTelemetry inicializado com sucesso. Enviando traces para: %s", tempoEndpoint)

	// Retornar função de cleanup
	cleanup := func() {
		if err := tp.Shutdown(ctx); err != nil {
			log.Printf("Erro ao desligar TracerProvider: %v", err)
		}
	}

	return cleanup, nil
}

// GetTracer retorna o tracer configurado
func GetTracer() trace.Tracer {
	return tracer
}

// StartSpan inicia um novo span com o nome especificado
func StartSpan(ctx context.Context, name string, opts ...trace.SpanStartOption) (context.Context, trace.Span) {
	return tracer.Start(ctx, name, opts...)
}
