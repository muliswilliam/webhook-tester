package models

import (
	"gorm.io/gorm"
	"time"
)

type User struct {
	gorm.Model
	FullName         string    `json:"full_name"`
	Email            string    `json:"email" gorm:"type:varchar(255);unique"`
	Password         string    `json:"-"`
	APIKey           string    `json:"-"`
	ResetToken       string    `json:"-"`
	ResetTokenExpiry time.Time `json:"-"`
}
