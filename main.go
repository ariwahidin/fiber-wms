package main

import (
	"encoding/json"
	"fiber-app/config"
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
	"log"
	_ "net/http/pprof"
	"os"
	"time"

	integration_ctrl "fiber-app/controllers/integration_ctrl"
	notification_ctrl "fiber-app/controllers/notification_ctrl"
	report_builder "fiber-app/controllers/report_builder"
	reportmailer "fiber-app/controllers/report_mailer"
	rpt_builder_ctrl "fiber-app/controllers/rpt_builder"
	integration_service "fiber-app/services/integration_service"
	rm_services "fiber-app/services/report_mailer"

	"github.com/gofiber/fiber/v2"
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
	division_controller.SetupDivisionRoutes(app)
	inventory_controller.SetupInventoryRoutes(app)
	inbound_controller.SetupInboundRoutes(app)
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
