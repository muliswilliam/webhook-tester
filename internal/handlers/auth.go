package handlers

import (
	"html/template"
	"log"
	"net/http"
	"os"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"

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

// AuthHandler handles registration and login
type AuthHandler struct {
	auth    service.AuthService
	metrics metrics.Recorder
	logger  *log.Logger
}

func NewAuthHandler(auth service.AuthService, l *log.Logger, m metrics.Recorder) *AuthHandler {
	return &AuthHandler{auth: auth, logger: l, metrics: m}
}

func (h *AuthHandler) RegisterGet(w http.ResponseWriter, r *http.Request) {
	data := RegisterPageData{
		CSRFField: csrf.TemplateField(r),
	}

	utils.RenderHtmlWithoutLayout(w, r, "register", data)
}

// helper to render the register page
func (h *AuthHandler) renderRegisterForm(w http.ResponseWriter, r *http.Request, data *RegisterPageData) {
	data.CSRFField = csrf.TemplateField(r)
	utils.RenderHtmlWithoutLayout(w, r, "register", data)
}

func (h *AuthHandler) RegisterPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusInternalServerError)
		return
	}

	fullName := r.FormValue("name")
	email := r.FormValue("email")
	password := r.FormValue("password")

	_, err := h.auth.Register(email, password, fullName)
	if err != nil {
		h.logger.Printf("error registering user: %v", err)
		h.renderRegisterForm(w, r, &RegisterPageData{
			Error:     err.Error(),
			FullName:  fullName,
			Email:     email,
			CSRFField: csrf.TemplateField(r),
		})
		return
	}

	h.metrics.IncSignUp()

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}

func (h *AuthHandler) LoginGet(w http.ResponseWriter, r *http.Request) {
	data := LoginPageData{
		CSRFField: csrf.TemplateField(r),
	}
	utils.RenderHtmlWithoutLayout(w, r, "login", data)
}

func (h *AuthHandler) LoginPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "unable to parse form", http.StatusInternalServerError)
		return
	}
	email := r.FormValue("email")
	password := r.FormValue("password")

	user, err := h.auth.Authenticate(email, password)
	if err != nil {
		// On failure, re-show login with a generic error
		h.logger.Printf("error authenticating user: %v", err)
		h.renderLoginForm(w, r, &LoginPageData{
			Error: "Invalid email or password",
		})
		return
	}

	err = h.auth.CreateSession(w, r, user.ID)
	if err != nil {
		h.logger.Printf("error creating session: %v", err)
		http.Error(w, "unable to save session", http.StatusInternalServerError)
	}

	// clear guest session
	h.auth.ClearSession(w, r, service.GuestSessionName)

	h.metrics.IncLogin()

	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// renderLoginForm is a small helper to DRY up template rendering
func (h *AuthHandler) renderLoginForm(w http.ResponseWriter, r *http.Request, data *LoginPageData) {
	data.CSRFField = csrf.TemplateField(r)
	utils.RenderHtmlWithoutLayout(w, r, "login", data)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.auth.ClearSession(w, r, service.SessionName)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) ForgotPasswordGet(w http.ResponseWriter, r *http.Request) {
	data := ForgotPasswordPageData{
		CSRFField: csrf.TemplateField(r),
	}
	utils.RenderHtmlWithoutLayout(w, r, "forgot-password", data)
}

// ForgotPasswordPost handles the form submission.
func (h *AuthHandler) ForgotPasswordPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "unable to parse form", http.StatusInternalServerError)
		return
	}

	email := r.FormValue("email")
	domain := os.Getenv("DOMAIN")

	link, err := h.auth.ForgotPassword(email, domain)
	data := ForgotPasswordPageData{CSRFField: csrf.TemplateField(r)}
	if err != nil {
		h.logger.Printf("Forgot password error: %v", err)
		// render without revealing details
		utils.RenderHtmlWithoutLayout(w, r, "forgot-password", data)
		return
	}
	// Log or email the link
	h.logger.Printf("Password reset link: %s", link)
	data.Success = true
	utils.RenderHtmlWithoutLayout(w, r, "forgot-password", data)
}

func (h *AuthHandler) renderResetForm(w http.ResponseWriter, r *http.Request, data *ResetPasswordPageData) {
	utils.RenderHtmlWithoutLayout(w, r, "reset-password", data)
}

// ResetPasswordGet renders the reset form if the token is valid.
func (h *AuthHandler) ResetPasswordGet(w http.ResponseWriter, r *http.Request) {
	token := r.URL.Query().Get("token")
	if token == "" {
		h.renderResetForm(w, r, &ResetPasswordPageData{
			Error:     "Missing token",
			CSRFField: csrf.TemplateField(r),
		})
		return
	}

	if _, err := h.auth.ValidateResetToken(token); err != nil {
		h.logger.Printf("invalid reset token: %v", err)
		h.renderResetForm(w, r, &ResetPasswordPageData{
			Error:     "Invalid or expired reset link",
			CSRFField: csrf.TemplateField(r),
		})
		return
	}

	// Token is good—show the form
	h.renderResetForm(w, r, &ResetPasswordPageData{
		Token:     token,
		CSRFField: csrf.TemplateField(r),
	})
}
func (h *AuthHandler) ResetPasswordPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "unable to parse form", http.StatusInternalServerError)
		return
	}

	token := r.FormValue("token")
	password := r.FormValue("password")
	confirmPassword := r.FormValue("confirm_password")

	data := &ResetPasswordPageData{
		Token:           token,
		Password:        password,
		ConfirmPassword: confirmPassword,
		CSRFField:       csrf.TemplateField(r),
	}

	if password != confirmPassword {
		data.Error = "Passwords do not match"
		h.renderResetForm(w, r, data)
		return
	}

	if err := h.auth.ResetPassword(token, password); err != nil {
		// webhookSvc returns “invalid or expired token” or other msgs
		data.Error = err.Error()
		h.renderResetForm(w, r, data)
		return
	}

	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
