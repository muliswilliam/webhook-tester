package handlers

import (
	"log"
	"webhook-tester/internal/metrics"

	"github.com/wader/gormstore/v2"
	"gorm.io/gorm"
)

type Handler struct {
	DB           *gorm.DB
	SessionStore *gormstore.Store
	Logger       *log.Logger
	Metrics      metrics.Recorder
}
