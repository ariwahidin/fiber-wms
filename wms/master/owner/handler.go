package owner

import (
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type OwnerHandler struct {
	DB *gorm.DB
}

func NewOwnerHandler(db *gorm.DB) *OwnerHandler {
	return &OwnerHandler{DB: db}
}

func (h *OwnerHandler) GetAllOwners(ctx *fiber.Ctx) error {
	var owners []Owner
	if err := h.DB.Find(&owners).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "Failed to retrieve owners",
		})
	}
	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Owners retrieved successfully",
		"data":    owners,
	})
}
