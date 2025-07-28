package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"

	"github.com/gorilla/mux"
	"github.com/joho/godotenv"

	"concurso-go-app/internal/database"
	"concurso-go-app/internal/services"
)

func main() {
	// Carregar variáveis de ambiente
	if err := godotenv.Load(); err != nil {
		log.Println("Arquivo .env não encontrado, usando variáveis do sistema")
	}

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

	// Configurar rotas
	r := mux.NewRouter()

	// Endpoint para popular dados
	r.HandleFunc("/start", startHandler).Methods("POST")

	// Endpoint para extrair registros e enviar para Kafka
	r.HandleFunc("/extrair/{data}", extrairHandler).Methods("POST")

	// Endpoint para consumir registros do Kafka
	r.HandleFunc("/consumir/{data}", consumirHandler).Methods("POST")

	// Endpoint para limpar tópico Kafka
	r.HandleFunc("/limpar", limparKafkaHandler).Methods("POST")

	// Iniciar servidor
	port := os.Getenv("API_PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Servidor iniciado na porta %s", port)
	log.Fatal(http.ListenAndServe(":"+port, r))
}

func startHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	service := services.NewConcursoService()

	// Popular dados (as tabelas são criadas automaticamente se não existirem)
	if err := service.PopularDados(); err != nil {
		http.Error(w, `{"erro": "Erro ao popular dados: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"mensagem": "Dados populados com sucesso (3100 registros)",
	}
	json.NewEncoder(w).Encode(response)
}

func extrairHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	data := vars["data"]

	// Validar formato da data
	if len(data) != 10 || data[4] != '-' || data[7] != '-' {
		http.Error(w, `{"erro": "Formato de data inválido. Use YYYY-MM-DD"}`, http.StatusBadRequest)
		return
	}

	service := services.NewConcursoService()

	if err := service.ExtrairRegistros(data); err != nil {
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
	w.Header().Set("Content-Type", "application/json")

	vars := mux.Vars(r)
	data := vars["data"]

	// Validar formato da data
	if len(data) != 10 || data[4] != '-' || data[7] != '-' {
		http.Error(w, `{"erro": "Formato de data inválido. Use YYYY-MM-DD"}`, http.StatusBadRequest)
		return
	}

	service := services.NewConcursoService()

	if err := service.ConsumirRegistros(data); err != nil {
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
	w.Header().Set("Content-Type", "application/json")

	// Usar o service para limpar (que tem a lógica melhorada)
	service := services.NewConcursoService()

	if err := service.LimparTopicoKafka(); err != nil {
		http.Error(w, `{"erro": "Erro ao limpar Kafka: `+err.Error()+`"}`, http.StatusInternalServerError)
		return
	}

	response := map[string]string{
		"mensagem": "Tópico Kafka limpo com sucesso",
	}
	json.NewEncoder(w).Encode(response)
}
