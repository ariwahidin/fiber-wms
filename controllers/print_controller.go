package controllers

import (
	"fiber-app/config"
	"fiber-app/middleware"
	"fiber-app/services"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type PrintController struct {
	PrintService *services.PrintService
}

func NewPrintController(
	printService *services.PrintService,
) *PrintController {
	return &PrintController{
		PrintService: printService,
	}
}

func (c *PrintController) CreatePrintLog(ctx *fiber.Ctx) error {
	var req services.CreatePrintLogRequest

	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{
				"success": false,
				"message": "Invalid request payload",
			},
		)
	}

	// Get authenticated user ID
	userIDValue := ctx.Locals("userID")

	userIDFloat, ok := userIDValue.(float64)
	if !ok || userIDFloat <= 0 {
		return ctx.Status(fiber.StatusUnauthorized).JSON(
			fiber.Map{
				"success": false,
				"message": "Unauthorized",
			},
		)
	}

	result, err := c.PrintService.CreateLog(
		services.CreatePrintLogParams{
			Request:   req,
			UserID:    uint(userIDFloat),
			IPAddress: ctx.IP(),
		},
	)

	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{
				"success": false,
				"message": err.Error(),
			},
		)
	}

	return ctx.Status(fiber.StatusCreated).JSON(
		fiber.Map{
			"success": true,
			"message": "Print activity recorded successfully",
			"data":    result,
		},
	)
}

// ============================================================
// PRINT ROUTES
// ============================================================

func SetupPrintRoutes(app *fiber.App, db *gorm.DB) {
	printService := services.NewPrintService(db)

	printController := NewPrintController(printService)

	api := app.Group(
		config.MAIN_ROUTES+"/print",
		middleware.AuthMiddleware,
	)

	api.Post("/log", printController.CreatePrintLog)
}
