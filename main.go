package main

import (
	"fiber-app/config"
	"fiber-app/controllers/configurations"
	"fiber-app/controllers/idgen"
	"fiber-app/database"
	"fiber-app/middleware"
	"fiber-app/routes"
	"fmt"
	"log"

	"github.com/gofiber/fiber/v2"
)

// Model untuk Receiving
// Struktur untuk menyimpan file yang telah diproses

func main() {

	app := fiber.New()

	idgen.Init()

	// Pastikan database ada
	configurations.EnsureDatabaseExists(config.DBName)
	configurations.EnsureDatabaseExists(config.DBUnit)

	// Connect to database
	mainDB, err := configurations.OpenMasterDB()

	if err != nil {
		log.Fatalf(" Failed to connect to database: %v", err)
	}

	// Auto migrate models
	err = database.Migrate(mainDB)
	if err != nil {
		log.Fatalf("Failed to auto migrate: %v", err)
	}

	unitDB, err := configurations.OpenDatabaseConnection(config.DBUnit)

	if err != nil {
		log.Fatalf("Failed to connect to database: %v", err)
	}

	err = database.MigrateBusinessUnit(unitDB)
	if err != nil {
		log.Fatalf("Failed to auto migrate unit database: %v", err)
	}

	database.SeedUnit(mainDB)
	database.RunSeeders(unitDB)

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
	api.Post("/configurations/create-db", middleware.AuthMiddleware, configurations.CreateDatabase)
	api.Post("/configurations/get-all-table", middleware.AuthMiddleware, configurations.GetAllTables())
	api.Get("/configurations/get-all-bu", configurations.GetAllBusinessUnit)
	api.Post("/configurations/db-migrate", configurations.MigrateDB)

	// api.Use(middleware.AuthMiddleware)

	// Print all registered routes
	// for _, route := range app.Stack() {
	// 	for _, r := range route {
	// 		fmt.Printf("%s %s\n", r.Method, r.Path)
	// 	}
	// }

	port := config.APP_PORT
	fmt.Println("🚀 Server berjalan di port " + port)

	if err := app.Listen(":" + port); err != nil {
		log.Fatal(err)
	}

}

// func checkUnprocessedFiles(db *gorm.DB) {

// 	fmt.Println("📂 Memproses file unprocessed")

// 	unproccessed_folder := "D:\\Golang Project\\backend-wms\\sap-data\\unprocessed\\"
// 	files, err := filepath.Glob(unproccessed_folder + "*.csv")
// 	if err != nil {
// 		log.Fatal("❌ Gagal membaca folder:", err)
// 	}

// 	for _, file := range files {
// 		fmt.Println("📂 Memproses file:", file)
// 		processFile(db, file)
// 	}

// }

// func processReceivingCSV(db *gorm.DB, filename string) {
// 	file, err := os.Open(filename)
// 	if err != nil {
// 		fmt.Println("❌ Gagal membuka file:", err)
// 		return
// 	}

// 	reader := csv.NewReader(file)
// 	records, err := reader.ReadAll()
// 	if err != nil {
// 		fmt.Println("❌ Gagal membaca file CSV:", err)
// 		file.Close() // Pastikan file ditutup jika gagal membaca
// 		return
// 	}

// 	file.Close() // Tutup file sebelum dipindahkan!

// 	var inboundDetails []models.InboundDetail

// 	for i, record := range records {
// 		if i == 0 {
// 			continue // Skip header
// 		}

// 		var supplier models.Supplier
// 		var product models.Product
// 		var wh_code models.WarehouseCode

// 		quantity, _ := strconv.Atoi(record[4])

// 		data := models.Receiving{
// 			InboundID:     record[0],
// 			PO_Number:     record[1],
// 			Material:      record[2],
// 			Description:   record[3],
// 			Quantity:      quantity,
// 			UOM:           record[5],
// 			Warehouse:     record[6],
// 			ReceivingDate: record[7],
// 			Supplier:      record[8],
// 			Filename:      filepath.Base(filename),
// 		}

// 		// Supplier
// 		db.Where("supplier_code = ?", data.Supplier).First(&supplier)
// 		if supplier.ID == 0 {
// 			supplier = models.Supplier{
// 				SupplierCode: data.Supplier,
// 				SupplierName: data.Supplier,
// 			}
// 			db.Create(&supplier)
// 		}

// 		// Product
// 		db.Where("item_code = ?", data.Material).First(&product)
// 		if product.ID == 0 {
// 			product = models.Product{
// 				ItemCode: data.Material,
// 				GMC:      data.Material,
// 				Barcode:  data.Material,
// 				ItemName: data.Description,
// 			}
// 			db.Create(&product)
// 		} else {
// 			fmt.Println("📌 Produk sudah ada:", product.ItemCode)
// 		}

// 		// Warehouse Code
// 		db.Where("warehouse_code = ?", data.Warehouse).First(&wh_code)
// 		if wh_code.ID == 0 {
// 			wh_code = models.WarehouseCode{
// 				WarehouseCode: data.Warehouse,
// 			}
// 			db.Create(&wh_code)
// 		}

