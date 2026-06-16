package company_config_controller

import (
	"errors"
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/middleware"
	"fiber-app/models"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CompanyConfigController struct {
	DB *gorm.DB
}

func NewCompanyConfigController(db *gorm.DB) *CompanyConfigController {
	return &CompanyConfigController{DB: db}
}

// ─── Helper: get or create single config row ─────────────────────────────────

func (c *CompanyConfigController) getConfig() (*models.CompanyConfig, error) {
	var cfg models.CompanyConfig
	err := c.DB.First(&cfg).Error

	if errors.Is(err, gorm.ErrRecordNotFound) {
		// Buat default config kalau belum ada
		cfg = models.CompanyConfig{
			CompanyName:  "PT Yusen Logistics Interlink Indonesia",
			CompanyShort: "Yusen Logistics",
			AppName:      "YuTrackWMS",
			Tagline:      "Track Everything in Warehouse",
			LogoURL:      "/uploads/logo.png",
			PrimaryColor: "#041F5F",
			AccentColor:  "#1A50C8",
			LoginTheme:   "ThemeModern",
			LoginSlides:  models.JSONSlides{},
		}
		if err := c.DB.Create(&cfg).Error; err != nil {
			return nil, err
		}
		return &cfg, nil
	}

	if err != nil {
		return nil, err
	}

	return &cfg, nil
}

// ─── PUBLIC: Untuk login page & dokumen (no auth) ────────────────────────────

// GET /api/company-config
// Dipakai oleh: login page, header, footer, dokumen/laporan
func (c *CompanyConfigController) GetConfig(ctx *fiber.Ctx) error {
	cfg, err := c.getConfig()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    cfg,
	})
}

// GET /api/company-config/login
// Khusus login page — hanya return field yang dibutuhkan login
func (c *CompanyConfigController) GetLoginConfig(ctx *fiber.Ctx) error {
	cfg, err := c.getConfig()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Hanya expose field yang relevan untuk login page
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"app_name":      cfg.AppName,
			"tagline":       cfg.Tagline,
			"company_name":  cfg.CompanyName,
			"logo_url":      cfg.LogoURL,
			"primary_color": cfg.PrimaryColor,
			"accent_color":  cfg.AccentColor,
			"login_theme":   cfg.LoginTheme,
			"login_slides":  cfg.LoginSlides,
		},
	})
}

// ─── PROTECTED: Untuk admin (butuh auth) ─────────────────────────────────────

// PUT /api/admin/company-config
// Update semua config
func (c *CompanyConfigController) UpdateConfig(ctx *fiber.Ctx) error {
	cfg, err := c.getConfig()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	var input models.CompanyConfig
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Pakai map agar field kosong "" tetap tersimpan
	// (GORM skip zero value kalau pakai struct di Updates)
	updates := map[string]interface{}{
		"company_name":  input.CompanyName,
		"company_short": input.CompanyShort,
		"address":       input.Address,
		"phone":         input.Phone,
		"email":         input.Email,
		"website":       input.Website,
		"app_name":      input.AppName,
		"tagline":       input.Tagline,
		"logo_url":      input.LogoURL,
		"primary_color": input.PrimaryColor,
		"accent_color":  input.AccentColor,
		"login_theme":   input.LoginTheme,
		"login_slides":  input.LoginSlides,
		"updated_by":    int(ctx.Locals("userID").(float64)),
		"updated_at":    time.Now(),
	}

	if err := c.DB.Model(cfg).Updates(updates).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	// Reload setelah update
	updated, err := c.getConfig()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Company config updated successfully",
		"data":    updated,
	})
}

// PUT /api/admin/company-config/login-theme
// Update hanya theme login page
func (c *CompanyConfigController) UpdateLoginTheme(ctx *fiber.Ctx) error {
	cfg, err := c.getConfig()
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	var input struct {
		LoginTheme  string            `json:"login_theme"`
		LoginSlides models.JSONSlides `json:"login_slides"`
	}
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	updates := map[string]interface{}{
		"updated_by": int(ctx.Locals("userID").(float64)),
		"updated_at": time.Now(),
	}
	if input.LoginTheme != "" {
		updates["login_theme"] = input.LoginTheme
	}
	if input.LoginSlides != nil {
		updates["login_slides"] = input.LoginSlides
	}

	if err := c.DB.Model(cfg).Updates(updates).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Login theme updated successfully",
	})
}

// POST /api/admin/company-config/upload
// Upload logo atau slide image
// Field: file (multipart), type: "logo" | "slide"
func (c *CompanyConfigController) UploadImage(ctx *fiber.Ctx) error {
	uploadType := ctx.FormValue("type") // "logo" atau "slide"
	if uploadType != "logo" && uploadType != "slide" {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "type must be 'logo' or 'slide'",
		})
	}

	file, err := ctx.FormFile("file")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "file is required",
		})
	}

	// Validasi ekstensi
	ext := strings.ToLower(filepath.Ext(file.Filename))
	allowed := map[string]bool{".jpg": true, ".jpeg": true, ".png": true, ".webp": true}
	if !allowed[ext] {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"error":   "only jpg, jpeg, png, webp allowed",
		})
	}

	// Buat folder jika belum ada
	uploadDir := fmt.Sprintf("./public/uploads/company/%s", uploadType)
	if err := os.MkdirAll(uploadDir, os.ModePerm); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "failed to create upload directory",
		})
	}

	// Nama file unik pakai timestamp
	filename := fmt.Sprintf("%d%s", time.Now().UnixMilli(), ext)
	savePath := filepath.Join(uploadDir, filename)

	if err := ctx.SaveFile(file, savePath); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"error":   "failed to save file",
		})
	}

	// URL yang bisa diakses dari frontend
	fileURL := fmt.Sprintf("/uploads/company/%s/%s", uploadType, filename)

	// Kalau logo, langsung update di config
	if uploadType == "logo" {
		cfg, err := c.getConfig()
		if err == nil {
			c.DB.Model(cfg).Updates(map[string]interface{}{
				"logo_url":   fileURL,
				"updated_by": int(ctx.Locals("userID").(float64)),
				"updated_at": time.Now(),
			})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":  true,
		"message":  "File uploaded successfully",
		"file_url": fileURL,
	})
}

// ─── Routes ───────────────────────────────────────────────────────────────────

func SetupCompanyConfigRoutes(app *fiber.App) {

	// ── PUBLIC (no auth, tapi tetap inject DB) ────────────────────────────────
	publicCtrl := &CompanyConfigController{}
	public := app.Group(config.MAIN_ROUTES + "/company-config")
	public.Use(database.InjectDBMiddlewareFromEnv(publicCtrl))

	public.Get("/", publicCtrl.GetConfig)           // Semua config (untuk dokumen, dll)
	public.Get("/login", publicCtrl.GetLoginConfig) // Khusus login page

	// ── PROTECTED (butuh auth) ────────────────────────────────────────────────
	adminCtrl := &CompanyConfigController{}
	admin := app.Group(config.MAIN_ROUTES+"/admin/company-config", middleware.AuthMiddleware)
	admin.Use(database.InjectDBMiddleware(adminCtrl))

	admin.Put("/", adminCtrl.UpdateConfig)                // Update semua config
	admin.Put("/login-theme", adminCtrl.UpdateLoginTheme) // Update theme + slides saja
	admin.Post("/upload", adminCtrl.UploadImage)          // Upload logo / slide image
}
