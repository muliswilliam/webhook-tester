package handlers

import (
	"fmt"
	"html/template"
	"log"
	"net/http"
	"os"
	"strings"
	"time"
	"webhook-tester/internal/models"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/sessions"

	"github.com/gorilla/csrf"
)

type RegisterPageData struct {
	CSRFField template.HTML
	Error     string
	FullName  string
	Email     string
	Password  string
}

type LoginPageData struct {
	CSRFField template.HTML
	Error     string
}

type ForgotPasswordPageData struct {
	CSRFField template.HTML
	Error     string
	Success   bool
}

type ResetPasswordPageData struct {
	CSRFField       template.HTML
	Error           string
	Token           string
	Password        string
	ConfirmPassword string
}

func (h *Handler) RegisterGet(w http.ResponseWriter, r *http.Request) {
	data := RegisterPageData{
		CSRFField: csrf.TemplateField(r),
	}

	utils.RenderHtmlWithoutLayout(w, r, "register", data)
}

func (h *Handler) RegisterPost(w http.ResponseWriter, r *http.Request) {
	err := r.ParseForm()
	if err != nil {
		http.Error(w, "parse error", http.StatusInternalServerError)
		return
	}

	fullName := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")

	rules := utils.PasswordRules{
		MinLength:        8,
		RequireLowercase: true,
		RequireUppercase: true,
		RequireNumber:    true,
	}

	err = utils.ValidatePassword(password, rules)
	if err != nil {
		data := RegisterPageData{
			Error:     err.Error(),
			FullName:  fullName,
			Email:     email,
			Password:  password,
			CSRFField: csrf.TemplateField(r),
		}
		utils.RenderHtmlWithoutLayout(w, r, "register", data)
		return
	}

	passwordHash, err := utils.HashPassword(password)
	if err != nil {
		http.Error(w, "hashing error", http.StatusInternalServerError)
		return
	}

	key, err := utils.GenerateAPIKey("user_", 32)
	if err != nil {
		http.Error(w, "api key error", http.StatusInternalServerError)
		return
	}

	u := models.User{
		FullName: fullName,
		Email:    email,
		Password: passwordHash,
		APIKey:   key,
	}

	if err := sqlstore.InsertUser(h.DB, &u); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			utils.RenderHtmlWithoutLayout(w, r, "register", RegisterPageData{
				Error:    "Email already in use",
				FullName: fullName,
				Email:    email,
				Password: password,
			})
		} else {
			log.Printf("Error inserting user: %v", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
		}
		return
	}

	h.Metrics.IncSignUp()

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) LoginGet(w http.ResponseWriter, r *http.Request) {
	data := LoginPageData{
		CSRFField: csrf.TemplateField(r),
	}
	utils.RenderHtmlWithoutLayout(w, r, "login", data)
}

func (h *Handler) LoginPost(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	password := r.FormValue("password")

	var user models.User
	err := h.DB.First(&user, "email = ?", email).Error
	if err != nil {
		data := LoginPageData{
			CSRFField: csrf.TemplateField(r),
			Error:     "Invalid username / password",
		}
		utils.RenderHtmlWithoutLayout(w, r, "login", data)
		return
	}

	if !utils.CheckPasswordHash(password, user.Password) {
		data := LoginPageData{
			Error:     "Invalid username / password",
			CSRFField: csrf.TemplateField(r),
		}
		utils.RenderHtmlWithoutLayout(w, r, "login", data)
		return
	}

	session, err := h.SessionStore.Get(r, sessions.Name)
	if err != nil {

	}
	session.Values["user_id"] = user.ID
	session.Values["email"] = user.Email
	session.Values["full_name"] = user.FullName
	err = h.SessionStore.Save(r, w, session)
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

	h.Metrics.IncLogin()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) Logout(w http.ResponseWriter, r *http.Request) {
	session, err := h.SessionStore.Get(r, sessions.Name)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	session.Options.MaxAge = -1
	_ = session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *Handler) ForgotPasswordGet(w http.ResponseWriter, r *http.Request) {
	data := ForgotPasswordPageData{
		CSRFField: csrf.TemplateField(r),
	}
	utils.RenderHtmlWithoutLayout(w, r, "forgot-password", data)
}

func (h *Handler) ForgotPasswordPost(w http.ResponseWriter, r *http.Request) {
	email := r.FormValue("email")
	user := models.User{}
	data := ForgotPasswordPageData{
		CSRFField: csrf.TemplateField(r),
	}
	err := h.DB.First(&user, "email = ?", email).Error
	if err != nil {
		h.Logger.Printf("Error getting user: %v", err)
		utils.RenderHtmlWithoutLayout(w, r, "forgot-password", data)
		return
	}
	token, err := utils.GenerateSecureToken(32) // 32 byte = 64 hex chars
	if err != nil {
		log.Printf("failed to generate reset token: %v", err)
		http.Error(w, "Something went wrong", http.StatusInternalServerError)
		return
	}

	user.ResetToken = token
	user.ResetTokenExpiry = time.Now().Add(time.Hour * 24) // expires in 1 day
	h.DB.Save(&user)

	// (For now) Log the reset link instead of sending email
	resetLink := fmt.Sprintf("%s/reset-password?token=%s", os.Getenv("DOMAIN"), token)
	log.Printf("Password reset link for %s: %s", user.Email, resetLink)
	data.Success = true
	utils.RenderHtmlWithoutLayout(w, r, "forgot-password", data)
}

func (h *Handler) ResetPasswordGet(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		utils.RenderHtmlWithoutLayout(w, r, "reset-password", map[string]interface{}{
			"Error": "Missing token",
		})
		return
	}

	user := models.User{}
	err := h.DB.First(&user, "reset_token = ?", token).Error
	if err != nil || time.Now().After(user.ResetTokenExpiry) {
		h.Logger.Printf("Error getting user: %v", err)
		utils.RenderHtmlWithoutLayout(w, r, "reset-password", map[string]interface{}{
			"Error":     "Invalid or expired reset link",
			"CSRFField": csrf.TemplateField(r),
		})
		return
	}

	utils.RenderHtmlWithoutLayout(w, r, "reset-password", map[string]interface{}{
		"CSRFField": csrf.TemplateField(r),
		"Token":     token,
	})
}

func (h *Handler) ResetPasswordPost(w http.ResponseWriter, r *http.Request) {
	token := r.FormValue("token")
	password := r.FormValue("password")
	confirm := r.FormValue("confirm_password")

	data := ResetPasswordPageData{
		CSRFField:       csrf.TemplateField(r),
		Password:        password,
		ConfirmPassword: confirm,
	}

	if password != confirm {
		data.Error = "Passwords do not match"
		utils.RenderHtmlWithoutLayout(w, r, "reset-password", data)
		return
	}

	rules := utils.PasswordRules{
		MinLength:        8,
		RequireLowercase: true,
		RequireUppercase: true,
		RequireNumber:    true,
	}

	err := utils.ValidatePassword(password, rules)
	if err != nil {
		data.Error = err.Error()
		utils.RenderHtmlWithoutLayout(w, r, "reset-password", data)
	}

	var user models.User
	err = h.DB.First(&user, "reset_token = ?", token).Error
	if err != nil || time.Now().After(user.ResetTokenExpiry) {
		data.Error = "Invalid or expired reset link"
		utils.RenderHtmlWithoutLayout(w, r, "reset-password", data)
		return
	}

	// Update password
	hashedPassword, _ := utils.HashPassword(password)
	user.Password = hashedPassword
	user.ResetToken = ""
	user.ResetTokenExpiry = time.Time{}

	h.DB.Save(&user)
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
