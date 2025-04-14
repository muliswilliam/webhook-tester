package models

import "gorm.io/gorm"

type User struct {
	gorm.Model
	FullName string `json:"full_name"`
	Email    string `json:"email" gorm:"type:varchar(255);unique"`
	Password string
	APIKey   string `json:"api_key"`
}
