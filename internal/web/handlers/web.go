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

func createDefaultWebhookCookie(w http.ResponseWriter) *http.Cookie {
	defaultWh := models.Webhook{
		ID:            utils.GenerateID(),
		Title:         "Default Webhook",
		ResponseCode:  http.StatusOK,
		ResponseDelay: 0,
	}
	if err := sqlstore.InsertWebhook(defaultWh); err != nil {
		log.Printf("Error inserting default webhook: %v", err)
	}

	cookie := &http.Cookie{
		Name:     sessionIdName,
		Value:    defaultWh.ID,
		Path:     "/",
		HttpOnly: true,
		Secure:   false,     // Set to true in production
		MaxAge:   86400 * 2, // 2 days
	}
	http.SetCookie(w, cookie)
	return cookie
}

func fetchWebhookWithRequests(id string) (models.Webhook, error) {
	var webhook models.Webhook
	err := db.DB.Preload("Requests", func(db *gorm.DB) *gorm.DB {
		return db.Order("received_at DESC")
	}).First(&webhook, "id = ?", id).Error
	return webhook, err
}

func fetchUserWebhooks(userID interface{}) []models.Webhook {
	var webhooks []models.Webhook
	err := db.DB.Preload("Requests", func(db *gorm.DB) *gorm.DB {
		return db.Order("received_at DESC").Limit(10)
	}).
		Where("user_id = ?", userID).Find(&webhooks).
		Order("created_at DESC").Error

	if err != nil {
		log.Printf("Error loading user webhooks: %v", err)
	}
	return webhooks
}

func Home(w http.ResponseWriter, r *http.Request) {
	userID, err := web.Authorize(r)
	if err != nil {
		log.Println("user not logged in")
	}

	// Get or create default webhook via cookie
	cookie, err := r.Cookie(sessionIdName)
	if err != nil && userID == 0 {
		log.Printf("Cookie err: %v", err)
		cookie = createDefaultWebhookCookie(w)
	}
	var webhooks []models.Webhook
	var webhook models.Webhook
	var activeWebhook models.Webhook

	// Determine active webhook ID
	address := r.URL.Query().Get("address")
	var webhookID = address
	if webhookID == "" && cookie != nil {
		webhookID = cookie.Value
	}

	if webhookID != "" && userID == 0 {
		webhook, err = fetchWebhookWithRequests(webhookID)
		if err != nil {
			log.Printf("failed to get webhook: %v", err)
			http.Error(w, "failed to get webhook", http.StatusInternalServerError)
			return
		}
		webhooks = append(webhooks, webhook)
	} else {
		// Load user's other webhooks if logged in
		webhooks = fetchUserWebhooks(userID)
	}

	if address != "" {
		activeWebhook, err = fetchWebhookWithRequests(address)
		if err != nil {
			log.Printf("failed to get webhook: %v", err)
		}
	} else {
		activeWebhook = webhooks[0]
	}

	// Render the home page
	data := struct {
		User          models.User
		Webhooks      []models.Webhook
		Webhook       models.Webhook
		RequestsCount uint
		Domain        string
		Year          int
	}{
		User:          web.GetLoggedInUser(r),
		Webhooks:      webhooks,
		Webhook:       activeWebhook,
		RequestsCount: uint(len(activeWebhook.Requests)),
		Domain:        os.Getenv("DOMAIN"),
		Year:          time.Now().Year(),
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
		ID       string
		Year     int
		User     models.User
		Webhooks []models.Webhook
		Webhook  models.Webhook
		Request  models.WebhookRequest
	}{
		ID:       requestId,
		Webhooks: []models.Webhook{webhook},
		Webhook:  webhook,
		User:     web.GetLoggedInUser(r),
		Request:  request,
		Year:     time.Now().Year(),
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
			Error: "Invalid username / password",
		}
		err = templates.Execute(w, data)
		if err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	}

	if !utils.CheckPasswordHash(password, user.Password) {
		data := struct {
			Error string
		}{
			Error: "Invalid username / password",
		}
		err = templates.Execute(w, data)
		if err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
	}

	session, err := web.SessionStore.Get(r, web.SessionName)
	session.Values["user_id"] = user.ID
	session.Values["email"] = user.Email
	session.Values["full_name"] = user.FullName
	err = web.SessionStore.Save(r, w, session)
	if err != nil {
		log.Printf("failed to save session: %v", err)
		http.Error(w, "failed to save session", http.StatusInternalServerError)
	}

	// remove guest session
	cookie, err := r.Cookie(sessionIdName)
	if err != nil {
		log.Printf("Cookie err: %v", err)
	}
	cookie.MaxAge = -1
	http.SetCookie(w, cookie)

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
