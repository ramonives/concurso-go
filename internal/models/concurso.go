package models

import (
	"database/sql"
	"time"
)

type Concurso struct {
	ID        int            `json:"id"`
	Nome      string         `json:"nome"`
	Status    sql.NullString `json:"status"`
	DataProva time.Time      `json:"data_prova"`
}
