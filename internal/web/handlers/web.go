package handlers

import (
	"html/template"
	"path/filepath"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web"

	"github.com/go-chi/chi/v5"
	"gorm.io/gorm"

	"log"
	"net/http"
	"os"
	"time"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
)

var sessionIdName = "_webhook_tester_guest_session_id"

func Home(w http.ResponseWriter, r *http.Request) {
	cookie, err := r.Cookie(sessionIdName) // or login cookie

	if err != nil {
		log.Printf("Cookie err: %v", err)

		// create a default webhook
		defaultWh := models.Webhook{
			ID:            utils.GenerateID(),
			Title:         "Default Webhook",
			ResponseCode:  http.StatusOK,
			ResponseDelay: 0,
		}
		err := sqlstore.InsertWebhook(defaultWh)
		if err != nil {
			log.Printf("Error inserting default webhook: %v", err)
		}

		cookie = &http.Cookie{
			Name:     sessionIdName,
			Value:    defaultWh.ID,
			Path:     "/",
			HttpOnly: true,
			Secure:   false,     // Set to true in production with HTTPS
			MaxAge:   86400 * 2, // 2 days
		}
		http.SetCookie(w, cookie)
	}

	var webhookId string
	address := r.URL.Query().Get("address")
	if address == "" {
		webhookId = cookie.Value
	} else {
		webhookId = address
	}

	var webhook models.Webhook
	err = db.DB.Model(&models.Webhook{}).Preload("Requests", func(db *gorm.DB) *gorm.DB {
		db = db.Order("received_at DESC")
		return db
	}).Find(&webhook, "id = ?", webhookId).Error

	if err != nil {
		log.Printf("failed to get webhook: %v", err)
		http.Error(w, "failed to get webhook", http.StatusInternalServerError)
	}

	data := struct {
		User    models.User
		Webhook models.Webhook
		Domain  string
		Year    int
	}{
		User:    web.GetLoggedInUser(r),
		Webhook: webhook,
		Domain:  os.Getenv("DOMAIN"),
		Year:    time.Now().Year(),
	}

	web.Render(w, "home", data)
}

func Request(w http.ResponseWriter, r *http.Request) {
	requestId := chi.URLParam(r, "id")
	address := r.URL.Query().Get("address")
	var webhook models.Webhook
	err := db.DB.Model(&models.Webhook{}).Preload("Requests", func(db *gorm.DB) *gorm.DB {
		db = db.Order("received_at DESC")
		return db
	}).Find(&webhook, "id = ?", address).Error

	if err != nil {
		log.Printf("failed to get webhook request: %v", err)
		http.Error(w, "failed to get webhook request", http.StatusInternalServerError)
	}

	var request models.WebhookRequest

	for _, r := range webhook.Requests {
		if r.ID == requestId {
			request = r
		}
	}

	data := struct {
		ID      string
		Year    int
		User    models.User
		Webhook models.Webhook
		Request models.WebhookRequest
	}{
		ID:      requestId,
		Webhook: webhook,
		User:    web.GetLoggedInUser(r),
		Request: request,
		Year:    time.Now().Year(),
	}

	web.Render(w, "request", data)
}

func Login(w http.ResponseWriter, r *http.Request) {
	tmplRoot := filepath.Join("internal", "web", "templates")
	tmplPath := filepath.Join(tmplRoot, "login.html")
	templates := template.Must(template.ParseFiles(filepath.Join(tmplRoot, "base.html"), tmplPath))

	if r.Method == "GET" {
		err := templates.Execute(w, nil)
		if err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
		return
	}

	email := r.FormValue("email")
	password := r.FormValue("password")

	var user models.User
	err := db.DB.First(&user, "email = ?", email).Error

	if err != nil {
		data := struct {
			Error string
		}{
			Error: "Invalid username / passowrd",
		}
		templates.Execute(w, data)
	}

	if !utils.CheckPasswordHash(password, user.Password) {
		data := struct {
			Error string
		}{
			Error: "Invalid username / passowrd",
		}
		templates.Execute(w, data)
	}

	session, err := web.SessionStore.Get(r, web.SessionName)
	session.Values["user_id"] = user.ID
	session.Values["email"] = user.Email
	session.Values["full_name"] = user.FullName
	web.SessionStore.Save(r, w, session)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func Logout(w http.ResponseWriter, r *http.Request) {
	session, err := web.SessionStore.Get(r, web.SessionName)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	session.Options.MaxAge = -1
	_ = session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
