package handlers

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"webhook-tester/internal/dtos"
	"webhook-tester/internal/metrics"
	"webhook-tester/internal/middlewares"
	"webhook-tester/internal/models"
	"webhook-tester/internal/service"
	"webhook-tester/internal/utils"

	"gorm.io/datatypes"
	"gorm.io/gorm"

	"github.com/go-chi/chi/v5"
)

type WebhookAiHandler struct {
	Service *service.WebhookService
	Metrics metrics.Recorder
	Logger  *log.Logger
}

func NewWebhookApiHandler(svc *service.WebhookService, m metrics.Recorder, l *log.Logger) *WebhookAiHandler {
	return &WebhookAiHandler{Service: svc, Metrics: m, Logger: l}
}

// CreateWebhookApi Creates a webhook
// @Summary    Create a webhook
// @Description Returns the details of the created webhook
// @Tags        Webhooks
// @Accept      json
// @Produce     json
// @Security     ApiKeyAuth
// @Param        webhook body dtos.CreateWebhookRequest true "Webhook body"
// @Success     201  {object}  dtos.Webhook
// @Failure     400  {object}  dtos.ErrorResponse
// @Failure     401  {object}  dtos.ErrorResponse
// @Router      /webhooks [post]
func (h *WebhookAiHandler) CreateWebhookApi(w http.ResponseWriter, r *http.Request) {
	user := middlewares.GetAPIAuthenticatedUser(r)
	input := dtos.CreateWebhookRequest{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		renderAPIError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	webhook := models.Webhook{
		ID:              utils.GenerateID(),
		Title:           input.Title,
		ResponseCode:    input.ResponseCode,
		ResponseDelay:   input.ResponseDelay,
		ContentType:     &input.ContentType,
		Payload:         &input.Payload,
		ResponseHeaders: headersMap(input.ResponseHeaders),
		UserID:          int(user.ID),
		NotifyOnEvent:   input.NotifyOnEvent,
		ForwardURL:      &input.ForwardURL, // Normalize unsets ""
	}
	webhook.Normalize()
	if err := h.Service.ValidateWebhook(r.Context(), &webhook); err != nil {
		renderAPIError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.Service.CreateWebhook(&webhook); err != nil {
		h.Logger.Printf("error creating webhook: %v", err)
		renderAPIError(w, http.StatusInternalServerError, "could not create webhook")
		return
	}
	h.Metrics.IncWebhooksCreated()
	utils.RenderJSON(w, http.StatusCreated, dtos.NewWebhookDTO(webhook))
}

// ListWebhooksApi Lists webhooks
// @Summary    List webhooks
// @Description List webhooks and their most recent requests
// @Tags        Webhooks
// @Produce     json
// @Security     ApiKeyAuth
// @Success     200  {array} dtos.Webhook
// @Failure     401  {object}  dtos.ErrorResponse
// @Router      /webhooks [get]
func (h *WebhookAiHandler) ListWebhooksApi(w http.ResponseWriter, r *http.Request) {
	user := middlewares.GetAPIAuthenticatedUser(r)
	webhooks, err := h.Service.ListWebhooks(user.ID)
	if err != nil {
		h.Logger.Printf("error listing webhooks: %v", err)
		renderAPIError(w, http.StatusInternalServerError, "could not list webhooks")
		return
	}

	out := make([]dtos.Webhook, len(webhooks))
	for i, wh := range webhooks {
		out[i] = dtos.NewWebhookDTO(wh)
	}
	utils.RenderJSON(w, http.StatusOK, out)
}

// GetWebhookApi Gets a webhook by webhook ID
// @Summary    Get webhook by ID
// @Description Get a webhook by ID along with its requests
// @Tags        Webhooks
// @Produce     json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Webhook ID"
// @Success     200  {object} dtos.Webhook
// @Failure     401  {object}  dtos.ErrorResponse
// @Failure     404  {object}  dtos.ErrorResponse
// @Router      /webhooks/{id} [get]
func (h *WebhookAiHandler) GetWebhookApi(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	user := middlewares.GetAPIAuthenticatedUser(r)
	webhook, err := h.Service.GetUserWebhookWithRequests(webhookID, user.ID)
	if err != nil {
		h.renderLookupError(w, err)
		return
	}
	utils.RenderJSON(w, http.StatusOK, dtos.NewWebhookDTO(*webhook))
}

// UpdateWebhookApi Updates a webhook
// @Summary  Update a webhook
// @Description Changes only the fields included in the body. PUT is accepted as an alias of PATCH.
// @Tags        Webhooks
// @Accept      json
// @Produce     json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Webhook ID"
// @Param        webhook body dtos.UpdateWebhookRequest true "Fields to change"
// @Success     200  {object} dtos.Webhook
// @Failure     400  {object}  dtos.ErrorResponse
// @Failure     401  {object}  dtos.ErrorResponse
// @Failure     404  {object}  dtos.ErrorResponse
// @Router      /webhooks/{id} [patch]
// @Router      /webhooks/{id} [put]
func (h *WebhookAiHandler) UpdateWebhookApi(w http.ResponseWriter, r *http.Request) {
	webhookID := chi.URLParam(r, "id")
	user := middlewares.GetAPIAuthenticatedUser(r)
	current, err := h.Service.GetUserWebhook(webhookID, user.ID)
	if err != nil {
		h.renderLookupError(w, err)
		return
	}
	// Apply the changes to a copy, so a rejected update leaves no trace.
	webhook := new(models.Webhook)
	*webhook = *current

	input := dtos.UpdateWebhookRequest{}
	if err := json.NewDecoder(r.Body).Decode(&input); err != nil {
		renderAPIError(w, http.StatusBadRequest, "invalid JSON body: "+err.Error())
		return
	}

	if input.Title != nil {
		webhook.Title = *input.Title
	}
	if input.ResponseCode != nil {
		// Normalize treats 0 as unset, but an explicit 0 is a bad value.
		if err := models.ValidateResponseCode(*input.ResponseCode); err != nil {
			renderAPIError(w, http.StatusBadRequest, err.Error())
			return
		}
		webhook.ResponseCode = *input.ResponseCode
	}
	if input.ResponseDelay != nil {
		webhook.ResponseDelay = *input.ResponseDelay
	}
	if input.ContentType != nil {
		webhook.ContentType = input.ContentType // "" resets to the default
	}
	if input.Payload != nil {
		webhook.Payload = input.Payload
	}
	if input.ResponseHeaders != nil {
		webhook.ResponseHeaders = headersMap(*input.ResponseHeaders)
	}
	if input.NotifyOnEvent != nil {
		webhook.NotifyOnEvent = *input.NotifyOnEvent
	}
	if input.ForwardURL.Set {
		webhook.ForwardURL = input.ForwardURL.Value // null and "" both clear it
	}
	webhook.Normalize()
	if err := h.Service.ValidateWebhook(r.Context(), webhook); err != nil {
		renderAPIError(w, http.StatusBadRequest, err.Error())
		return
	}

	if err := h.Service.UpdateWebhook(webhook); err != nil {
		h.Logger.Printf("error updating webhook: %v", err)
		renderAPIError(w, http.StatusInternalServerError, "could not update webhook")
		return
	}

	// Answer with the same shape as GET, requests included.
	updated, err := h.Service.GetUserWebhookWithRequests(webhookID, user.ID)
	if err != nil {
		h.renderLookupError(w, err)
		return
	}
	utils.RenderJSON(w, http.StatusOK, dtos.NewWebhookDTO(*updated))
}

// DeleteWebhookApi deletes a webhook
// @Summary      Delete a webhook
// @Description  Deletes a webhook and its captured requests
// @Tags         Webhooks
// @Produce      json
// @Security     ApiKeyAuth
// @Param        id   path      string  true  "Webhook ID"
// @Success      204  {string}  string  "No Content"
// @Failure      401  {object}  dtos.ErrorResponse
// @Failure      404  {object}  dtos.ErrorResponse
// @Router       /webhooks/{id} [delete]
func (h *WebhookAiHandler) DeleteWebhookApi(w http.ResponseWriter, r *http.Request) {
	id := chi.URLParam(r, "id")
	user := middlewares.GetAPIAuthenticatedUser(r)
	if err := h.Service.DeleteWebhook(id, user.ID); err != nil {
		h.renderLookupError(w, err)
		return
	}

	w.WriteHeader(http.StatusNoContent)
}

// renderLookupError answers a failed webhook lookup: 404 when the caller has
// no such webhook, 500 otherwise.
func (h *WebhookAiHandler) renderLookupError(w http.ResponseWriter, err error) {
	if errors.Is(err, gorm.ErrRecordNotFound) {
		renderAPIError(w, http.StatusNotFound, "webhook not found")
		return
	}
	h.Logger.Printf("error loading webhook: %v", err)
	renderAPIError(w, http.StatusInternalServerError, "could not load webhook")
}

func renderAPIError(w http.ResponseWriter, status int, message string) {
	utils.RenderJSON(w, status, dtos.ErrorResponse{Error: message})
}

func headersMap(headers map[string]string) datatypes.JSONMap {
	if len(headers) == 0 {
		return nil
	}
	m := make(datatypes.JSONMap, len(headers))
	for k, v := range headers {
		m[k] = v
	}
	return m
}
