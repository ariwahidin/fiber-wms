package main

import (
	"encoding/json"
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/controllers/customer_controller"
	"fiber-app/controllers/division_controller"
	"fiber-app/controllers/idgen"
	"fiber-app/controllers/inbound_controller"
	"fiber-app/controllers/inventory_controller"
	"fiber-app/controllers/item_controller"
	"fiber-app/controllers/location_controller"
	"fiber-app/controllers/origin_controller"
	"fiber-app/controllers/outbound_controller"
	"fiber-app/controllers/owner_controller"
	"fiber-app/controllers/qa_controller"
	"fiber-app/controllers/shopee_config_controller"
	"fiber-app/controllers/shopee_controller"
	"fiber-app/controllers/supplier_controller"
	"fiber-app/controllers/transporter_controller"
	"fiber-app/controllers/truck_controller"
	"fiber-app/controllers/vas_controller"
	"fiber-app/database"
	"fiber-app/middleware"
	"fiber-app/migration"
	"fiber-app/routes"
	scheduler "fiber-app/shceduler"
	"fiber-app/wms/master/owner"
	"fmt"
	"html/template"
	"log"
	_ "net/http/pprof"
	"os"
	"time"

	company_config_controller "fiber-app/controllers/company_config_controller"
	integration_ctrl "fiber-app/controllers/integration_ctrl"
	notification_ctrl "fiber-app/controllers/notification_ctrl"
	report_builder "fiber-app/controllers/report_builder"
	reportmailer "fiber-app/controllers/report_mailer"
	rpt_builder_ctrl "fiber-app/controllers/rpt_builder"
	integration_service "fiber-app/services/integration_service"
	rm_services "fiber-app/services/report_mailer"

	_ "fiber-app/docs"

	"github.com/gofiber/contrib/websocket"
	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/swagger"
)

// struct log
type AccessLog struct {
	Time      string        `json:"time"`
	IP        string        `json:"ip"`
	Method    string        `json:"method"`
	Path      string        `json:"path"`
	Status    int           `json:"status"`
	UserAgent string        `json:"user_agent"`
	Referer   string        `json:"referer"`
	Latency   time.Duration `json:"latency_ms"`
	UserID    int           `json:"user_id,omitempty"`
}

// channel buat log
var logChan = make(chan AccessLog, 100)

// @title           WMS API
// @version         1.0
// @description     API documentation for Warehouse Management System.
// @termsOfService  https://example.com/terms/

// @contact.name   WMS Support
// @contact.email  support@example.com

