package controllers

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type DeliveryHubRequest struct {
	Name     string `json:"name"`
	Address  string `json:"address"`
	City     string `json:"city"`
	IsActive *bool  `json:"is_active"`
}

func (c *ShippingController) GetDeliveryHubs(ctx *fiber.Ctx) error {
	var hubs []models.DeliveryHub

	query := c.DB.Model(&models.DeliveryHub{})

	if ctx.Query("all") != "true" {
		query = query.Where("is_active = ?", true)
	}

	if err := query.Order("name ASC").Find(&hubs).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to retrieve delivery hubs",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    hubs,
	})
}

func (c *ShippingController) CreateDeliveryHub(ctx *fiber.Ctx) error {
	var payload DeliveryHubRequest

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Address = strings.TrimSpace(payload.Address)
	payload.City = strings.TrimSpace(payload.City)

	if payload.Name == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Hub name is required",
		})
	}

	hub := models.DeliveryHub{
		Name:     payload.Name,
		Address:  payload.Address,
		City:     payload.City,
		IsActive: true,
	}

	tx := c.DB.Begin()
	if tx.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   tx.Error.Error(),
		})
	}

	if err := tx.Create(&hub).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to create delivery hub",
			"error":   err.Error(),
		})
	}

	// ID berasal dari database sehingga kode tidak bergantung
	// pada perhitungan jumlah record yang rawan bentrok.
	hub.Code = fmt.Sprintf("HUB-%06d", hub.ID)

	if err := tx.Model(&hub).Update("code", hub.Code).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to generate delivery hub code",
			"error":   err.Error(),
		})
	}

	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit delivery hub",
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Delivery hub created successfully",
		"data":    hub,
	})
}

func (c *ShippingController) UpdateDeliveryHub(ctx *fiber.Ctx) error {
	id := ctx.Params("id")

	var payload DeliveryHubRequest

	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid payload",
			"error":   err.Error(),
		})
	}

	payload.Name = strings.TrimSpace(payload.Name)
	payload.Address = strings.TrimSpace(payload.Address)
	payload.City = strings.TrimSpace(payload.City)

	if payload.Name == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Hub name is required",
		})
	}

	var hub models.DeliveryHub

	if err := c.DB.First(&hub, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "Delivery hub not found",
			})
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	hub.Name = payload.Name
	hub.Address = payload.Address
	hub.City = payload.City

	if payload.IsActive != nil {
		hub.IsActive = *payload.IsActive
	}

	if err := c.DB.Save(&hub).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update delivery hub",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Delivery hub updated successfully",
		"data":    hub,
	})
}

func (c *ShippingController) DeleteDeliveryHub(ctx *fiber.Ctx) error {
	id := ctx.Params("id")

	result := c.DB.Model(&models.DeliveryHub{}).
		Where("id = ?", id).
		Update("is_active", false)

	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   result.Error.Error(),
		})
	}

	if result.RowsAffected == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Delivery hub not found",
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Delivery hub deactivated successfully",
	})
}
