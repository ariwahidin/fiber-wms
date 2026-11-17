package controllers

import (
	"errors"
	"strings"

	"fiber-app/models"

	"gorm.io/gorm"
)

const (
	DeliveryModeDirect = "DIRECT"
	DeliveryModeHub    = "VIA_HUB"
)

// Apply the selected delivery mode to one SPK detail.
// Hub information is always loaded from the master table;
// client-supplied hub name/address is never trusted.
func applyDeliveryAssignment(
	tx *gorm.DB,
	item OrderItem,
	detail *models.OrderDetail,
) error {
	mode := strings.ToUpper(strings.TrimSpace(item.DeliveryMode))

	// Backward compatibility for existing frontend payloads.
	if mode == "" {
		mode = DeliveryModeDirect
	}

	switch mode {
	case DeliveryModeDirect:
		detail.DeliveryMode = DeliveryModeDirect
		detail.DeliveryHubID = nil
		detail.DeliveryHubCode = ""
		detail.DeliveryHubName = ""
		detail.DeliveryHubAddress = ""
		detail.DeliveryHubCity = ""
		return nil

	case DeliveryModeHub:
		if item.DeliveryHubID == nil || *item.DeliveryHubID == 0 {
			return errors.New("delivery hub is required for VIA_HUB assignment")
		}

		var hub models.DeliveryHub

		if err := tx.
			Where("id = ? AND is_active = ?", *item.DeliveryHubID, true).
			First(&hub).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				return errors.New("selected delivery hub does not exist or is inactive")
			}
			return err
		}

		hubID := hub.ID

		detail.DeliveryMode = DeliveryModeHub
		detail.DeliveryHubID = &hubID
		detail.DeliveryHubCode = hub.Code
		detail.DeliveryHubName = hub.Name
		detail.DeliveryHubAddress = hub.Address
		detail.DeliveryHubCity = hub.City

		return nil

	default:
		return errors.New("invalid delivery mode; allowed values are DIRECT or VIA_HUB")
	}
}
