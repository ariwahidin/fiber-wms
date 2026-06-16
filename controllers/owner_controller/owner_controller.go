package owner_controller

import (
	"errors"
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"
	"fiber-app/models"
	"fiber-app/services/qr_parser"
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

// ─── Owner CRUD ───────────────────────────────────────────────────────────────

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

// ─── QR Config Helpers ────────────────────────────────────────────────────────

func getDefaultQRConfig() models.OwnerQRConfig {
	return models.OwnerQRConfig{
		PatternType:   "bracket_kv",
		Delimiter:     "",
		MfgDateFormat: "YYYYMMDD",
		QtyStripUnit:  true,
		FieldMap: models.QRFieldMap{
			SKU:              "SKU",
			EAN:              "EAN",
			Serial:           "SERIAL",
			CartonSerial:     "CARTON_SERIAL",
			Batch:            "BATCH",
			MfgDate:          "MFG_DATE",
			QtyPerCarton:     "QTY_PER_CARTON",
			InnerSerialStart: "INNER_SERIAL_START",
			InnerSerialEnd:   "INNER_SERIAL_END",
			Product:          "PRODUCT",
			Brand:            "BRAND",
			Model:            "MODEL",
		},
	}
}

func getConfigByOwnerCode(db *gorm.DB, ownerCode string) (models.OwnerQRConfig, error) {
	var owner models.Owner
	if err := db.Where("code = ?", ownerCode).First(&owner).Error; err != nil {
		return models.OwnerQRConfig{}, err
	}

	var cfg models.OwnerQRConfig
	if err := db.Where("owner_id = ?", owner.ID).First(&cfg).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return getDefaultQRConfig(), nil
		}
		return models.OwnerQRConfig{}, err
	}
	return cfg, nil
}

func modelConfigToParserConfig(m models.OwnerQRConfig) qr_parser.Config {
	return qr_parser.Config{
		PatternType:   m.PatternType,
		Delimiter:     m.Delimiter,
		MfgDateFormat: m.MfgDateFormat,
		QtyStripUnit:  m.QtyStripUnit,
		FieldMap: qr_parser.QRFieldMap{
			SKU:              m.FieldMap.SKU,
			EAN:              m.FieldMap.EAN,
			Serial:           m.FieldMap.Serial,
			CartonSerial:     m.FieldMap.CartonSerial,
			Batch:            m.FieldMap.Batch,
			MfgDate:          m.FieldMap.MfgDate,
			QtyPerCarton:     m.FieldMap.QtyPerCarton,
			InnerSerialStart: m.FieldMap.InnerSerialStart,
			InnerSerialEnd:   m.FieldMap.InnerSerialEnd,
			Product:          m.FieldMap.Product,
			Brand:            m.FieldMap.Brand,
			Model:            m.FieldMap.Model,
		},
	}
}

// ─── QR Config CRUD ───────────────────────────────────────────────────────────

func (c *OwnerController) GetQRConfig(ctx *fiber.Ctx) error {
	ownerCode := ctx.Query("owner")
	if ownerCode == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "owner code is required"})
	}

	cfg, err := getConfigByOwnerCode(c.DB, ownerCode)
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Owner not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": cfg})
}

func (c *OwnerController) UpsertQRConfig(ctx *fiber.Ctx) error {
	ownerID, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid owner ID"})
	}

	var input models.OwnerQRConfig
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	input.OwnerID = uint(ownerID)

	var existing models.OwnerQRConfig
	err = c.DB.Where("owner_id = ?", ownerID).First(&existing).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		if err := c.DB.Create(&input).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	} else {
		if err := c.DB.Model(&existing).Updates(&input).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
		input = existing
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": input})
}

// ─── QR Parse Endpoint ────────────────────────────────────────────────────────

func (c *OwnerController) ParseQR(ctx *fiber.Ctx) error {
	var body struct {
		OwnerCode string `json:"owner_code"`
		Raw       string `json:"raw"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	if body.Raw == "" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "raw QR string is required"})
	}

	cfg, err := getConfigByOwnerCode(c.DB, body.OwnerCode)
	if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	parserCfg := modelConfigToParserConfig(cfg)
	result, err := qr_parser.Parse(body.Raw, parserCfg)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "data": result})
}

// ─── Routes ───────────────────────────────────────────────────────────────────

func SetupOwnerRoutes(app *fiber.App) {
	api := app.Group(config.MAIN_ROUTES+"/owners", middleware.AuthMiddleware)
	ownerController := &OwnerController{}
	api.Use(database.InjectDBMiddleware(ownerController))

	// Owner CRUD
	api.Get("/", ownerController.GetAllOwner)
	api.Post("/", ownerController.CreateOwner)
	api.Put("/:id", ownerController.UpdateOwner)
	api.Get("/user/", ownerController.GetOwnerByUserID)

	// QR Config
	api.Get("/qr-config", ownerController.GetQRConfig)        // GET ?owner=YUWELL
	api.Put("/:id/qr-config", ownerController.UpsertQRConfig) // PUT /owners/1/qr-config
	api.Post("/qr-config/parse", ownerController.ParseQR)     // POST /owners/qr-config/parse
}
