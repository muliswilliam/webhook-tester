package handlers

import (
	"encoding/json"
	"errors"
	"net/http"
	"time"
	"webhook-tester/internal/dtos"
	"webhook-tester/internal/middlewares"
	"webhook-tester/internal/models"
	sqlstore "webhook-tester/internal/store/sql"
	"webhook-tester/internal/utils"

	"gorm.io/gorm"

	"github.com/go-chi/chi/v5"
)

// CreateWebhookApi Creates a webhook
// @Summary    Create a webhook
// @Description Returns the details of the created webhook
// @Tags        Webhooks
// @Produce     json
// @Security     ApiKeyAuth
// @Param        webhook body dtos.CreateWebhookRequest true "Webhook body"
// @Success     200  {object}  dtos.Webhook
// @Router      /webhooks [post]
func (h *Handler) CreateWebhookApi(w http.ResponseWriter, r *http.Request) {
	user := middlewares.GetAuthenticatedUser(r)
	input := dtos.CreateWebhookRequest{}
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
	h.Metrics.IncWebhooksCreated()
	dto := dtos.NewWebhookDTO(webhook)
	dto.Requests = make([]models.WebhookRequest, 0)
	utils.RenderJSON(w, http.StatusCreated, dto)
}

// ListWebhooksApi Creates a webhook
// @Summary    List webhooks
// @Description List webhooks and associated request
// @Tags        Webhooks
// @Produce     json
// @Security     ApiKeyAuth
// @Success     200  {object} []dtos.Webhook
// @Router      /webhooks [get]
func (h *Handler) ListWebhooksApi(w http.ResponseWriter, r *http.Request) {
	user := middlewares.GetAuthenticatedUser(r)
	webhooks, err := sqlstore.GetUserWebhooks(user.ID, h.DB)
	if err != nil {
		utils.RenderJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	utils.RenderJSON(w, http.StatusOK, webhooks)
}

// GetWebhookApi Gets a webhook by webhook ID
// @Summary    Get webhook by ID
// @Description Get a webhook by ID along with its requests
// @Tags        Webhooks
// @Produce     json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Webhook ID"
// @Success     200  {object} []dtos.Webhook
// @Router      /webhooks/{id} [get]
func (h *Handler) GetWebhookApi(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	user := middlewares.GetAuthenticatedUser(r)
	webhook, err := sqlstore.GetUserWebhook(h.DB, webhookID, user.ID)

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			utils.RenderJSON(w, http.StatusNotFound, map[string]string{
				"error": "webhook not found",
			})
			return
		}

		h.Logger.Printf("error getting webhook: %v", err)

		utils.RenderJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}
	dto := dtos.NewWebhookDTO(webhook)
	utils.RenderJSON(w, http.StatusOK, dto)
}

// UpdateWebhookApi Updates a webhook
// @Summary  Updates a webhook
// @Description Updates a webhook
// @Tags        Webhooks
// @Produce     json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Webhook ID"
// @Param        webhook body dtos.UpdateWebhookRequest true "Updated webhook"
// @Success     200  {object} []dtos.Webhook
// @Router      /webhooks/{id} [put]
func (h *Handler) UpdateWebhookApi(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	user := middlewares.GetAuthenticatedUser(r)
	webhook, err := sqlstore.GetUserWebhook(h.DB, webhookID, user.ID)
	if err != nil {
		utils.RenderJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	input := dtos.UpdateWebhookRequest{}

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

	dto := dtos.NewWebhookDTO(webhook)
	utils.RenderJSON(w, http.StatusOK, dto)
}

// DeleteWebhookApi deletes a webhook
// @Summary      Delete a webhook
// @Description  Deletes a webhook
// @Tags         Webhooks
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Webhook ID"
// @Success      204  {string}  string  "No Content"
// @Failure      500  {object}  ErrorResponse
// @Router       /webhooks/{id} [delete]
func (h *Handler) DeleteWebhookApi(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middlewares.GetAuthenticatedUser(r)
	if err := sqlstore.DeleteUserWebhook(h.DB, id, user.ID); err != nil {
		utils.RenderJSON(w, http.StatusInternalServerError, map[string]string{
			"error": err.Error(),
		})
		return
	}

	w.WriteHeader(http.StatusNoContent)
}
