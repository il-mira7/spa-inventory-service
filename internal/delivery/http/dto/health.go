package dto

// HealthResponse представляет ответ статуса работоспособности сервиса
type HealthResponse struct {
	Status    string `json:"status"`
	Timestamp string `json:"timestamp"`
	Database  string `json:"database,omitempty"`
	Version   string `json:"version,omitempty"`
}
