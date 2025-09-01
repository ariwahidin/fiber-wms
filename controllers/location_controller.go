package controllers

import (
	"fiber-app/models"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type LocationController struct {
	DB *gorm.DB
}

func NewLocationController(DB *gorm.DB) *LocationController {
	return &LocationController{DB: DB}
}

// CREATE
func (lc *LocationController) CreateLocation(ctx *fiber.Ctx) error {
	userID := int(ctx.Locals("userID").(float64))

	var location models.Location
	if err := ctx.BodyParser(&location); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	location.LocationCode = location.Row + location.Bay + location.Level + location.Bin
	// if location.LocationCode == "" {
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Location code is required"})
	// }

	// if location.Bay%2 != 0 {
	// 	location.Area = "Ganjil"

	// } else {
	// 	location.Area = "Genap"
	// }

	bayInt, err := strconv.Atoi(location.Bay)
	if err != nil {
		location.Area = "Unknown"
	} else {
		if bayInt%2 != 0 {
			location.Area = "ganjil"
		} else {
			location.Area = "genap"
		}
	}

	location.CreatedBy = userID
	location.UpdatedBy = userID

	if err := lc.DB.Create(&location).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "Location created successfully",
		"data":    location,
	})
}

// READ ALL
func (lc *LocationController) GetAllLocations(ctx *fiber.Ctx) error {
	var locations []models.Location
	if err := lc.DB.Find(&locations).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    locations,
	})
}

// READ BY ID
func (lc *LocationController) GetLocationByID(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	var location models.Location

	if err := lc.DB.First(&location, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Location not found"})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    location,
	})
}

// UPDATE
func (lc *LocationController) UpdateLocation(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	userID := int(ctx.Locals("userID").(float64))

	var location models.Location
	if err := lc.DB.First(&location, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Location not found"})
	}

	var input models.Location
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	bayInt, err := strconv.Atoi(location.Bay)
	if err != nil {
		location.Area = "Unknown"
	} else {
		if bayInt%2 != 0 {
			location.Area = "ganjil"
		} else {
			location.Area = "genap"
		}
	}

	// location.LocationCode = input.LocationCode
	location.LocationCode = location.Row + location.Bay + location.Level + location.Bin
	location.Row = input.Row
	location.Bay = input.Bay
	location.Level = input.Level
	location.Bin = input.Bin
	// location.Area = input.Area
	location.IsActive = input.IsActive
	location.UpdatedBy = userID

	if err := lc.DB.Save(&location).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Location updated successfully",
		"data":    location,
	})
}

// DELETE
func (lc *LocationController) DeleteLocation(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	userID := int(ctx.Locals("userID").(float64))

	var location models.Location
	if err := lc.DB.First(&location, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Location not found"})
	}

	location.DeletedBy = userID
	if err := lc.DB.Save(&location).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := lc.DB.Delete(&location).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"message": "Location deleted successfully",
	})
}
