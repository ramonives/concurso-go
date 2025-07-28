package models

type KafkaHeader struct {
	Lote          string `json:"lote"`
	TotalEsperado int    `json:"total_esperado"`
	InicioEnvio   string `json:"inicio_envio"`
}

type KafkaFooter struct {
	Lote            string `json:"lote"`
	TotalProcessado int    `json:"total_processado"`
	FimEnvio        string `json:"fim_envio"`
}
