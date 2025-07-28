package services

import (
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"math/rand"
	"os"
	"os/exec"
	"path/filepath"
	"runtime/debug"
	"strings"
	"time"

	"concurso-go-app/internal/database"
	"concurso-go-app/internal/kafka"
	"concurso-go-app/internal/models"
)

type ConcursoService struct{}

func NewConcursoService() *ConcursoService {
	return &ConcursoService{}
}

// CriarTabelas cria as tabelas concurso e concurso_processado se não existirem
func (s *ConcursoService) CriarTabelas() error {
	// Criar tabela concurso
	_, err := database.DB.Exec(`
		CREATE TABLE IF NOT EXISTS concurso (
			id INT AUTO_INCREMENT PRIMARY KEY,
			nome VARCHAR(255) NOT NULL,
			status VARCHAR(50),
			data_prova DATE NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("erro ao criar tabela concurso: %v", err)
	}

	// Criar tabela concurso_processado
	_, err = database.DB.Exec(`
		CREATE TABLE IF NOT EXISTS concurso_processado (
			id INT AUTO_INCREMENT PRIMARY KEY,
			nome VARCHAR(255) NOT NULL,
			status VARCHAR(50) NOT NULL,
			data_prova DATE NOT NULL
		)
	`)
	if err != nil {
		return fmt.Errorf("erro ao criar tabela concurso_processado: %v", err)
	}

	return nil
}

// PopularDados popula as tabelas com dados de teste
func (s *ConcursoService) PopularDados() error {
	// Criar tabelas se não existirem
	if err := s.CriarTabelas(); err != nil {
		return fmt.Errorf("erro ao criar tabelas: %v", err)
	}

	// Limpar tabelas
	_, err := database.DB.Exec("DELETE FROM concurso")
	if err != nil {
		return fmt.Errorf("erro ao limpar tabela concurso: %v", err)
	}

	// Gerar dados de 01/01/2025 até 31/01/2025
	dataInicio := time.Date(2025, 1, 1, 0, 0, 0, 0, time.UTC)
	dataFim := time.Date(2025, 1, 31, 0, 0, 0, 0, time.UTC)

	// Iniciar transação para melhor performance
	tx, err := database.DB.Begin()
	if err != nil {
		return fmt.Errorf("erro ao iniciar transação: %v", err)
	}
	defer func() {
		if err != nil {
			tx.Rollback()
		}
	}()

	contador := 0
	totalDias := 31
	diaAtual := 0
	const BATCH_SIZE = 1000 // Batch de 1000 registros por INSERT

	fmt.Printf("Iniciando população de dados (BATCH INSERT)...\n")
	fmt.Printf("Período: 01/01/2025 até 31/01/2025\n")

	// Calcular total esperado
	registrosDia01 := 1000
	registrosOutrosDias := 161290 // (5MM - 1000) / 30 dias
	totalEsperado := registrosDia01 + (30 * registrosOutrosDias)
	fmt.Printf("Total esperado: %d registros\n", totalEsperado)

	for data := dataInicio; !data.After(dataFim); data = data.AddDate(0, 0, 1) {
		diaAtual++

		var registrosPorDia int
		if data.Day() == 1 {
			registrosPorDia = 1000 // Dia 01: 1000 registros
			fmt.Printf("Processando dia %d/%d: %s (%d registros - 900 aprovados + 100 NULL)\n",
				diaAtual, totalDias, data.Format("2006-01-02"), registrosPorDia)
		} else {
			registrosPorDia = 161290 // Dia 02-31: ~161K registros
			fmt.Printf("Processando dia %d/%d: %s (%d registros - aprovados/reprovados)\n",
				diaAtual, totalDias, data.Format("2006-01-02"), registrosPorDia)
		}

		// Processar em batches usando padrão otimizado
		const batchSize = 1000
		insertBase := `INSERT INTO concurso (nome, status, data_prova) VALUES `

		valCount := 0
		valueStrings := []string{}
		valueArgs := []interface{}{}

		for i := 1; i <= registrosPorDia; i++ {
			nome := fmt.Sprintf("Candidato_%d_%s", i, data.Format("2006-01-02"))

			var status sql.NullString
			if data.Day() == 1 {
				// Dia 01: 900 aprovados + 100 NULL
				if i <= 900 {
					status.String = "aprovado"
					status.Valid = true
				} else {
					status.Valid = false // NULL
				}
			} else {
				// Dia 02-31: 70% aprovado + 30% reprovado
				status.Valid = true
				if rand.Float32() < 0.7 {
					status.String = "aprovado"
				} else {
					status.String = "reprovado"
				}
			}

			valueStrings = append(valueStrings, "(?, ?, ?)")

			valueArgs = append(valueArgs, nome, status, data.Format("2006-01-02"))
			valCount += 3

			if i%batchSize == 0 || i == registrosPorDia {
				query := insertBase + strings.Join(valueStrings, ",")
				_, err := tx.Exec(query, valueArgs...)
				if err != nil {
					return fmt.Errorf("erro ao inserir batch: %v", err)
				}

				contador += len(valueStrings)
				fmt.Printf("  Progresso: %d registros inseridos (batch %d-%d)\n", contador, i-len(valueStrings)+1, i)

				// Limpar arrays para reutilizar
				valueStrings = []string{}
				valueArgs = []interface{}{}
				valCount = 0
			}
		}
	}

	// Commit da transação
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("erro ao fazer commit: %v", err)
	}

	log.Printf("Populadas %d registros com sucesso (OTIMIZADO)", contador)
	return nil
}

// ExtrairRegistros extrai registros por data e envia para Kafka
func (s *ConcursoService) ExtrairRegistros(data string) error {
	inicio := time.Now()
	// Criar tabelas se não existirem
	if err := s.CriarTabelas(); err != nil {
		// Log de erro detalhado para banco
		if logErr := s.gerarLogErroDetalhado(data, "BANCO", "O banco de dados está fora do ar! Sua Integração foi abortada", err, map[string]string{"operacao": "criar_tabelas", "data": data}); logErr != nil {
			fmt.Printf("⚠️  Erro ao gerar log de erro: %v\n", logErr)
		}
		return fmt.Errorf("erro ao criar tabelas: %v", err)
	}

	// Buscar total de registros primeiro
	var totalRegistros int
	err := database.DB.QueryRow(`
		SELECT COUNT(*) 
		FROM concurso 
		WHERE DATE(data_prova) = ?
	`, data).Scan(&totalRegistros)
	if err != nil {
		return fmt.Errorf("erro ao contar registros: %v", err)
	}

	if totalRegistros == 0 {
		return fmt.Errorf("nenhum registro encontrado para a data %s", data)
	}

	fmt.Printf("📊 Encontrados %d registros para extração\n", totalRegistros)

	// Extrair em batches para não sobrecarregar memória
	const BATCH_SIZE = 10000
	var registros []models.Concurso

	for offset := 0; offset < totalRegistros; offset += BATCH_SIZE {
		rows, err := database.DB.Query(`
			SELECT id, nome, status, data_prova 
			FROM concurso 
			WHERE DATE(data_prova) = ?
			LIMIT ? OFFSET ?
		`, data, BATCH_SIZE, offset)
		if err != nil {
			return fmt.Errorf("erro ao buscar registros: %v", err)
		}

		batchRegistros := []models.Concurso{}
		for rows.Next() {
			var c models.Concurso
			err := rows.Scan(&c.ID, &c.Nome, &c.Status, &c.DataProva)
			if err != nil {
				rows.Close()
				return fmt.Errorf("erro ao ler registro: %v", err)
			}
			batchRegistros = append(batchRegistros, c)
		}
		rows.Close()

		registros = append(registros, batchRegistros...)
		fmt.Printf("  Extraídos: %d/%d registros\n", len(registros), totalRegistros)
	}

	// Inicializar produtor Kafka
	if err := kafka.InitProducer(); err != nil {
		// Log de erro detalhado para Kafka
		if logErr := s.gerarLogErroDetalhado(data, "KAFKA", "Erro ao inicializar produtor Kafka", err, map[string]string{"operacao": "inicializar_produtor", "data": data}); logErr != nil {
			fmt.Printf("⚠️  Erro ao gerar log de erro: %v\n", logErr)
		}
		return fmt.Errorf("erro ao inicializar produtor Kafka: %v", err)
	}

	// Enviar header
	agora := time.Now()
	lote := fmt.Sprintf("concurso%s", agora.Format("02/01/2006 15:04:05"))
	loteArquivo := fmt.Sprintf("concurso%s", agora.Format("02012006_150405")) // Para nomes de arquivo
	topicName := fmt.Sprintf("concurso_%s", data)                             // Tópico específico por data (mantém compatibilidade)

	header := models.KafkaHeader{
		Lote:          lote,
		TotalEsperado: len(registros),
		InicioEnvio:   data, // Só a data, sem timestamp
	}

	if err := kafka.SendMessage(topicName, header); err != nil {
		// Log de erro detalhado para Kafka
		if logErr := s.gerarLogErroDetalhado(data, "KAFKA", "Erro ao enviar header para Kafka", err, map[string]interface{}{"operacao": "enviar_header", "data": data, "header": header}); logErr != nil {
			fmt.Printf("⚠️  Erro ao gerar log de erro: %v\n", logErr)
		}
		return fmt.Errorf("erro ao enviar header: %v", err)
	}

	// Enviar registros em batches para Kafka
	totalProcessado := 0
	registrosParaEnviar := len(registros)
	const KAFKA_BATCH_SIZE = 10000 // Aumentado para 10K por batch

	fmt.Printf("Enviando %d registros para Kafka (BATCH)...\n", registrosParaEnviar)

	// Enviar em batches para melhor performance
	for i := 0; i < registrosParaEnviar; i += KAFKA_BATCH_SIZE {
		end := i + KAFKA_BATCH_SIZE
		if end > registrosParaEnviar {
			end = registrosParaEnviar
		}

		// Enviar batch atual
		for j := i; j < end; j++ {
			if err := kafka.SendMessage(topicName, registros[j]); err != nil {
				// Log de erro detalhado para Kafka
				if logErr := s.gerarLogErroDetalhado(data, "KAFKA", "Erro ao enviar registro para Kafka", err, map[string]interface{}{"operacao": "enviar_registro", "data": data, "registro": registros[j], "total_processado": totalProcessado}); logErr != nil {
					fmt.Printf("⚠️  Erro ao gerar log de erro: %v\n", logErr)
				}
				return fmt.Errorf("erro ao enviar registro: %v", err)
			}
			totalProcessado++
		}

		// Log a cada batch
		fmt.Printf("  Enviados: %d/%d registros (batch %d-%d)\n", totalProcessado, registrosParaEnviar, i+1, end)
	}

	// Enviar footer
	footer := models.KafkaFooter{
		Lote:            lote,
		TotalProcessado: totalProcessado,
		FimEnvio:        data, // Só a data, sem timestamp
	}

	if err := kafka.SendMessage(topicName, footer); err != nil {
		// Log de erro detalhado para Kafka
		if logErr := s.gerarLogErroDetalhado(data, "KAFKA", "Erro ao enviar footer para Kafka", err, map[string]interface{}{"operacao": "enviar_footer", "data": data, "footer": footer}); logErr != nil {
			fmt.Printf("⚠️  Erro ao gerar log de erro: %v\n", logErr)
		}
		return fmt.Errorf("erro ao enviar footer: %v", err)
	}

	// Validar se quantidade enviada bate com quantidade processada
	if totalProcessado != len(registros) {
		erroCarga := fmt.Errorf("quantidade enviada (%d) diferente da quantidade processada (%d)", totalProcessado, len(registros))
		if logErr := s.gerarLogErroDetalhado(data, "KAFKA", "Ocorreram erros na carga", erroCarga, map[string]interface{}{"operacao": "validacao_carga", "data": data, "total_enviado": totalProcessado, "total_registros": len(registros), "header": header, "footer": footer}); logErr != nil {
			fmt.Printf("⚠️  Erro ao gerar log de erro: %v\n", logErr)
		}
		return fmt.Errorf("erro na carga: %v", erroCarga)
	}

	tempoTotal := time.Since(inicio)

	// Gerar logs
	if err := s.gerarLogExtracao(data, loteArquivo, len(registros), tempoTotal); err != nil {
		fmt.Printf("⚠️  Erro ao gerar log de extração: %v\n", err)
	}

	if err := s.gerarLogKafkaCarga(header, footer, tempoTotal); err != nil {
		fmt.Printf("⚠️  Erro ao gerar log de carga Kafka: %v\n", err)
	}

	log.Printf("Enviados %d registros para Kafka (lote: %s)", totalProcessado, lote)
	return nil
}

// ConsumirRegistros consome registros do Kafka e processa
func (s *ConcursoService) ConsumirRegistros(data string) error {
	inicio := time.Now()
	// Criar tabelas se não existirem
	if err := s.CriarTabelas(); err != nil {
		return fmt.Errorf("erro ao criar tabelas: %v", err)
	}

	// Inicializar consumidor Kafka
	if err := kafka.InitConsumer(); err != nil {
		return fmt.Errorf("erro ao inicializar consumidor Kafka: %v", err)
	}
	defer kafka.CloseConsumer()

	agora := time.Now()
	loteArquivo := fmt.Sprintf("concurso%s", agora.Format("02012006_150405")) // Para nomes de arquivo
	var header *models.KafkaHeader
	var footer *models.KafkaFooter
	var lote string // Será definido quando encontrar o header
	var registros []models.Concurso
	var registrosValidos []models.Concurso
	var registrosComErro bool = false // Marca se há erro no lote

	// Handler para processar mensagens - OTIMIZADO COM DEBUG
	handler := func(message []byte) bool {
		// Tentar header
		var headerMsg models.KafkaHeader
		if err := json.Unmarshal(message, &headerMsg); err == nil {
			if headerMsg.TotalEsperado > 0 {
				header = &headerMsg
				lote = headerMsg.Lote // Usar o lote do header
				fmt.Printf("📋 Header encontrado: %d registros esperados\n", header.TotalEsperado)
				return false
			}
		}

		// Tentar footer
		var footerMsg models.KafkaFooter
		if err := json.Unmarshal(message, &footerMsg); err == nil {
			if footerMsg.Lote == lote && footerMsg.TotalProcessado > 0 {
				footer = &footerMsg
				fmt.Printf("📋 Footer encontrado: %d registros processados\n", footer.TotalProcessado)
				return true // PARAR IMEDIATAMENTE
			}
		}

		// Tentar registro
		var registro models.Concurso
		if err := json.Unmarshal(message, &registro); err == nil {
			registros = append(registros, registro)

			// Validar status - se inválido, marca para rejeitar todo o lote
			if !registro.Status.Valid || (registro.Status.String != "aprovado" && registro.Status.String != "reprovado") {
				// Encontrar registro inválido - marcar para rejeitar todo o lote
				registrosValidos = nil  // Limpar registros válidos
				registrosComErro = true // Marcar que há erro no lote
			} else if !registrosComErro {
				// Só adiciona se não há erro no lote
				registrosValidos = append(registrosValidos, registro)
			}

			// Log de progresso a cada 1000 registros (otimizado)
			if len(registros)%1000 == 0 {
				fmt.Printf("  Consumidos: %d registros\n", len(registros))
			}

			// DEBUG: Log a cada 10K para ver se está progredindo
			if len(registros)%10000 == 0 {
				fmt.Printf("🔍 DEBUG: Processados %d registros, continuando...\n", len(registros))
			}
		}

		return false
	}

	// Consumir mensagens do tópico específico da data
	topicName := fmt.Sprintf("concurso_%s", data)
	if err := kafka.ConsumeMessages(topicName, handler); err != nil {
		return fmt.Errorf("erro ao consumir mensagens: %v", err)
	}

	// Validações
	if header == nil {
		fmt.Printf("❌ ERRO: Header não encontrado\n")
		// Salvar registros para análise
		if err := s.gerarLogLoteErro(data, lote, "Header não encontrado", registros); err != nil {
			fmt.Printf("⚠️  Erro ao gerar log de erro: %v\n", err)
		}

		// Enviar erro para tópico Kafka e coletar IDs
		var idsLinhaKafka []string
		for i, registro := range registros {
			idLinhaKafka := fmt.Sprintf("%s_%d", lote, i)
			idsLinhaKafka = append(idsLinhaKafka, idLinhaKafka)
			if err := s.enviarErroParaKafka(data, idLinhaKafka, registro, "Header não encontrado"); err != nil {
				fmt.Printf("⚠️  Erro ao enviar para Kafka: %v\n", err)
			}
		}

		// Salvar IDs das linhas Kafka
		if err := s.salvarIDsLinhaKafka(data, loteArquivo, idsLinhaKafka, "Header não encontrado"); err != nil {
			fmt.Printf("⚠️  Erro ao salvar IDs: %v\n", err)
		}
		return fmt.Errorf("header não encontrado")
	}
	if footer == nil {
		fmt.Printf("❌ ERRO: Footer não encontrado\n")
		// Salvar registros para análise
		if err := s.gerarLogLoteErro(data, lote, "Footer não encontrado", registros); err != nil {
			fmt.Printf("⚠️  Erro ao gerar log de erro: %v\n", err)
		}

		// Enviar erro para tópico Kafka e coletar IDs
		var idsLinhaKafka []string
		for i, registro := range registros {
			idLinhaKafka := fmt.Sprintf("%s_%d", lote, i)
			idsLinhaKafka = append(idsLinhaKafka, idLinhaKafka)
			if err := s.enviarErroParaKafka(data, idLinhaKafka, registro, "Footer não encontrado"); err != nil {
				fmt.Printf("⚠️  Erro ao enviar para Kafka: %v\n", err)
			}
		}

		// Salvar IDs das linhas Kafka
		if err := s.salvarIDsLinhaKafka(data, loteArquivo, idsLinhaKafka, "Footer não encontrado"); err != nil {
			fmt.Printf("⚠️  Erro ao salvar IDs: %v\n", err)
		}
		return fmt.Errorf("footer não encontrado")
	}
	if header.TotalEsperado != footer.TotalProcessado {
		fmt.Printf("❌ ERRO: Total esperado (%d) diferente do processado (%d)\n", header.TotalEsperado, footer.TotalProcessado)
		// Salvar registros para análise
		motivo := fmt.Sprintf("Total esperado (%d) diferente do processado (%d)", header.TotalEsperado, footer.TotalProcessado)
		if err := s.gerarLogLoteErro(data, lote, motivo, registros); err != nil {
			fmt.Printf("⚠️  Erro ao gerar log de erro: %v\n", err)
		}

		// Enviar erro para tópico Kafka e coletar IDs
		var idsLinhaKafka []string
		for i, registro := range registros {
			idLinhaKafka := fmt.Sprintf("%s_%d", lote, i)
			idsLinhaKafka = append(idsLinhaKafka, idLinhaKafka)
			if err := s.enviarErroParaKafka(data, idLinhaKafka, registro, motivo); err != nil {
				fmt.Printf("⚠️  Erro ao enviar para Kafka: %v\n", err)
			}
		}

		// Salvar IDs das linhas Kafka
		if err := s.salvarIDsLinhaKafka(data, loteArquivo, idsLinhaKafka, motivo); err != nil {
			fmt.Printf("⚠️  Erro ao salvar IDs: %v\n", err)
		}
		return fmt.Errorf("total esperado (%d) diferente do processado (%d)", header.TotalEsperado, footer.TotalProcessado)
	}

	// Inserir registros válidos
	if len(registrosValidos) > 0 {
		fmt.Printf("Inserindo %d registros válidos na tabela processada...\n", len(registrosValidos))

		// Preparar statement
		stmt, err := database.DB.Prepare(`
			INSERT INTO concurso_processado (nome, status, data_prova) VALUES (?, ?, ?)
		`)
		if err != nil {
			return fmt.Errorf("erro ao preparar statement: %v", err)
		}
		defer stmt.Close()

		// Inserir registros em batch para melhor performance
		totalInseridos := 0
		const BATCH_SIZE = 1000

		for i := 0; i < len(registrosValidos); i += BATCH_SIZE {
			end := i + BATCH_SIZE
			if end > len(registrosValidos) {
				end = len(registrosValidos)
			}

			// Construir batch insert
			var values []string
			var args []interface{}

			for j := i; j < end; j++ {
				registro := registrosValidos[j]
				values = append(values, "(?, ?, ?)")
				args = append(args, registro.Nome, registro.Status.String, registro.DataProva.Format("2006-01-02"))
			}

			// Executar batch insert
			query := fmt.Sprintf("INSERT INTO concurso_processado (nome, status, data_prova) VALUES %s", strings.Join(values, ","))
			_, err := database.DB.Exec(query, args...)
			if err != nil {
				return fmt.Errorf("erro ao inserir batch: %v", err)
			}

			totalInseridos += len(values)
			fmt.Printf("  Inseridos: %d/%d registros válidos (batch %d-%d)\n", totalInseridos, len(registrosValidos), i+1, end)
		}

		fmt.Printf("✅ Processamento concluído: %d registros válidos inseridos de %d total\n", len(registrosValidos), len(registros))

		// Tópico específico por data - não precisa limpar
		fmt.Printf("✅ Tópico Kafka %s mantido (sem conflitos)\n", topicName)

		// Gerar log de consumo (sucesso)
		tempoTotal := time.Since(inicio)
		if err := s.gerarLogConsumo(data, loteArquivo, len(registros), tempoTotal, "sucesso"); err != nil {
			fmt.Printf("⚠️  Erro ao gerar log de consumo: %v\n", err)
		}

		fmt.Printf("🎉 CONSUMO CONCLUÍDO COM SUCESSO! %d registros processados em %s\n", len(registrosValidos), s.formatarTempo(tempoTotal))
	} else {
		fmt.Printf("⚠️  Nenhum registro válido encontrado de %d total\n", len(registros))

		// Tópico específico por data - não precisa limpar
		fmt.Printf("✅ Tópico Kafka %s mantido (sem conflitos)\n", topicName)

		// Gerar log de consumo (sem registros válidos)
		tempoTotal := time.Since(inicio)
		if err := s.gerarLogConsumo(data, loteArquivo, len(registros), tempoTotal, "sem_registros_validos"); err != nil {
			fmt.Printf("⚠️  Erro ao gerar log de consumo: %v\n", err)
		}

		// Salvar TODOS os registros para análise posterior (incluindo os válidos)
		motivo := "Lote rejeitado - Status inválido encontrado (NULL ou diferente de aprovado/reprovado)"
		if err := s.gerarLogLoteErro(data, loteArquivo, motivo, registros); err != nil {
			fmt.Printf("⚠️  Erro ao gerar log de erro: %v\n", err)
		}

		// Enviar erro para tópico Kafka e coletar IDs
		var idsLinhaKafka []string
		for i, registro := range registros {
			idLinhaKafka := fmt.Sprintf("%s_%d", lote, i)
			idsLinhaKafka = append(idsLinhaKafka, idLinhaKafka)
			if err := s.enviarErroParaKafka(data, idLinhaKafka, registro, motivo); err != nil {
				fmt.Printf("⚠️  Erro ao enviar para Kafka: %v\n", err)
			}
		}

		// Salvar IDs das linhas Kafka
		if err := s.salvarIDsLinhaKafka(data, loteArquivo, idsLinhaKafka, motivo); err != nil {
			fmt.Printf("⚠️  Erro ao salvar IDs: %v\n", err)
		}

		fmt.Printf("❌ CONSUMO CONCLUÍDO COM FALHA! Nenhum registro válido encontrado em %s\n", s.formatarTempo(tempoTotal))
		fmt.Printf("📄 Registros com erro salvos para análise posterior\n")
	}

	return nil
}

// LimparTopicoKafka limpa o tópico Kafka
func (s *ConcursoService) LimparTopicoKafka() error {
	// Abordagem simples: usar um tópico com timestamp para evitar conflitos
	topicName := fmt.Sprintf("concurso_%d", time.Now().Unix())

	fmt.Printf("🔄 Usando tópico temporário: %s\n", topicName)

	// Criar tópico temporário
	createCmd := exec.Command("docker", "exec", "concurso_kafka", "kafka-topics",
		"--bootstrap-server", "localhost:9092",
		"--create",
		"--topic", topicName,
		"--partitions", "1",
		"--replication-factor", "1")

	if err := createCmd.Run(); err != nil {
		return fmt.Errorf("erro ao criar tópico temporário: %v", err)
	}

	// Deletar tópico antigo
	deleteCmd := exec.Command("docker", "exec", "concurso_kafka", "kafka-topics",
		"--bootstrap-server", "localhost:9092",
		"--delete",
		"--topic", "concurso")

	if err := deleteCmd.Run(); err != nil {
		fmt.Printf("⚠️  Erro ao deletar tópico antigo: %v\n", err)
	}

	// Aguardar um pouco
	time.Sleep(2 * time.Second)

	// Recriar tópico original
	recreateCmd := exec.Command("docker", "exec", "concurso_kafka", "kafka-topics",
		"--bootstrap-server", "localhost:9092",
		"--create",
		"--topic", "concurso",
		"--partitions", "1",
		"--replication-factor", "1")

	if err := recreateCmd.Run(); err != nil {
		return fmt.Errorf("erro ao recriar tópico original: %v", err)
	}

	// Deletar tópico temporário
	deleteTempCmd := exec.Command("docker", "exec", "concurso_kafka", "kafka-topics",
		"--bootstrap-server", "localhost:9092",
		"--delete",
		"--topic", topicName)

	if err := deleteTempCmd.Run(); err != nil {
		fmt.Printf("⚠️  Erro ao deletar tópico temporário: %v\n", err)
	}

	fmt.Printf("✅ Tópico Kafka limpo com sucesso\n")
	return nil
}

// limparTopicoKafkaAlternativo método alternativo para limpar Kafka
func (s *ConcursoService) limparTopicoKafkaAlternativo() error {
	// Resetar offset para o final (efetivamente "limpa" as mensagens antigas)
	fmt.Printf("🔄 Resetando offset do tópico...\n")

	// Criar um grupo de consumidor temporário e resetar offset
	resetCmd := exec.Command("docker", "exec", "concurso_kafka", "kafka-consumer-groups",
		"--bootstrap-server", "localhost:9092",
		"--group", "temp-cleanup-group",
		"--topic", "concurso",
		"--reset-offsets",
		"--to-latest",
		"--execute")

	if err := resetCmd.Run(); err != nil {
		fmt.Printf("⚠️  Erro ao resetar offset: %v\n", err)
	}

	// Consumir todas as mensagens antigas
	fmt.Printf("🧹 Consumindo mensagens antigas...\n")

	consumeCmd := exec.Command("docker", "exec", "concurso_kafka", "kafka-console-consumer",
		"--bootstrap-server", "localhost:9092",
		"--topic", "concurso",
		"--group", "temp-cleanup-group",
		"--from-beginning",
		"--max-messages", "10000",
		"--timeout-ms", "5000")

	if err := consumeCmd.Run(); err != nil {
		fmt.Printf("⚠️  Erro ao consumir mensagens: %v\n", err)
	}

	fmt.Printf("✅ Limpeza alternativa concluída\n")
	return nil
}

// formatarTempo formata duração para string legível
func (s *ConcursoService) formatarTempo(d time.Duration) string {
	if d < time.Second {
		return fmt.Sprintf("%dms", d.Milliseconds())
	}
	if d < time.Minute {
		return fmt.Sprintf("%.2fs", d.Seconds())
	}
	return d.String()
}

// extrairDataDoLote extrai a data do nome do lote
func (s *ConcursoService) extrairDataDoLote(lote string) string {
	if len(lote) >= 8 {
		data := lote[len(lote)-8:] // Pega os últimos 8 caracteres (YYYYMMDD)
		if len(data) == 8 {
			return fmt.Sprintf("%s-%s-%s", data[:4], data[4:6], data[6:8])
		}
	}
	return time.Now().Format("2006-01-02")
}

// gerarLogExtracao gera log de extração
func (s *ConcursoService) gerarLogExtracao(data string, lote string, total int, tempoTotal time.Duration) error {
	// Criar diretório se não existir
	logDir := filepath.Join("logs", data)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de log: %v", err)
	}

	logData := models.ExtracaoLog{
		Data:          data,
		Lote:          lote,
		TotalExtraido: total,
		TempoExecucao: s.formatarTempo(tempoTotal),
		Timestamp:     time.Now(),
	}

	jsonData, err := json.MarshalIndent(logData, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar log: %v", err)
	}

	filename := filepath.Join(logDir, fmt.Sprintf("extracao_%s.json", lote))
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("erro ao salvar log: %v", err)
	}

	fmt.Printf("📄 Log de extração salvo em: %s\n", filename)
	return nil
}

// gerarLogKafkaCarga gera log de carga no Kafka
func (s *ConcursoService) gerarLogKafkaCarga(header models.KafkaHeader, footer models.KafkaFooter, tempoTotal time.Duration) error {
	// Usar a data atual já que o lote agora tem timestamp
	data := time.Now().Format("2006-01-02")

	// Criar diretório se não existir
	logDir := filepath.Join("logs", data)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de log: %v", err)
	}

	logData := models.KafkaCargaLog{
		Header:     header,
		Footer:     footer,
		TempoEnvio: s.formatarTempo(tempoTotal),
		Timestamp:  time.Now(),
	}

	jsonData, err := json.MarshalIndent(logData, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar log: %v", err)
	}

	// Gerar nome de arquivo válido baseado no timestamp atual
	agora := time.Now()
	loteArquivo := fmt.Sprintf("concurso%s", agora.Format("02012006_150405"))
	filename := filepath.Join(logDir, fmt.Sprintf("kafka_carga_%s.json", loteArquivo))
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("erro ao salvar log: %v", err)
	}

	fmt.Printf("📄 Log de carga Kafka salvo em: %s\n", filename)
	return nil
}

// gerarLogConsumo gera log de consumo
func (s *ConcursoService) gerarLogConsumo(data string, lote string, totalConsumido int, tempoTotal time.Duration, status string) error {
	// Criar diretório se não existir
	logDir := filepath.Join("logs", data)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de log: %v", err)
	}

	logData := models.ConsumoLog{
		Data:               data,
		Lote:               lote,
		TotalConsumido:     totalConsumido,
		TempoProcessamento: s.formatarTempo(tempoTotal),
		Status:             status,
		Timestamp:          time.Now(),
	}

	jsonData, err := json.MarshalIndent(logData, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar log: %v", err)
	}

	filename := filepath.Join(logDir, fmt.Sprintf("consumo_%s.json", lote))
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("erro ao salvar log: %v", err)
	}

	fmt.Printf("📄 Log de consumo salvo em: %s\n", filename)
	return nil
}

// gerarLogLoteErro gera log de erro de lote
func (s *ConcursoService) gerarLogLoteErro(data string, lote string, motivo string, registrosComErro []models.Concurso) error {
	// Criar diretório se não existir
	logDir := filepath.Join("logs", data)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de log: %v", err)
	}

	// Calcular estatísticas
	totalRegistros := len(registrosComErro)
	registrosValidos := 0
	registrosInvalidos := 0

	for _, registro := range registrosComErro {
		if registro.Status.Valid && (registro.Status.String == "aprovado" || registro.Status.String == "reprovado") {
			registrosValidos++
		} else {
			registrosInvalidos++
		}
	}

	logData := models.LoteErroLog{
		Data:               data,
		Lote:               lote,
		Motivo:             motivo,
		TotalRegistros:     totalRegistros,
		RegistrosValidos:   registrosValidos,
		RegistrosInvalidos: registrosInvalidos,
		RegistrosComErro:   registrosComErro,
		Timestamp:          time.Now(),
	}

	jsonData, err := json.MarshalIndent(logData, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar log: %v", err)
	}

	filename := filepath.Join(logDir, fmt.Sprintf("lote_erro_%s.json", lote))
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("erro ao salvar log: %v", err)
	}

	fmt.Printf("📄 Log de erro salvo em: %s\n", filename)
	fmt.Printf("📊 Estatísticas: %d total, %d válidos, %d inválidos\n", totalRegistros, registrosValidos, registrosInvalidos)
	return nil
}

// gerarLogErroDetalhado gera log de erro com stack trace e payload
func (s *ConcursoService) gerarLogErroDetalhado(data string, categoria string, mensagem string, err error, payload interface{}) error {
	// Criar diretório se não existir
	logDir := filepath.Join("logs", data)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de log: %v", err)
	}

	// Capturar stack trace completo
	stackTrace := ""
	if err != nil {
		stackTrace = fmt.Sprintf("Error: %v\nStack: %s", err, debug.Stack())
	}

	// Capturar linha do erro
	linhaErro := "N/A"
	if err != nil {
		linhaErro = fmt.Sprintf("%T: %v", err, err)
	}

	logData := models.ErroLog{
		Categoria:  categoria,
		Mensagem:   mensagem,
		ErrorTrace: stackTrace,
		LinhaErro:  linhaErro,
		Payload:    payload,
		Timestamp:  time.Now(),
	}

	jsonData, err := json.MarshalIndent(logData, "", "  ")
	if err != nil {
		return fmt.Errorf("erro ao serializar log: %v", err)
	}

	filename := filepath.Join(logDir, fmt.Sprintf("erros_%s_%s.json", categoria, time.Now().Format("20060102_150405")))
	if err := os.WriteFile(filename, jsonData, 0644); err != nil {
		return fmt.Errorf("erro ao salvar log: %v", err)
	}

	fmt.Printf("📄 Log de erro detalhado salvo em: %s\n", filename)
	return nil
}

// enviarErroParaKafka envia erro para tópico de erros do Kafka
func (s *ConcursoService) enviarErroParaKafka(data string, idLinhaKafka string, payload interface{}, motivo string) error {
	// Inicializar produtor se necessário
	if kafka.Producer == nil {
		if err := kafka.InitProducer(); err != nil {
			return fmt.Errorf("erro ao inicializar produtor para erro: %v", err)
		}
	}

	erroKafka := models.ErroKafkaLog{
		IDLinhaKafka: idLinhaKafka,
		Payload:      payload,
		Motivo:       motivo,
		Timestamp:    time.Now(),
	}

	// Enviar para tópico de erros
	topicErros := "concurso_erros"
	if err := kafka.SendMessage(topicErros, erroKafka); err != nil {
		return fmt.Errorf("erro ao enviar erro para Kafka: %v", err)
	}

	fmt.Printf("📤 Erro enviado para tópico Kafka: %s (ID: %s)\n", topicErros, idLinhaKafka)
	return nil
}

// salvarIDsLinhaKafka salva IDs das linhas Kafka em arquivo texto
func (s *ConcursoService) salvarIDsLinhaKafka(data string, lote string, idsLinhaKafka []string, motivo string) error {
	// Criar diretório se não existir
	logDir := filepath.Join("logs", data)
	if err := os.MkdirAll(logDir, 0755); err != nil {
		return fmt.Errorf("erro ao criar diretório de log: %v", err)
	}

	filename := filepath.Join(logDir, fmt.Sprintf("ids_linha_kafka_%s.txt", lote))

	// Criar conteúdo do arquivo
	conteudo := fmt.Sprintf("=== IDs LINHA KAFKA - %s ===\n", time.Now().Format("2006-01-02 15:04:05"))
	conteudo += fmt.Sprintf("Data: %s\n", data)
	conteudo += fmt.Sprintf("Lote: %s\n", lote)
	conteudo += fmt.Sprintf("Motivo: %s\n", motivo)
	conteudo += fmt.Sprintf("Total de IDs: %d\n\n", len(idsLinhaKafka))

	for i, id := range idsLinhaKafka {
		conteudo += fmt.Sprintf("%d. %s\n", i+1, id)
	}

	if err := os.WriteFile(filename, []byte(conteudo), 0644); err != nil {
		return fmt.Errorf("erro ao salvar arquivo de IDs: %v", err)
	}

	fmt.Printf("📄 IDs das linhas Kafka salvos em: %s\n", filename)
	return nil
}
