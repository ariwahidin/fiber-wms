package controllers

import (
	"errors"
	"strconv"

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

// ============================================================
// CREATE PRINT LOG
// ============================================================

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

	// Get authenticated user ID from middleware.
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
// GET PRINT LOGS
// ============================================================

func (c *PrintController) GetPrintLogs(ctx *fiber.Ctx) error {

	page, err := strconv.Atoi(ctx.Query("page", "1"))

	if err != nil {
		page = 1
	}

	pageSize, err := strconv.Atoi(ctx.Query("page_size", "20"))

	if err != nil {
		pageSize = 20
	}

	result, err := c.PrintService.GetLogs(
		services.PrintLogFilter{
			Page:         page,
			PageSize:     pageSize,
			Search:       ctx.Query("search"),
			Module:       ctx.Query("module"),
			DocumentType: ctx.Query("document_type"),
			PrintAction:  ctx.Query("print_action"),
			Status:       ctx.Query("status"),
			DateFrom:     ctx.Query("date_from"),
			DateTo:       ctx.Query("date_to"),
		},
	)

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(
			fiber.Map{
				"success": false,
				"message": "Failed to retrieve print logs",
			},
		)
	}

	return ctx.JSON(
		fiber.Map{
			"success":    true,
			"message":    "Print logs retrieved successfully",
			"data":       result.Data,
			"summary":    result.Summary,
			"pagination": result.Pagination,
		},
	)
}

// ============================================================
// GET PRINT LOG BY ID
// ============================================================

func (c *PrintController) GetPrintLogByID(ctx *fiber.Ctx) error {

	id, err := strconv.ParseUint(ctx.Params("id"), 10, 64)

	if err != nil || id == 0 {
		return ctx.Status(fiber.StatusBadRequest).JSON(
			fiber.Map{
				"success": false,
				"message": "Invalid print log ID",
			},
		)
	}

	item, err := c.PrintService.GetLogByID(uint(id))

	if err != nil {

		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(
				fiber.Map{
					"success": false,
					"message": "Print log not found",
				},
			)
		}

		return ctx.Status(fiber.StatusInternalServerError).JSON(
			fiber.Map{
				"success": false,
				"message": "Failed to retrieve print log",
			},
		)
	}

	return ctx.JSON(
		fiber.Map{
			"success": true,
			"data":    item,
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
	api.Get("/logs", printController.GetPrintLogs)
	api.Get("/logs/:id", printController.GetPrintLogByID)
}
