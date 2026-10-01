package store

import (
	"log"

	"gorm.io/gorm"

	"webhook-tester/internal/models"
	"webhook-tester/internal/repository"
	"webhook-tester/internal/utils"
)

// Ensure GormDeliveryRepo implements repository.DeliveryRepository
var _ repository.DeliveryRepository = &GormDeliveryRepo{}

// GormDeliveryRepo is a GORM implementation of DeliveryRepository.
type GormDeliveryRepo struct {
	DB     *gorm.DB
	logger *log.Logger
}

// NewGormDeliveryRepo constructs a new repository with a logger.
func NewGormDeliveryRepo(db *gorm.DB, logger *log.Logger) *GormDeliveryRepo {
	return &GormDeliveryRepo{DB: db, logger: logger}
}

// Insert stores d, generating its ID if unset.
func (r *GormDeliveryRepo) Insert(d *models.Delivery) error {
	if d.ID == "" {
		d.ID = utils.GenerateID()
	}
	if err := r.DB.Create(d).Error; err != nil {
		r.logger.Printf("insert delivery for request %s failed: %v", d.RequestID, err)
		return err
	}
	return nil
}

// ListByRequest returns the captured request's deliveries, newest first.
func (r *GormDeliveryRepo) ListByRequest(requestID string) ([]models.Delivery, error) {
	var list []models.Delivery
	if err := r.DB.
		Where("request_id = ?", requestID).
		Order(newestDeliveriesFirst).
		Find(&list).Error; err != nil {
		r.logger.Printf("list deliveries for request %s failed: %v", requestID, err)
		return nil, err
	}
	return list, nil
}

// DeleteByRequest removes all deliveries of a captured request.
func (r *GormDeliveryRepo) DeleteByRequest(requestID string) error {
	if err := deleteDeliveriesByRequest(r.DB, requestID); err != nil {
		r.logger.Printf("delete deliveries for request %s failed: %v", requestID, err)
		return err
	}
	return nil
}

// DeleteByWebhook removes all deliveries of a webhook's captured requests.
func (r *GormDeliveryRepo) DeleteByWebhook(webhookID string) error {
	if err := deleteDeliveriesByWebhooks(r.DB, webhookID); err != nil {
		r.logger.Printf("delete deliveries for webhook %s failed: %v", webhookID, err)
		return err
	}
	return nil
}

// deleteDeliveriesByRequest deletes a captured request's deliveries. The
// request delete paths call it inside their transaction, before deleting the
// request itself, so no delivery is orphaned.
func deleteDeliveriesByRequest(tx *gorm.DB, requestID string) error {
	return tx.Where("request_id = ?", requestID).Delete(&models.Delivery{}).Error
}

// deleteDeliveriesByWebhooks deletes the deliveries of the given webhooks.
// The webhook and bulk request delete paths call it inside their
// transaction, before deleting the requests, so no delivery is orphaned.
func deleteDeliveriesByWebhooks(tx *gorm.DB, webhookIDs ...string) error {
	return tx.Where("webhook_id IN ?", webhookIDs).Delete(&models.Delivery{}).Error
}

// newestDeliveriesFirst orders deliveries newest first.
const newestDeliveriesFirst = "started_at DESC, id DESC"

// deliveriesQueryChunk bounds how many request IDs one deliveries query
// matches, keeping it well below the databases' bind parameter limits.
const deliveriesQueryChunk = 500

// attachDeliveries loads the deliveries of the given captured requests into
// each request's Deliveries, newest first.
func attachDeliveries(db *gorm.DB, lists ...[]models.WebhookRequest) error {
	var ids []string
	for _, list := range lists {
		for _, wr := range list {
			ids = append(ids, wr.ID)
		}
	}

	byRequest := make(map[string][]models.Delivery)
	for start := 0; start < len(ids); start += deliveriesQueryChunk {
		chunk := ids[start:min(start+deliveriesQueryChunk, len(ids))]
		var found []models.Delivery
		if err := db.Where("request_id IN ?", chunk).Order(newestDeliveriesFirst).Find(&found).Error; err != nil {
			return err
		}
		for _, d := range found {
			byRequest[d.RequestID] = append(byRequest[d.RequestID], d)
		}
	}

	for _, list := range lists {
		for i := range list {
			list[i].Deliveries = byRequest[list[i].ID]
		}
	}
	return nil
}