// 		db.Create(&data)

// 		inboundDetails = append(inboundDetails, models.InboundDetail{
// 			InboundId: 0,
// 			ItemID:    int(product.ID),
// 			ItemCode:  product.ItemCode,
// 			Quantity:  data.Quantity,
// 			Uom:       data.UOM,
// 			WhsCode:   data.Warehouse,
// 			RecDate:   data.ReceivingDate,
// 			Status:    "open",
// 		})

// 		fmt.Println("✅ Data Inserted:", data)
// 	}

// 	fmt.Println("✅ Data Detail:", inboundDetails)

// 	// Inisialisasi repository
// 	repo := repositories.NewInboundRepository(db)

// 	// Membuat inbound baru
// 	inboundHeader := models.InboundHeader{}
// 	inboundHeader.InboundDate = time.Now().Format("2006-01-02 15:04:05")
// 	inboundHeader.SupplierCode = records[1][8]
// 	inboundHeader.PoNo = records[1][1]
// 	newInbound, err := repo.CreateInboundOpen(inboundHeader, inboundDetails)
// 	if err != nil {
// 		fmt.Println("❌ Gagal membuat inbound:", err)
// 	}

// 	// **Tutup file sebelum pindah**
// 	time.Sleep(1 * time.Second) // Tunggu sebentar untuk memastikan file tidak terkunci

// 	const processedFolder = "D:\\Golang Project\\backend-wms\\sap-data\\processed"

// 	// Pastikan folder `processed` ada
// 	if _, err := os.Stat(processedFolder); os.IsNotExist(err) {
// 		err := os.MkdirAll(processedFolder, os.ModePerm)
// 		if err != nil {
// 			log.Fatalf("❌ Gagal membuat folder processed: %v", err)
// 		}
// 	}

// 	// Pindahkan file ke folder processed
// 	processedFilePath := filepath.Join(processedFolder, filepath.Base(filename))

// 	err = os.Rename(filename, processedFilePath)
// 	if err != nil {
// 		fmt.Println("⚠️  Rename gagal, coba metode copy & delete...")
// 		err = copyAndDeleteFile(filename, processedFilePath)
// 		if err != nil {
// 			log.Fatalf("❌ Gagal memindahkan file ke folder processed: %v", err)
// 		}
// 	}

// 	fmt.Println("✅ Inbound Created:", newInbound)
// }

// // **Metode alternatif untuk memindahkan file jika rename gagal**
// func copyAndDeleteFile(src, dst string) error {
// 	sourceFile, err := os.Open(src)
// 	if err != nil {
// 		return err
// 	}
// 	defer sourceFile.Close()

// 	destinationFile, err := os.Create(dst)
// 	if err != nil {
// 		return err
// 	}
// 	defer destinationFile.Close()

// 	_, err = io.Copy(destinationFile, sourceFile)
// 	if err != nil {
// 		return err
// 	}

// 	return os.Remove(src) // Hapus file lama setelah berhasil disalin
// }

// func processReceivingCSV(db *gorm.DB, filename string) {
// 	file, err := os.Open(filename)
// 	if err != nil {
// 		fmt.Println("❌ Gagal membuka file:", err)
// 		return
// 	}
// 	defer file.Close()

// 	reader := csv.NewReader(file)
// 	records, err := reader.ReadAll()
// 	if err != nil {
// 		fmt.Println("❌ Gagal membaca file CSV:", err)
// 		return
// 	}

// 	var inboundDetails []models.InboundDetail

// 	for i, record := range records {
// 		if i == 0 {
// 			continue // Skip header
// 		}

// 		var supplier models.Supplier
// 		var product models.Product
// 		var wh_code models.WarehouseCode

// 		quantity, _ := strconv.Atoi(record[4])

// 		data := Receiving{
// 			InboundID:     record[0],
// 			PO_Number:     record[1],
// 			Material:      record[2],
// 			Description:   record[3],
// 			Quantity:      quantity,
// 			UOM:           record[5],
// 			Warehouse:     record[6],
// 			ReceivingDate: record[7],
// 			Supplier:      record[8],
// 			Filename:      filepath.Base(filename),
// 		}

// 		//Supplier
// 		db.Where("supplier_code = ?", data.Supplier).First(&supplier)
// 		if supplier.ID == 0 {
// 			supplier = models.Supplier{
// 				SupplierCode: data.Supplier,
// 				SupplierName: data.Supplier,
// 			}
// 			db.Create(&supplier)
// 		}

// 		//Product
// 		// Product
// 		db.Where("item_code = ?", data.Material).First(&product)
// 		if product.ID == 0 {
// 			product = models.Product{
// 				ItemCode: data.Material,
// 				GMC:      data.Material,
// 				Barcode:  data.Material,
// 				ItemName: data.Description,
// 			}
// 			db.Create(&product)
// 		} else {
// 			fmt.Println("📌 Produk sudah ada:", product.ItemCode)
// 		}

