package controllers

import (
	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type VasController struct {
	DB *gorm.DB
}

func NewVasController(db *gorm.DB) *VasController {
	return &VasController{DB: db}
}

// DTO untuk input
type CreateMainVasDTO struct {
	Name         string  `json:"name" validate:"required"`
	IsKoli       bool    `json:"isKoli"`
	IsActive     bool    `json:"isActive"`
	DefaultPrice float64 `json:"defaultPrice"`
}

func (c *VasController) CreateMainVas(ctx *fiber.Ctx) error {
	var dto CreateMainVasDTO

	// Ambil JSON dari body request
	if err := ctx.BodyParser(&dto); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Simpan ke DB
	vas := models.MainVas{
		Name:         dto.Name,
		IsKoli:       dto.IsKoli,
		IsActive:     dto.IsActive,
		DefaultPrice: dto.DefaultPrice,
		CreatedBy:    int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&vas).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	vasRate := models.VasRate{
		MainVasId: vas.ID,
		RateIdr:   int(dto.DefaultPrice),
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&vasRate).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Return JSON dengan ID
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "VAS created successfully",
		"data":    vas,
	})
}

// Get all
func (c *VasController) GetAllMainVas(ctx *fiber.Ctx) error {
	var items []models.MainVas
	if err := c.DB.Find(&items).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to fetch", "error": err.Error(),
		})
	}
	return ctx.JSON(fiber.Map{"success": true, "data": items})
}

// Update
func (c *VasController) UpdateMainVas(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	var body models.MainVas

	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false, "message": "Invalid request", "error": err.Error(),
		})
	}

	body.UpdatedBy = int(ctx.Locals("userID").(float64))

	if err := c.DB.Model(&models.MainVas{}).Where("id = ?", id).Updates(body).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to update", "error": err.Error(),
		})
	}

	mainVas := models.MainVas{}
	if err := c.DB.First(&mainVas, id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to fetch", "error": err.Error(),
		})
	}

	vasRate := models.VasRate{
		MainVasId: mainVas.ID,
		RateIdr:   int(body.DefaultPrice),
		CreatedBy: int(ctx.Locals("userID").(float64)),
		UpdatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&vasRate).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{"success": true, "message": "Vas updated"})
}

// DTO untuk input
type CreateVasDTO struct {
	IsActive   bool  `json:"isActive"`
	MainVasIds []int `json:"mainVasIds"`
}

func (c *VasController) CreateVas(ctx *fiber.Ctx) error {
	var dto CreateVasDTO

	// Ambil JSON dari body request
	if err := ctx.BodyParser(&dto); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	var mainVas []models.MainVas
	if err := c.DB.Where("id IN ?", dto.MainVasIds).Find(&mainVas).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	var vasName string
	for _, vas := range mainVas {
		vasName += vas.Name + ", "
	}
	vasName = vasName[:len(vasName)-2] // Remove last comma and space

	// Simpan ke DB
	vas := models.Vas{
		Name:      vasName,
		IsActive:  dto.IsActive,
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&vas).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}
	// Simpan detail

	for _, mainVas := range mainVas {
		vas.MainVasDetails = append(vas.MainVasDetails, models.VasDetail{
			VasId:     int(vas.ID),
			MainVasId: mainVas.ID,
		})
	}

	if err := c.DB.Create(&vas.MainVasDetails).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Return JSON dengan ID
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "VAS created successfully",
		"data":    vas,
	})
}

func (c *VasController) GetAllVas(ctx *fiber.Ctx) error {
	var items []models.Vas
	if err := c.DB.Preload("MainVasDetails").Find(&items).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false, "message": "Failed to fetch", "error": err.Error(),
		})
	}
	return ctx.JSON(fiber.Map{"success": true, "data": items})
}

type UpdateVasDTO struct {
	IsActive   bool  `json:"isActive"`
	MainVasIds []int `json:"mainVasIds"`
}

func (c *VasController) UpdateVas(ctx *fiber.Ctx) error {
	id := ctx.Params("id")

	var dto UpdateVasDTO
	if err := ctx.BodyParser(&dto); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Cari vas existing
	var vas models.Vas
	if err := c.DB.Preload("MainVasDetails").First(&vas, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"error":   "VAS not found",
		})
	}

	// Ambil main vas baru dari dto
	var mainVas []models.MainVas
	if err := c.DB.Where("id IN ?", dto.MainVasIds).Find(&mainVas).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Bikin name baru dari gabungan mainVas
	var vasName string
	for _, m := range mainVas {
		vasName += m.Name + ", "
	}
	if len(vasName) > 2 {
		vasName = vasName[:len(vasName)-2]
	}

	// Update vas utama
	vas.Name = vasName
	vas.IsActive = dto.IsActive
	vas.UpdatedBy = int(ctx.Locals("userID").(float64))
	if err := c.DB.Save(&vas).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	if err := c.DB.
		Where("vas_id = ?", vas.ID).
		Unscoped().
		Delete(&models.VasDetail{}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Insert detail baru
	var newDetails []models.VasDetail
	for _, m := range mainVas {
		newDetails = append(newDetails, models.VasDetail{
			VasId:     int(vas.ID),
			MainVasId: m.ID,
		})
	}
	if len(newDetails) > 0 {
		if err := c.DB.Create(&newDetails).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"error":   err.Error(),
			})
		}
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "VAS updated successfully",
		"data":    vas,
	})
}