// @securityDefinitions.apikey BearerAuth
// @in header
// @name Authorization
// @description Enter your JWT token with the Bearer prefix. Example: Bearer eyJhbGciOi...
func main() {

	// buka file log
	file, err := os.OpenFile("access.jsonl", os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		log.Fatal("Gagal buka file log:", err)
	}
	defer file.Close()

	// worker untuk nulis log ke file dalam format JSON
	go func() {
		encoder := json.NewEncoder(file)
		for entry := range logChan {
			if err := encoder.Encode(entry); err != nil {
				log.Println("Gagal encode log:", err)
			}
		}
	}()

	config.LoadConfig()
	app := fiber.New()
	// app.Get("/docs/*", swagger.HandlerDefault)
	app.Get("/docs/*", swagger.New(swagger.Config{
		Title:                "WMS API Portal",
		URL:                  "/docs/doc.json",
		DeepLinking:          true,
		DocExpansion:         "list",
		Filter:               swagger.FilterConfig{Enabled: true},
		PersistAuthorization: true,
		CustomStyle: template.CSS(`
			:root {
				--wms-navy: #10243a;
				--wms-navy-light: #183651;
				--wms-green: #15966b;
				--wms-bg: #f3f6fa;
				--wms-text: #25364a;
				--wms-muted: #718096;
				--wms-border: #e2e8f0;
			}

			* {
				box-sizing: border-box;
			}

			body {
				margin: 0;
				background: var(--wms-bg);
				color: var(--wms-text);
				font-family: "Segoe UI", Inter, Arial, sans-serif;
				-webkit-font-smoothing: antialiased;
			}

			/* Top navigation */
			.swagger-ui .topbar {
				position: sticky;
				top: 0;
				z-index: 100;
				padding: 16px 0;
				background: var(--wms-navy);
				border-bottom: 3px solid var(--wms-green);
				box-shadow: 0 4px 16px rgba(16, 36, 58, .15);
			}

			.swagger-ui .topbar .wrapper {
				max-width: 1440px;
				padding: 0 32px;
			}

			.swagger-ui .topbar-wrapper {
				display: flex;
				align-items: center;
				min-height: 38px;
			}

			.swagger-ui .topbar-wrapper .link {
				position: relative;
				display: flex;
				align-items: center;
				width: 280px;
				height: 38px;
				visibility: hidden;
			}

			.swagger-ui .topbar-wrapper .link::after {
				content: "WMS  /  DEVELOPER PORTAL";
				position: absolute;
				inset: 0;
				display: flex;
				align-items: center;
				visibility: visible;
				color: #ffffff;
				font-size: 17px;
				font-weight: 750;
				letter-spacing: .7px;
				white-space: nowrap;
			}

			.swagger-ui .topbar .download-url-wrapper {
				display: flex;
				align-items: center;
				flex: 1;
				gap: 10px;
			}

			.swagger-ui .topbar input[type=text] {
				height: 40px;
				padding: 0 14px;
				border: 1px solid #40566d;
				border-radius: 7px;
				background: #ffffff;
				color: var(--wms-text);
				box-shadow: none;
			}

			.swagger-ui .topbar .download-url-button {
				height: 40px;
				padding: 0 22px;
				border-radius: 7px;
				background: var(--wms-green);
				font-weight: 650;
			}

			/* Main content width */
			.swagger-ui .wrapper {
				max-width: 1440px;
				padding-right: 32px;
				padding-left: 32px;
			}

			/* API introduction */
			.swagger-ui .information-container {
				padding-top: 32px;
			}

			.swagger-ui .info {
				margin: 0 0 26px;
			}

			.swagger-ui .info .title {
				margin: 0 0 12px;
				color: var(--wms-navy);
				font-size: 32px;
				font-weight: 750;
				letter-spacing: -.7px;
			}

			.swagger-ui .info p,
			.swagger-ui .info li {
				color: #536579;
				font-size: 14px;
				line-height: 1.8;
			}

			.swagger-ui .info a {
				color: var(--wms-green);
			}

			.swagger-ui .info .title small {
				top: -4px;
				margin-left: 8px;
				padding: 4px 8px;
				border-radius: 5px;
				background: #e7f5ee;
				color: #11734f;
				font-size: 11px;
				font-weight: 700;
			}

			/* Authorization panel */
			.swagger-ui .scheme-container {
				margin: 0 0 28px;
				padding: 20px 24px;
				border: 1px solid var(--wms-border);
				border-radius: 12px;
				background: #ffffff;
				box-shadow: 0 3px 12px rgba(16, 36, 58, .035);
			}

			.swagger-ui .btn.authorize {
				min-height: 40px;
				padding: 0 20px;
				border: 1px solid var(--wms-green);
				border-radius: 7px;
				background: #ffffff;
				color: var(--wms-green);
				font-weight: 650;
			}

			.swagger-ui .btn.authorize:hover {
				background: #eaf7f1;
			}

			/* Tag headings */
			.swagger-ui .opblock-tag-section {
				margin-bottom: 24px;
			}

			.swagger-ui .opblock-tag {
				margin: 0;
				padding: 17px 4px;
				border-bottom: 1px solid #dce4ed;
				color: var(--wms-navy);
				font-size: 19px;
				font-weight: 750;
				letter-spacing: -.2px;
			}

			.swagger-ui .opblock-tag small {
				color: var(--wms-muted);
				font-size: 12px;
				font-weight: 400;
			}

			/* Endpoint cards */
			.swagger-ui .opblock {
				margin: 10px 0;
				overflow: hidden;
				border: 1px solid var(--wms-border);
				border-radius: 9px;
				background: #ffffff;
				box-shadow: 0 2px 5px rgba(16, 36, 58, .025);
				transition: box-shadow .15s ease, transform .15s ease;
			}

			.swagger-ui .opblock:hover {
				box-shadow: 0 5px 14px rgba(16, 36, 58, .07);
			}

			.swagger-ui .opblock .opblock-summary {
				padding: 5px 12px;
			}

			.swagger-ui .opblock .opblock-summary-method {
				min-width: 76px;
				padding: 8px 12px;
				border-radius: 5px;
				font-size: 11px;
				font-weight: 750;
				letter-spacing: .4px;
				text-shadow: none;
				box-shadow: none;
			}

			.swagger-ui .opblock .opblock-summary-path {
				color: var(--wms-navy);
				font-size: 13px;
				font-weight: 700;
			}

			.swagger-ui .opblock .opblock-summary-description {
				color: #66778a;
				font-size: 12px;
			}

			.swagger-ui .opblock .opblock-section-header {
				background: #f7f9fc;
				box-shadow: none;
				border-top: 1px solid var(--wms-border);
			}

			.swagger-ui .opblock-body {
				background: #ffffff;
			}

			/* Request and response panels */
			.swagger-ui .model-box,
			.swagger-ui .models {
				border-radius: 8px;
			}

			.swagger-ui section.models {
				margin-top: 32px;
				border: 1px solid var(--wms-border);
				border-radius: 10px;
				background: #ffffff;
			}

			.swagger-ui section.models h4 {
				color: var(--wms-navy);
				font-size: 15px;
			}

			.swagger-ui .model-title {
				color: var(--wms-navy);
			}

			.swagger-ui table thead tr th,
			.swagger-ui table thead tr td {
				color: var(--wms-navy);
				font-size: 12px;
				font-weight: 700;
			}

			.swagger-ui .btn.execute {
				min-height: 40px;
				border: none;
				border-radius: 6px;
				background: var(--wms-green);
				font-weight: 700;
				box-shadow: none;
			}

			.swagger-ui .btn.execute:hover {
				background: #107a57;
			}

			.swagger-ui input,
			.swagger-ui select,
			.swagger-ui textarea {
				border: 1px solid #d7e0ea;
				border-radius: 6px;
				box-shadow: none;
			}

			.swagger-ui input:focus,
			.swagger-ui select:focus,
			.swagger-ui textarea:focus {
				border-color: var(--wms-green);
				outline: 2px solid rgba(21, 150, 107, .12);
			}

			.swagger-ui .response-col_status {
				color: var(--wms-navy);
				font-weight: 750;
			}

			/* Mobile */
			@media (max-width: 768px) {
				.swagger-ui .topbar .wrapper,
				.swagger-ui .wrapper {
					padding-right: 16px;
					padding-left: 16px;
				}

				.swagger-ui .topbar-wrapper {
					flex-wrap: wrap;
					gap: 12px;
				}

				.swagger-ui .topbar-wrapper .link {
					width: 100%;
				}

				.swagger-ui .topbar .download-url-wrapper {
					width: 100%;
				}

				.swagger-ui .info .title {
					font-size: 25px;
				}

				.swagger-ui .scheme-container {
					padding: 14px;
				}

				.swagger-ui .opblock .opblock-summary {
					padding: 5px;
				}

				.swagger-ui .opblock .opblock-summary-method {
					min-width: 55px;
					padding: 7px;
				}

				.swagger-ui .opblock .opblock-summary-description {
					display: none;
				}
			}
		`),
	}))
	config.SetupCORS(app)

	// middleware logger custom
	app.Use(func(c *fiber.Ctx) error {
		start := time.Now()
		err := c.Next()

		// Ambil userID dari Locals (float64 → int)
		var userID int
		if uidVal := c.Locals("userID"); uidVal != nil {
			if uidFloat, ok := uidVal.(float64); ok {
				userID = int(uidFloat)
			}
		}

		entry := AccessLog{
			Time:      start.Format(time.RFC3339),
			IP:        c.IP(),
			Method:    c.Method(),
			Path:      c.Path(),
			Status:    c.Response().StatusCode(),
			UserAgent: c.Get("User-Agent"),
			Referer:   c.Get("Referer"),
			Latency:   time.Since(start) / time.Millisecond, // simpan dalam ms
			UserID:    userID,
		}

		// kirim ke channel
		select {
		case logChan <- entry:
		default:
			// kalau channel penuh, buang (biar request tetap jalan)
		}

		return err
	})

	// Pastikan database ada
	database.EnsureDatabaseExists(config.DBUnit)

	if err != nil {
		log.Fatalf(" Failed to connect to database: %v", err)
	}

	unitDB, err := database.OpenDatabaseConnection(config.DBUnit)

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	} else {
		fmt.Println("✅ Connected to unit database successfully")
	}

	// ── Read-Only DB untuk Report Mailer ──────────────────────────────
	reportDB, err := database.OpenReadOnlyDB()
	if err != nil {
		log.Printf("⚠️  Read-only DB tidak tersedia: %v", err)
		log.Printf("⚠️  Report Mailer akan menggunakan koneksi utama (kurang aman)")
		reportDB = unitDB // fallback ke koneksi utama
	} else {
		fmt.Println("✅ Connected to read-only report database")
	}

	err = migration.MigrateBusinessUnit(unitDB)
	if err != nil {
		log.Fatalf("Failed to auto migrate unit database: %v", err)
	} else {
		fmt.Println("✅ Migrated unit database successfully")
	}

	idgen.Init()
	idgen.AutoGenerateSnowflakeID(unitDB)
	database.RunSeeders(unitDB)
	owner.SeedOwner(unitDB)

	// Setup CORS middleware
	config.SetupCORS(app)
	app.Static("/uploads", "./public/uploads")
	app.Use(middleware.ConsoleLogger())

	mainRoutes := os.Getenv("MAIN_ROUTES") // /api/v1
	app.Get(mainRoutes+"/health", func(c *fiber.Ctx) error {
		// ping DB
		sqlDB, err := unitDB.DB() // db = *gorm.DB kamu
		if err != nil || sqlDB.Ping() != nil {
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"status":  "degraded",
				"message": "Database unreachable",
				"services": fiber.Map{
					"api": "ok",
					"db":  "error",
				},
			})
		}

		return c.Status(fiber.StatusOK).JSON(fiber.Map{
			"status":  "ok",
			"message": "Fiber WMS is healthy",
			"services": fiber.Map{
				"api": "ok",
				"db":  "ok",
			},
		})
	})

	app.Use(mainRoutes+"/ws", func(c *fiber.Ctx) error {
		if websocket.IsWebSocketUpgrade(c) {
			return c.Next()
		}
		return fiber.ErrUpgradeRequired
	})
	app.Get(mainRoutes+"/ws/events", websocket.New(controllers.RealtimeWSHandler()))

	supplier_controller.SetupSupplierRoutes(app)
	item_controller.SetupProductRoutes(app)
	customer_controller.SetupCustomerRoutes(app)
	transporter_controller.SetupTransporterRoutes(app)
	truck_controller.SetupTruckRoutes(app)
	origin_controller.SetupOriginRoutes(app)
	location_controller.SetupLocationRoutes(app)
	vas_controller.SetupVasRoutes(app)
	qa_controller.SetupQaRoutes(app)
	owner_controller.SetupOwnerRoutes(app)
	company_config_controller.SetupCompanyConfigRoutes(app)
	division_controller.SetupDivisionRoutes(app)
	inventory_controller.SetupInventoryRoutes(app)
	inbound_controller.SetupInboundRoutes(app)
	inbound_controller.SetupIntegrationRoutes(app)
	outbound_controller.SetupOutboundRoutes(app)
	shopee_config_controller.SetupShopeeConfigRoutes(app)

	routes.SetupAuthRoutes(app)
	routes.SetupDashboardRoutes(app)
	routes.SetupHandlingRoutes(app)
	routes.SetupUserRoutes(app)
	routes.SetupMenuRoutes(app)
	routes.SetupWarehouseRoutes(app)
	routes.SetupMobileInboundRoutes(app)
	routes.SetupMobileOutboundRoutes(app)
	routes.SetupMobilePackingRoutes(app)
	routes.SetupShippingRoutes(app)
	routes.SetupMobileInventoryRoutes(app)
	owner.SetupOwnerRoutes(app)
	routes.SetupStockTakeRoutes(app)
	routes.SetupIntegrationRoutes(app)
	routes.SetupMasterCartonRoutes(app)

	rm_services.InitScheduler(unitDB, reportDB)
	reportmailer.SetupEmailConfigRoutes(app, unitDB)
	reportmailer.SetupReportRoutes(app, unitDB, reportDB)
	reportmailer.SetupScheduleRoutes(app, unitDB, reportDB)
	reportmailer.SetupSendHistoryRoutes(app, unitDB)
	reportmailer.SetupReportQueryRoutes(app, unitDB, reportDB)
	notification_ctrl.SetupNotificationRoutes(app, unitDB)
	integration_ctrl.SetupIntegrationRoutes(app, unitDB, reportDB)
	integration_service.InitIntegrationScheduler(unitDB, reportDB)
	report_builder.SetupReportBuilderRoutes(app, unitDB)
	rpt_builder_ctrl.SetupRptBuilderRoutes(app, unitDB, reportDB)

	shopee_controller.SetupShopeeRoutes(app) // tanpa middleware auth karena Shopee yang akses
	port := config.APP_PORT
	fmt.Println("🚀 Server berjalan di port " + port)
	fmt.Println("N8N token configured:", config.N8NIntegrationToken != "")

	go func() {
		time.Sleep(3 * time.Second)
		if err := scheduler.StartShopeeScheduler(unitDB, reportDB); err != nil {
			log.Printf("[Shopee] Auto-start skipped: %v", err)
		}
	}()

	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}

}
