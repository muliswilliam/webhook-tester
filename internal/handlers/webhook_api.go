package handlers

import (
	"encoding/json"
	"errors"
	"gorm.io/gorm"
	"net/http"
	"time"
	"webhook-tester/internal/middlewares"
	"webhook-tester/internal/models"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"

	"github.com/go-chi/chi/v5"
)

// CreateWebhookRequest
// swagger:request
type CreateWebhookRequest struct {
	// Title of the webhook
	// required: true
	Title         string `json:"title"`
	ResponseCode  int    `json:"response_code"`
	ResponseDelay uint   `json:"response_delay"` // milliseconds
	ContentType   string `json:"content_type"`
	Payload       string `json:"payload"`
	NotifyOnEvent bool   `json:"notify_on_event"`
} // @name CreateWebhookRequest

// CreateWebhookResponse Webhook data
// swagger:response CreateWebhookResponse
type CreateWebhookResponse struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	Title         string    `json:"title"`
	ResponseCode  int       `json:"response_code"`
	ResponseDelay uint      `json:"response_delay"` // milliseconds
	ContentType   *string   `json:"content_type"`
	Payload       *string   `json:"payload"`
	NotifyOnEvent bool      `json:"notify_on_event"`
	UserID        int       `json:"user_id"`
	CreatedAt     time.Time `json:"created_at"`
	UpdatedAt     time.Time `json:"updated_at,omitempty"`
} // @name CreateWebhookResponse

// CreateWebhookApi Creates a webhook
// @Summary    Create a webhook
// @Description Returns the details of the created webhook
// @Tags        Webhooks
// @Produce     json
// @Security     ApiKeyAuth
// @Param        webhook body handlers.CreateWebhookRequest true "Webhook body"
// @Success     200  {object}  handlers.CreateWebhookResponse
// @Router      /webhooks [post]
func (h *Handler) CreateWebhookApi(w http.ResponseWriter, r *http.Request) {
	user := middlewares.GetAuthenticatedUser(r)
	input := CreateWebhookRequest{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.RenderJSON(w, http.StatusBadRequest, err.Error())
		return
	}

	code := input.ResponseCode
	if code == 0 {
		code = 200
	}

	webhook := models.Webhook{
		ID:            utils.GenerateID(),
		Title:         input.Title,
		ResponseCode:  code,
		ResponseDelay: input.ResponseDelay,
		ContentType:   &input.ContentType,
		Payload:       &input.Payload,
		UserID:        int(user.ID),
		CreatedAt:     time.Now().UTC(),
		UpdatedAt:     time.Now().UTC(),
		NotifyOnEvent: input.NotifyOnEvent,
	}

	if err := sqlstore.InsertWebhook(h.DB, webhook); err != nil {
		utils.RenderJSON(w, http.StatusInternalServerError, map[string]interface{}{
			"error": err.Error(),
		})
		return
	}

	utils.RenderJSON(w, http.StatusCreated, webhook)
}

func (h *Handler) ListWebhooksApi(w http.ResponseWriter, _ *http.Request) {
	webhooks, err := sqlstore.GetAllWebhooks(h.DB)
	if err != nil {
		utils.RenderJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	utils.RenderJSON(w, http.StatusOK, webhooks)
}
func (h *Handler) GetWebhookApi(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	webhook, err := sqlstore.GetWebhook(h.DB, webhookID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.RenderJSON(w, http.StatusNotFound, map[string]string{
				"error": "webhook not found",
			})
		}

		h.Logger.Printf("error getting webhook: %v", err)

		utils.RenderJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	utils.RenderJSON(w, http.StatusOK, webhook)
}

func (h *Handler) UpdateWebhookApi(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")

	webhook := models.Webhook{
		ID: webhookID,
	}

	var input struct {
		Title         string `json:"title"`
		ResponseCode  int    `json:"response_code"`
		ResponseDelay uint   `json:"response_delay"` // milliseconds
		ContentType   string `json:"content_type"`
		Payload       string `json:"payload"`
		NotifyOnEvent bool   `json:"notify_on_event"`
	}

	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		utils.RenderJSON(w, http.StatusBadRequest, map[string]string{
			"error": err.Error(),
		})
		return
	}

	if webhook.Title != input.Title && input.Title != "" {
		webhook.Title = input.Title
	}

	if webhook.ResponseCode != input.ResponseCode && input.ResponseCode != 0 {
		webhook.ResponseCode = input.ResponseCode
	}

	if webhook.ResponseDelay != input.ResponseDelay && input.ResponseDelay != 0 {
		webhook.ResponseDelay = input.ResponseDelay
	}

	if webhook.ContentType != &input.ContentType && input.ContentType != "" {
		webhook.ContentType = &input.ContentType
	}

	if webhook.Payload != &input.Payload {
		webhook.Payload = &input.Payload
	}

	if webhook.NotifyOnEvent != input.NotifyOnEvent {
		webhook.NotifyOnEvent = input.NotifyOnEvent
	}

	webhook.UpdatedAt = time.Now().UTC()

	if err := sqlstore.UpdateWebhook(h.DB, webhook); err != nil {
		utils.RenderJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	updated, err := sqlstore.GetWebhook(h.DB, webhookID)
	if err != nil {
		utils.RenderJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	utils.RenderJSON(w, http.StatusOK, updated)
}

func (h *Handler) DeleteWebhookApi(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	if err := sqlstore.DeleteWebhook(h.DB, id); err != nil {
		utils.RenderJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
