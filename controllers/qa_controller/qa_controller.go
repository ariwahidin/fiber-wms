package qa_controller

import (
	"errors"
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"
	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type QaController struct {
	DB *gorm.DB
}

func NewQaController(db *gorm.DB) *QaController {
	return &QaController{DB: db}
}

func (c *QaController) CreateQA(ctx *fiber.Ctx) error {
	var model models.QaStatus

	if err := ctx.BodyParser(&model); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	model.CreatedBy = int(ctx.Locals("userID").(float64))

	if err := c.DB.Create(&model).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Qa Status created successfully", "data": model})
}

func (c *QaController) GetAllQA(ctx *fiber.Ctx) error {

	var models []models.QaStatus
	if err := c.DB.Find(&models).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Qa Status found", "data": models})
}

func (c *QaController) GetQAByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var model models.QaStatus
	if err := c.DB.First(&model, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Qa Status not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Qa Status found", "data": model})
}

func (c *QaController) UpdateQA(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var model models.QaStatus
	if err := ctx.BodyParser(&model); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	var existingModel models.QaStatus

	if err := c.DB.First(&existingModel, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Qa Status not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if model.Description == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Description is required"})
	}

	var inboundDetail models.InboundDetail
	if err := c.DB.Where("qa_status = ?", existingModel.QaStatus).First(&inboundDetail).Error; err != nil {
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	if inboundDetail.ID > 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Qa Status already used in transaction"})
	}

	model.UpdatedBy = int(ctx.Locals("userID").(float64))
	if err := c.DB.Debug().Model(&model).Where("id = ?", id).Updates(model).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Qa Status updated successfully", "data": model})
}

//=======================================================
// BEGIN SETUP QA ROUTES
//=======================================================

func SetupQaRoutes(app *fiber.App) {

	api := app.Group(config.MAIN_ROUTES+"/qa-status", middleware.AuthMiddleware)
	qaController := &QaController{}
	api.Use(database.InjectDBMiddleware(qaController))

	api.Post("/", qaController.CreateQA)
	api.Get("/", qaController.GetAllQA)
	api.Put("/:id", qaController.UpdateQA)
}

//=======================================================
// END SETUP QA ROUTES
//=======================================================
