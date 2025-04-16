package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/sessions"
)

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

	session, err := sessions.Store.Get(r, sessions.Name)
	session.Values["user_id"] = user.ID
	session.Values["email"] = user.Email
	session.Values["full_name"] = user.FullName
	err = sessions.Store.Save(r, w, session)
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
	session, err := sessions.Store.Get(r, sessions.Name)
	if err != nil {
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	session.Options.MaxAge = -1
	_ = session.Save(r, w)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}