// 		// Warehouse Code
// 		db.Where("warehouse_code = ?", data.Warehouse).First(&wh_code)
// 		if wh_code.ID == 0 {
// 			wh_code = models.WarehouseCode{
// 				WarehouseCode: data.Warehouse,
// 			}
// 			db.Create(&wh_code)
// 		}

// 		db.Create(&data)

// 		inboundDetails = append(inboundDetails, models.InboundDetail{
// 			InboundId: 0,
// 			ItemID:    int(product.ID),
// 			ItemCode:  product.ItemCode,
// 			Quantity:  data.Quantity,
// 			Uom:       data.UOM,
// 			WhsCode:   data.Warehouse,
// 			RecDate:   data.ReceivingDate,
// 			Status:    "open",
// 		})

// 		fmt.Println("✅ Data Inserted:", data)
// 	}

// 	fmt.Println("✅ Data Detail:", inboundDetails)

// 	// Inisialisasi repository
// 	repo := repositories.NewInboundRepository(db)

// 	// Membuat inbound baru
// 	inboundHeader := models.InboundHeader{}
// 	inboundHeader.InboundDate = time.Now().Format("2006-01-02 15:04:05")
// 	inboundHeader.SupplierCode = records[1][8]
// 	inboundHeader.PoNo = records[1][1]
// 	newInbound, err := repo.CreateInboundOpen(inboundHeader, inboundDetails)
// 	if err != nil {
// 		// log.Fatal("Gagal membuat inbound:", err)
// 		fmt.Println("❌ Gagal membuat inbound:", err)
// 	}

// 	defer file.Close()

// 	const processedFolder = "D:\\Golang Project\\backend-wms\\sap-data\\processed\\"

// 	// Pastikan folder `processed` ada
// 	if _, err := os.Stat(processedFolder); os.IsNotExist(err) {
// 		err := os.MkdirAll(processedFolder, os.ModePerm)
// 		if err != nil {
// 			log.Fatalf("❌ Gagal membuat folder processed: %v", err)
// 		}
// 	}

// 	// Pindahkan file ke folder processed
// 	processedFilePath := filepath.Join(processedFolder, filepath.Base(filename))
// 	err = os.Rename(filename, processedFilePath)
// 	if err != nil {
// 		log.Fatalf("❌ Gagal memindahkan file ke folder processed: %v", err)
// 	}

// 	fmt.Println("✅ Inbound Created:", newInbound)
// }

// func processFile(db *gorm.DB, filename string) {
// 	// Ambil hanya nama file tanpa path
// 	fileNameOnly := filepath.Base(filename)

// 	// Cek apakah file sudah pernah diproses
// 	var existingFile models.FileLog
// 	if err := db.Where("filename = ?", filepath.Base(filename)).First(&existingFile).Error; err == nil {
// 		log.Println("⚠️ File sudah pernah diproses, skip:", filename)
// 		return
// 	}

// 	info, err := os.Stat(filename)
// 	if err != nil {
// 		fmt.Println("❌ Gagal membaca file:", err)
// 		return
// 	}

// 	modifiedTime := info.ModTime() // Ambil waktu terakhir file diubah

// 	// Format tanggal untuk tampilan lebih rapi
// 	formattedTime := modifiedTime.Format("2006-01-02 15:04:05")

// 	fmt.Println("📂 Memproses file:", filename, "dengan waktu terakhir:", formattedTime)

// 	// Simpan nama file ke database setelah berhasil diproses
// 	db.Create(&models.FileLog{Filename: filepath.Base(filename), DateModified: modifiedTime})
// 	fmt.Println("✅ File berhasil diproses & disimpan:", filename)

// 	// Identifikasi jenis file berdasarkan pola nama
// 	switch {
// 	case strings.HasPrefix(fileNameOnly, "RCV_"):
// 		fmt.Println("📥 Processing Receiving File:", fileNameOnly)
// 		processReceivingCSV(db, filename)

// 	case strings.HasPrefix(fileNameOnly, "SHIPMENT_"):
// 		fmt.Println("🚚 Processing Shipment File:", fileNameOnly)
// 		// processShipmentCSV(filename)

// 	case strings.HasPrefix(fileNameOnly, "STOCK_"):
// 		fmt.Println("📦 Processing Inventory File:", fileNameOnly)
// 		// processStockCSV(filename)

// 	default:
// 		fmt.Println("⚠️ Unrecognized File:", fileNameOnly)
// 	}
// }

// func processReceivingCSV(file string) {
// 	fmt.Println("✅ Received:", file)
// 	// Tambahkan logika parsing dan insert ke database di sini
// }

// func processShipmentCSV(file string) {
// 	fmt.Println("✅ Shipped:", file)
// 	// Tambahkan logika parsing dan insert ke database di sini
// }

// func processStockCSV(file string) {
// 	fmt.Println("✅ Inventory Updated:", file)
// 	// Tambahkan logika parsing dan insert ke database di sini
// }
