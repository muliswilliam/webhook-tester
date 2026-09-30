package handlers

import (
	"errors"
	"html/template"
	"log"
	"net/http"
	"net/url"
	"os"
	"strings"
	"webhook-tester/internal/mailer"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"
	"webhook-tester/internal/web/view"

	"github.com/gorilla/csrf"
	"gorm.io/gorm"
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
	Email     string
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
	auth       *service.AuthService
	webhookSvc *service.WebhookService
	mailer     mailer.Mailer
	metrics    metrics.Recorder
	logger     *log.Logger
}

func NewAuthHandler(
	auth *service.AuthService,
	webhookSvc *service.WebhookService,
	m mailer.Mailer,
	l *log.Logger,
	mr metrics.Recorder,
) *AuthHandler {
	return &AuthHandler{auth: auth, webhookSvc: webhookSvc, mailer: m, logger: l, metrics: mr}
}

// claimGuestWorkspace moves the guest workspace named by the request's guest
// cookie, if any, into userID's account and clears the cookie. It returns the
// claimed webhook's ID, or "" if there was nothing to claim.
func (h *AuthHandler) claimGuestWorkspace(w http.ResponseWriter, r *http.Request, userID uint) string {
	c, err := r.Cookie(sessionIdName)
	if err != nil {
		return ""
	}
	c.MaxAge = -1
	c.Path = "/"
	http.SetCookie(w, c)

	if err := h.webhookSvc.ClaimGuestWebhook(c.Value, userID); err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			h.logger.Printf("error claiming guest webhook %s for user %d: %v", c.Value, userID, err)
		}
		return ""
	}
	return c.Value
}

func (h *AuthHandler) RegisterGet(w http.ResponseWriter, r *http.Request) {
	data := RegisterPageData{
		CSRFField: csrf.TemplateField(r),
	}

	view.RenderHTMLWithoutLayout(w, r, "register", data)
}

// helper to render the register page
func (h *AuthHandler) renderRegisterForm(w http.ResponseWriter, r *http.Request, data *RegisterPageData) {
	data.CSRFField = csrf.TemplateField(r)
	view.RenderHTMLWithoutLayout(w, r, "register", data)
}

func (h *AuthHandler) RegisterPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "failed to parse form", http.StatusInternalServerError)
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
	if err := utils.ValidatePassword(password, rules); err != nil {
		// re-render with error and preserve inputs
		h.renderRegisterForm(w, r, &RegisterPageData{
			Error:     err.Error(),
			FullName:  fullName,
			Email:     email,
			CSRFField: csrf.TemplateField(r),
		})
		return
	}

	user, err := h.auth.Register(email, password, fullName)
	if err != nil {
		// Duplicate email?
		if strings.Contains(err.Error(), "email already taken") {
			h.renderRegisterForm(w, r, &RegisterPageData{
				Error:     "That email is already registered",
				FullName:  fullName,
				Email:     email,
				CSRFField: csrf.TemplateField(r),
			})
			return
		}
		// Otherwise: unexpected
		h.logger.Printf("error registering user: %v", err)
		http.Error(w, "internal error", http.StatusInternalServerError)
		return
	}

	h.metrics.IncSignUp()

	if err := h.auth.CreateSession(w, r, user); err != nil {
		// The account exists, so let the user sign in by hand.
		h.logger.Printf("error creating session after registration: %v", err)
		utils.SetFlashSuccess(w, "Account created. Sign in to continue.")
		http.Redirect(w, r, "/login", http.StatusSeeOther)
		return
	}

	if id := h.claimGuestWorkspace(w, r, user.ID); id != "" {
		utils.SetFlashSuccess(w, "Account created. Your endpoint and its requests are now saved to your account.")
		http.Redirect(w, r, "/?address="+url.QueryEscape(id), http.StatusSeeOther)
		return
	}
	utils.SetFlashSuccess(w, "Account created. Create your first endpoint to start capturing requests.")
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) LoginGet(w http.ResponseWriter, r *http.Request) {
	data := LoginPageData{
		CSRFField: csrf.TemplateField(r),
	}
	view.RenderHTMLWithoutLayout(w, r, "login", data)
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
			Email: email,
		})
		return
	}

	err = h.auth.CreateSession(w, r, user)
	if err != nil {
		h.logger.Printf("error creating session: %v", err)
		http.Error(w, "unable to save session", http.StatusInternalServerError)
		return
	}

	h.metrics.IncLogin()

	if id := h.claimGuestWorkspace(w, r, user.ID); id != "" {
		utils.SetFlashSuccess(w, "Signed in. Your guest endpoint is now saved to your account.")
		http.Redirect(w, r, "/?address="+url.QueryEscape(id), http.StatusSeeOther)
		return
	}
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

// renderLoginForm is a small helper to DRY up template rendering
func (h *AuthHandler) renderLoginForm(w http.ResponseWriter, r *http.Request, data *LoginPageData) {
	data.CSRFField = csrf.TemplateField(r)
	view.RenderHTMLWithoutLayout(w, r, "login", data)
}

func (h *AuthHandler) Logout(w http.ResponseWriter, r *http.Request) {
	h.auth.ClearSession(w, r)
	http.Redirect(w, r, "/", http.StatusSeeOther)
}

func (h *AuthHandler) ForgotPasswordGet(w http.ResponseWriter, r *http.Request) {
	data := ForgotPasswordPageData{
		CSRFField: csrf.TemplateField(r),
	}
	view.RenderHTMLWithoutLayout(w, r, "forgot-password", data)
}

// ForgotPasswordPost handles the form submission.
func (h *AuthHandler) ForgotPasswordPost(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "unable to parse form", http.StatusInternalServerError)
		return
	}

	email := strings.TrimSpace(r.FormValue("email"))
	domain := os.Getenv("DOMAIN")

	// Send in the background and answer the same way whether or not the
	// account exists, so neither the response nor its timing reveals which
	// emails are registered.
	go h.sendPasswordReset(email, domain)

	view.RenderHTMLWithoutLayout(w, r, "forgot-password", ForgotPasswordPageData{
		CSRFField: csrf.TemplateField(r),
		Success:   true,
	})
}

func (h *AuthHandler) sendPasswordReset(email, domain string) {
	link, err := h.auth.ForgotPassword(email, domain)
	if err != nil {
		h.logger.Printf("forgot password for %q: %v", email, err)
		return
	}
	body := "Hi,\n\n" +
		"Someone asked to reset the password for your Webhook Tester account. " +
		"Open this link to choose a new password:\n\n" +
		link + "\n\n" +
		"The link expires in 24 hours. If you didn't ask for this, you can ignore this email - " +
		"your password won't change.\n"
	if err := h.mailer.Send(email, "Reset your Webhook Tester password", body); err != nil {
		h.logger.Printf("error sending password reset email: %v", err)
	}
}

func (h *AuthHandler) renderResetForm(w http.ResponseWriter, r *http.Request, data *ResetPasswordPageData) {
	view.RenderHTMLWithoutLayout(w, r, "reset-password", data)
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

	// Token is good - show the form
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

	utils.SetFlashSuccess(w, "Password updated. Sign in with your new password.")
	http.Redirect(w, r, "/login", http.StatusSeeOther)
}
