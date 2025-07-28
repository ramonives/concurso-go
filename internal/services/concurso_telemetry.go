package services

import (
	"context"
	"time"

	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/trace"

	"concurso-go-app/internal/telemetry"
)

// Exemplo de como instrumentar métodos do ConcursoService com OpenTelemetry

// ExtrairRegistrosInstrumentado é uma versão instrumentada do método ExtrairRegistros
func (s *ConcursoService) ExtrairRegistrosInstrumentado(ctx context.Context, data string) error {
	ctx, span := telemetry.StartSpan(ctx, "extrair_registros")
	defer span.End()

	// Adicionar atributos ao span
	span.SetAttributes(
		attribute.String("data", data),
		attribute.String("operacao", "extracao"),
	)

	// Medir tempo de execução
	start := time.Now()
	defer func() {
		span.SetAttributes(attribute.String("duracao", time.Since(start).String()))
	}()

	// Executar a operação original
	err := s.ExtrairRegistros(data)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("erro", err.Error()))
		return err
	}

	// Adicionar evento de sucesso
	span.AddEvent("extracao_concluida", trace.WithAttributes(
		attribute.String("data", data),
		attribute.String("status", "sucesso"),
	))

	return nil
}

// ConsumirRegistrosInstrumentado é uma versão instrumentada do método ConsumirRegistros
func (s *ConcursoService) ConsumirRegistrosInstrumentado(ctx context.Context, data string) error {
	ctx, span := telemetry.StartSpan(ctx, "consumir_registros")
	defer span.End()

	span.SetAttributes(
		attribute.String("data", data),
		attribute.String("operacao", "consumo"),
	)

	start := time.Now()
	defer func() {
		span.SetAttributes(attribute.String("duracao", time.Since(start).String()))
	}()

	err := s.ConsumirRegistros(data)
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("erro", err.Error()))
		return err
	}

	span.AddEvent("consumo_concluido", trace.WithAttributes(
		attribute.String("data", data),
		attribute.String("status", "sucesso"),
	))

	return nil
}

// PopularDadosInstrumentado é uma versão instrumentada do método PopularDados
func (s *ConcursoService) PopularDadosInstrumentado(ctx context.Context) error {
	ctx, span := telemetry.StartSpan(ctx, "popular_dados")
	defer span.End()

	span.SetAttributes(attribute.String("operacao", "populacao"))

	start := time.Now()
	defer func() {
		span.SetAttributes(attribute.String("duracao", time.Since(start).String()))
	}()

	err := s.PopularDados()
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("erro", err.Error()))
		return err
	}

	span.AddEvent("populacao_concluida", trace.WithAttributes(
		attribute.String("status", "sucesso"),
		attribute.Int("registros_esperados", 3100),
	))

	return nil
}

// CriarTabelasInstrumentado é uma versão instrumentada do método CriarTabelas
func (s *ConcursoService) CriarTabelasInstrumentado(ctx context.Context) error {
	ctx, span := telemetry.StartSpan(ctx, "criar_tabelas")
	defer span.End()

	span.SetAttributes(attribute.String("operacao", "criacao_tabelas"))

	start := time.Now()
	defer func() {
		span.SetAttributes(attribute.String("duracao", time.Since(start).String()))
	}()

	err := s.CriarTabelas()
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("erro", err.Error()))
		return err
	}

	span.AddEvent("tabelas_criadas", trace.WithAttributes(
		attribute.String("status", "sucesso"),
		attribute.String("tabelas", "concurso, concurso_processado"),
	))

	return nil
}

// LimparTopicoKafkaInstrumentado é uma versão instrumentada do método LimparTopicoKafka
func (s *ConcursoService) LimparTopicoKafkaInstrumentado(ctx context.Context) error {
	ctx, span := telemetry.StartSpan(ctx, "limpar_kafka")
	defer span.End()

	span.SetAttributes(attribute.String("operacao", "limpeza_kafka"))

	start := time.Now()
	defer func() {
		span.SetAttributes(attribute.String("duracao", time.Since(start).String()))
	}()

	err := s.LimparTopicoKafka()
	if err != nil {
		span.RecordError(err)
		span.SetAttributes(attribute.String("erro", err.Error()))
		return err
	}

	span.AddEvent("kafka_limpo", trace.WithAttributes(
		attribute.String("status", "sucesso"),
		attribute.String("topico", "concurso"),
	))

	return nil
}
