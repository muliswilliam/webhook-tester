package handlers

import (
	"github.com/wader/gormstore/v2"
	"gorm.io/gorm"
	"log"
)

type Handler struct {
	DB           *gorm.DB
	SessionStore *gormstore.Store
	Logger       *log.Logger
}
