package handlers

import (
	"html/template"
	"log"
	"net/http"
	"path/filepath"
	"strings"
	"webhook-tester/internal/db"
	"webhook-tester/internal/models"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/sessions"
)

func Register(w http.ResponseWriter, r *http.Request) {
	tmplRoot := filepath.Join("internal", "web", "templates")
	tmplPath := filepath.Join(tmplRoot, "register.html")
	templates := template.Must(template.ParseFiles(filepath.Join(tmplRoot, "base.html"), tmplPath))

	if r.Method == "GET" {
		err := templates.Execute(w, nil)
		if err != nil {
			http.Error(w, "template error", http.StatusInternalServerError)
		}
		return
	}

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
		templates.Execute(w, struct {
			Error    string
			FullName string
			Email    string
			Password string
		}{
			Error:    err.Error(),
			FullName: fullName,
			Email:    email,
			Password: password,
		})

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

	if err := sqlstore.InsertUser(&u); err != nil {
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			_ = templates.Execute(w, struct {
				Error    string
				FullName string
				Email    string
				Password string
			}{
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
