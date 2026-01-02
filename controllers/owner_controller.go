package controllers

import (
	"errors"
	"fiber-app/config"
	"fiber-app/middleware"
	"fiber-app/models"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func (c *Controller) CreateOwner(ctx *fiber.Ctx) error {
	var model models.Owner

	if err := ctx.BodyParser(&model); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	model.CreatedBy = int(ctx.Locals("userID").(float64))

	if err := c.DB.Create(&model).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Owner created successfully", "data": model})
}

func (c *Controller) GetAllOwner(ctx *fiber.Ctx) error {

	var models []models.Owner
	if err := c.DB.Find(&models).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Owner found", "data": models})
}

func (c *Controller) GetOwnerByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var model models.Owner
	if err := c.DB.First(&model, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Owner not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Owner found", "data": model})
}

func (c *Controller) UpdateOwner(ctx *fiber.Ctx) error {
	fmt.Println("Update Owner")
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	// id = uint(id)
	var model models.Owner
	if err := ctx.BodyParser(&model); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	model.UpdatedBy = int(ctx.Locals("userID").(float64))
	if err := c.DB.Model(&model).Where("id = ?", id).Updates(model).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Owner updated successfully", "data": model})
}

func (c *Controller) GetOwnerByUserID(ctx *fiber.Ctx) error {
	userID := int(ctx.Locals("userID").(float64))
	var models []models.UserOwner
	if err := c.DB.Where("user_id = ?", userID).Find(&models).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Owner found", "data": models})
}

//=======================================================
// BEGIN SETUP ROUTES
//=======================================================

func (ctrl *Controller) SetupOwnerRoutes(app *fiber.App) {
	api := app.Group(config.MAIN_ROUTES+"/owners", middleware.AuthMiddleware)
	api.Get("/", ctrl.GetAllOwner)
	api.Post("/", ctrl.CreateOwner)
	api.Put("/:id", ctrl.UpdateOwner)
	api.Get("/user/", ctrl.GetOwnerByUserID)
}

//=======================================================
// END SETUP ROUTES
//=======================================================
