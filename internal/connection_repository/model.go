package connection_repository

import (
	"time"
)

type Connection struct {
	ID             uint `gorm:"primary_key"`
	Account        string
	ClientID       string
	Dispatchers    string
	CanonicalFacts string
	Tags           string
	CreatedAt      time.Time
	UpdatedAt      time.Time
	StaleTimestamp time.Time
}
