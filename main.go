package main

import (
	"fiber-app/config"
	"fiber-app/controllers/idgen"
	"fiber-app/database"
	"fiber-app/middleware"
	"fiber-app/migration"
	"fiber-app/routes"
	"fiber-app/wms/master/owner"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
)

// Model untuk Receiving
// Struktur untuk menyimpan file yang telah diproses

func main() {

	app := fiber.New()

	// Pastikan database ada
	database.EnsureDatabaseExists(config.DBName)
	database.EnsureDatabaseExists(config.DBUnit)

	// Connect to database
	mainDB, err := database.OpenMasterDB()

	if err != nil {
		log.Fatalf(" Failed to connect to database: %v", err)
	}

	// Auto migrate models
	err = migration.Migrate(mainDB)
	if err != nil {
		log.Fatalf("Failed to auto migrate: %v", err)
	}

	unitDB, err := database.OpenDatabaseConnection(config.DBUnit)

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	err = migration.MigrateBusinessUnit(unitDB)
	if err != nil {
		log.Fatalf("Failed to auto migrate unit database: %v", err)
	}

	database.SeedUnit(mainDB)

	idgen.Init()
	idgen.AutoGenerateSnowflakeID(unitDB)
	database.RunSeeders(unitDB)
	owner.SeedOwner(unitDB)

	// checkUnprocessedFiles(db)

	// Initialize controllers

	// authMiddleware := middleware.NewAuthMiddleware(db)
	// authController := controllers.NewAuthController(db)

	// customerController := controllers.NewCustomerController(db)
	// handlingController := controllers.NewHandlingController(db)
	// transporterController := controllers.NewTransporterController(db)
	// truckController := controllers.NewTruckController(db)
	// originController := controllers.NewOriginController(db)
	// RfInboundController := controllers.NewRfInboundController(db)

	// Setup CORS middleware
	config.SetupCORS(app)

	// Setup routes
	// api := app.Group("/api")
	// guestApi := app.Group("/guest/api")
	// Aplikasikan middleware auth ke semua route di bawah /api

	routes.SetupAuthRoutes(app)
	routes.SetupDashboardRoutes(app)
	routes.SetupProductRoutes(app)
	routes.SetupCategoryRoutes(app)
	routes.SetupSupplierRoutes(app)
	routes.SetupCustomerRoutes(app)
	routes.SetupTransporterRoutes(app)
	routes.SetupTruckRoutes(app)
	routes.SetupOriginRoutes(app)
	routes.SetupHandlingRoutes(app)
	routes.SetupUserRoutes(app)
	routes.SetupMenuRoutes(app)
	routes.SetupInboundRoutes(app)
	routes.SetupWarehouseRoutes(app)
	routes.SetupOutboundRoutes(app)
	routes.SetupInventoryRoutes(app)
	routes.SetupMobileInboundRoutes(app)
	routes.SetupMobileOutboundRoutes(app)
	routes.SetupMobilePackingRoutes(app)
	routes.SetupShippingRoutes(app)
	routes.SetupMobileInventoryRoutes(app)
	owner.SetupOwnerRoutes(app)
	routes.SetupStockTakeRoutes(app)
	routes.SetupLocationRoutes(app)

	// routes.SetupRfInboundRoutes(app, RfInboundController)
	// routes.SetupOutboundRoutes(app, db)
	// routes.SetupStockTakeRoutes(app, db)
	// routes.SetupRfOutboundRoutes(app, db)

	// routes.SetupMobileShippingGuestRoutes(app, mobiles.NewShippingGuestController(db))
	// Route login (tidak perlu middleware auth)

	// api.Post(config.MAIN_ROUTES+"/login", authController.Login)
	// api.Get(config.MAIN_ROUTES+"/logout", authController.Logout)
	// api.Get(config.MAIN_ROUTES+"/isLoggedIn", middleware.AuthMiddleware, authController.IsLoggedIn)
	api := app.Group(config.MAIN_ROUTES)
	api.Post("/configurations/create-db", middleware.AuthMiddleware, database.CreateDatabase)
	api.Post("/configurations/get-all-table", middleware.AuthMiddleware, database.GetAllTables())
	api.Get("/configurations/get-all-bu", database.GetAllBusinessUnit)
	api.Post("/configurations/db-migrate", database.MigrateDB)

	port := config.APP_PORT
	fmt.Println("🚀 Server berjalan di port " + port)

	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}

}
