package models

import "time"

// ExtracaoLog representa o log de extração
type ExtracaoLog struct {
	Data          string    `json:"data"`
	Lote          string    `json:"lote"`
	TotalExtraido int       `json:"total_extraido"`
	TempoExecucao string    `json:"tempo_execucao"`
	Timestamp     time.Time `json:"timestamp"`
}

// KafkaCargaLog representa o log de carga no Kafka
type KafkaCargaLog struct {
	Header     KafkaHeader `json:"header"`
	Footer     KafkaFooter `json:"footer"`
	TempoEnvio string      `json:"tempo_envio"`
	Timestamp  time.Time   `json:"timestamp"`
}

// ConsumoLog representa o log de consumo
type ConsumoLog struct {
	Data               string    `json:"data"`
	Lote               string    `json:"lote"`
	TotalConsumido     int       `json:"total_consumido"`
	TempoProcessamento string    `json:"tempo_processamento"`
	Status             string    `json:"status"`
	Timestamp          time.Time `json:"timestamp"`
}

// LoteErroLog representa o log de erro de lote
type LoteErroLog struct {
	Data               string     `json:"data"`
	Lote               string     `json:"lote"`
	Motivo             string     `json:"motivo"`
	TotalRegistros     int        `json:"total_registros"`
	RegistrosValidos   int        `json:"registros_validos"`
	RegistrosInvalidos int        `json:"registros_invalidos"`
	RegistrosComErro   []Concurso `json:"registros_com_erro"`
	Timestamp          time.Time  `json:"timestamp"`
}

// ErroLog representa o log de erro geral
type ErroLog struct {
	Categoria  string      `json:"categoria"`   // "BANCO", "KAFKA", "VALIDACAO"
	Mensagem   string      `json:"mensagem"`    // Mensagem customizada
	ErrorTrace string      `json:"error_trace"` // Stack trace técnico
	LinhaErro  string      `json:"linha_erro"`  // Linha onde ocorreu
	Payload    interface{} `json:"payload"`     // Dados que causaram erro
	Timestamp  time.Time   `json:"timestamp"`
}

// ErroKafkaLog representa erro específico do Kafka
type ErroKafkaLog struct {
	IDLinhaKafka string      `json:"id_linha_kafka"`
	Payload      interface{} `json:"payload"`
	Motivo       string      `json:"motivo"`
	Timestamp    time.Time   `json:"timestamp"`
}
