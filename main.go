package main

import (
	"fiber-app/config"
	"fiber-app/controllers"
	"fiber-app/middleware"
	"fiber-app/models"
	"fiber-app/routes"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

func main() {

	app := fiber.New()

	// Connect to database
	db, err := config.ConnectDB()
	if err != nil {
		log.Fatalf(" Failed to connect to database: %v", err)
	}

	// Auto migrate models
	err = db.AutoMigrate(&models.User{})
	if err != nil {
		log.Fatalf("Failed to auto migrate: %v", err)
	}

	// Initialize controllers
	authController := controllers.NewAuthController(db)
	userController := controllers.NewUserController(db)
	productController := controllers.NewProductController(db)
	customerController := controllers.NewCustomerController(db)
	supplierController := controllers.NewSupplierController(db)
	handlingController := controllers.NewHandlingController(db)
	transporterController := controllers.NewTransporterController(db)
	truckController := controllers.NewTruckController(db)
	originController := controllers.NewOriginController(db)

	// Setup CORS middleware
	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://127.0.0.1:3000", // Tentukan origin spesifik
		AllowMethods:     "GET,POST,PUT,DELETE,OPTIONS",
		AllowHeaders:     "Origin, Content-Type, Accept, Authorization",
		AllowCredentials: true, // Bisa digunakan dengan origin spesifik
	}))

	// Setup routes
	api := app.Group("/api")
	routes.SetupUserRoutes(app, userController)
	routes.SetupProductRoutes(app, productController)
	routes.SetupCustomerRoutes(app, customerController)
	routes.SetupSupplierRoutes(app, supplierController)
	routes.SetupInboundRoutes(app, controllers.NewInboundController(db))
	routes.SetupHandlingRoutes(app, handlingController)
	routes.SetupTransporterRoutes(app, transporterController)
	routes.SetupTruckRoutes(app, truckController)
	routes.SetupOriginRoutes(app, originController)

	// Route login (tidak perlu middleware auth)
	api.Post("/v1/login", authController.Login)
	api.Get("/v1/logout", authController.Logout)
	api.Get("/v1/isLoggedIn", middleware.AuthMiddleware, authController.IsLoggedIn)

	// Aplikasikan middleware auth ke semua route di bawah /api
	api.Use(middleware.AuthMiddleware)

	port := config.APP_PORT
	fmt.Println("🚀 Server berjalan di port " + port)

	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}

}
