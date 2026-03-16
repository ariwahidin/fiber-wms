package owner_controller

import (
	"errors"
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"
	"fiber-app/models"
	"fmt"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type OwnerController struct {
	DB *gorm.DB
}

func NewOwnerController(db *gorm.DB) *OwnerController {
	return &OwnerController{DB: db}
}

func (c *OwnerController) CreateOwner(ctx *fiber.Ctx) error {
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

func (c *OwnerController) GetAllOwner(ctx *fiber.Ctx) error {

	var models []models.Owner
	if err := c.DB.Find(&models).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Owner found", "data": models})
}

func (c *OwnerController) GetOwnerByID(ctx *fiber.Ctx) error {
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

func (c *OwnerController) UpdateOwner(ctx *fiber.Ctx) error {
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

func (c *OwnerController) GetOwnerByUserID(ctx *fiber.Ctx) error {
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

func SetupOwnerRoutes(app *fiber.App) {
	api := app.Group(config.MAIN_ROUTES+"/owners", middleware.AuthMiddleware)
	ownerController := &OwnerController{}
	api.Use(database.InjectDBMiddleware(ownerController))
	api.Get("/", ownerController.GetAllOwner)
	api.Post("/", ownerController.CreateOwner)
	api.Put("/:id", ownerController.UpdateOwner)
	api.Get("/user/", ownerController.GetOwnerByUserID)
}

//=======================================================
// END SETUP ROUTES
//=======================================================
