package handlers

import (
	"github.com/gorilla/csrf"
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"webhook-tester/internal/models"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/sessions"
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

func parseTemplates(tmplPath string) *template.Template {
	tmplRoot := filepath.Join("internal", "web", "templates")
	templates := template.Must(
		template.ParseFiles(
			filepath.Join(tmplRoot, "base.html"),
			filepath.Join(tmplRoot, tmplPath),
		),
	)

	return templates
}

func handleTemplateErr(err error, w http.ResponseWriter) {
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *Handler) RegisterGet(w http.ResponseWriter, r *http.Request) {
	templates := parseTemplates("register.html")
	data := RegisterPageData{
		CSRFField: csrf.TemplateField(r),
	}
	err := templates.Execute(w, data)
	if err != nil {
		http.Error(w, "template error", http.StatusInternalServerError)
	}
}

func (h *Handler) RegisterPost(w http.ResponseWriter, r *http.Request) {
	templates := parseTemplates("register.html")
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
		err := templates.Execute(w, data)
		handleTemplateErr(err, w)
		return
	}

	passwordHash, err := utils.HashPassword(password)
	if err != nil {
		http.Error(w, "hashing error", http.StatusInternalServerError)
		return
	}

	u := models.User{
		FullName: fullName,
		Email:    email,
		Password: passwordHash,
		APIKey:   utils.GenerateApiKey(),
	}

	if err := sqlstore.InsertUser(h.DB, &u); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			_ = templates.Execute(w, RegisterPageData{
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

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *Handler) LoginGet(w http.ResponseWriter, r *http.Request) {
	templates := parseTemplates("login.html")
	data := LoginPageData{
		CSRFField: csrf.TemplateField(r),
	}
	err := templates.Execute(w, data)
	handleTemplateErr(err, w)
	return
}

func (h *Handler) LoginPost(w http.ResponseWriter, r *http.Request) {
	templates := parseTemplates("login.html")
	email := r.FormValue("email")
	password := r.FormValue("password")

	var user models.User
	err := h.DB.First(&user, "email = ?", email).Error

	if err != nil {
		data := LoginPageData{
			CSRFField: csrf.TemplateField(r),
			Error:     "Invalid username / password",
		}
		err = templates.Execute(w, data)
		handleTemplateErr(err, w)
		return
	}

	if !utils.CheckPasswordHash(password, user.Password) {
		data := LoginPageData{
			Error:     "Invalid username / password",
			CSRFField: csrf.TemplateField(r),
		}
		err = templates.Execute(w, data)
		handleTemplateErr(err, w)
		return
	}

	session, err := h.SessionStore.Get(r, sessions.Name)
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
