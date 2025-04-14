package handlers

import (
	"encoding/json"
	"net/http"
	"strings"
	"webhook-tester/internal/models"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"
)

func CreateUser(w http.ResponseWriter, r *http.Request) {
	var input struct {
		FullName string `json:"full_name"`
		Email    string `json:"email"`
		Password string `json:"password"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.RenderJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	passwordHash, err := utils.HashPassword(input.Password)
	if err != nil {
		utils.RenderJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	u := models.User{
		FullName: input.FullName,
		Email:    input.Email,
		Password: passwordHash,
		APIKey:   utils.GenerateApiKey(),
	}

	if err := sqlstore.InsertUser(&u); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			utils.RenderJSON(w, http.StatusBadRequest, map[string]string{
				"error": "email already exists",
			})
		} else {
			utils.RenderJSON(w, http.StatusInternalServerError, err.Error())
		}
		return
	}

	utils.RenderJSON(w, http.StatusCreated, map[string]string{
		"full_name": u.FullName,
		"email":     u.Email,
		"api_key":   u.APIKey,
	})
}
