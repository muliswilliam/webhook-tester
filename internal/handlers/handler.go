package handlers

import (
	"github.com/wader/gormstore/v2"
	"gorm.io/gorm"
)

type Handler struct {
	DB           *gorm.DB
	SessionStore *gormstore.Store
}
