package configurations

import (
	"database/sql"
	"fiber-app/config"
	"fiber-app/database"
	"fiber-app/models"
	"fmt"
	"log"
	"regexp"
	"strings"
	"sync"

	"github.com/gofiber/fiber/v2"
	"gorm.io/driver/mysql"
	"gorm.io/driver/postgres"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

type HanlderConfigurations struct {
	DB *gorm.DB
}

func NewHanlderConfigurations(db *gorm.DB) *HanlderConfigurations {
	return &HanlderConfigurations{DB: db}
}

type DBRequest struct {
	Name string `json:"dbName"`
}

func CreateDatabase(c *fiber.Ctx) error {
	var req DBRequest

	userIDVal := c.Locals("userID")
	if userIDVal == nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized: userID not found in context",
		})
	}

	fmt.Println("User ID:", userIDVal)

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	dbName := strings.TrimSpace(req.Name)
	if dbName == "" || !isValidDBName(dbName) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid database name"})
	}

	// Buat koneksi ke DB utama
	db, err := OpenMasterConnection()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to master DB"})
	}

	// Cek apakah DB sudah ada
	exists, err := checkDatabaseExists(db, dbName)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Error checking DB existence"})
	}
	if exists {
		return c.Status(400).JSON(fiber.Map{"error": "Database already exists", "success": false})
	}

	// Buat database
	if err := createDatabase(db, dbName); err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to create database"})
	}

	// Buka koneksi ke DB baru
	// newDB, err := openDatabaseConnection(dbName)
	// if err != nil {
	// 	return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to new DB"})
	// }

	// // Jalankan migrasi
	// RunMigration(newDB)

	// Simpan ke tabel BusinessUnit

	userIDFloat, ok := userIDVal.(float64)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "userID is not a valid number",
		})
	}

	bu := models.BusinessUnit{
		DbName:    dbName,
		CreatedBy: int(userIDFloat),
	}

	if err := db.Create(&bu).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to save BusinessUnit"})
	}

	return c.JSON(fiber.Map{"message": "Database " + dbName + " created successfully", "success": true, "data": dbName})
}

func MigrateDB(c *fiber.Ctx) error {

	var req DBRequest

	if err := c.BodyParser(&req); err != nil {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
	}

	dbName := strings.TrimSpace(req.Name)
	if dbName == "" || !isValidDBName(dbName) {
		return c.Status(400).JSON(fiber.Map{"error": "Invalid database name"})
	}

	db, err := OpenMasterConnection()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to master DB"})
	}

	exists, err := checkDatabaseExists(db, dbName)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Error checking DB existence"})
	}
	if !exists {
		return c.Status(400).JSON(fiber.Map{"error": "Database does not exist", "success": false})
	}

	newDB, err := openDatabaseConnection(dbName)
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to new DB"})
	}

	// RunMigration(newDB)
	database.MigrateBusinessUnit(newDB)
	database.RunSeeders(newDB)
	return c.JSON(fiber.Map{"message": "Database migrated", "success": true, "data": dbName})
}

func OpenMasterConnection() (*gorm.DB, error) {
	_, dialector := getDSNAndDialector(config.DBName)
	return gorm.Open(dialector, &gorm.Config{})
}

func OpenMasterDB() (*gorm.DB, error) {
	_, dialector := getDSNAndDialector(config.DBName)
	return gorm.Open(dialector, &gorm.Config{})
}

func openDatabaseConnection(dbName string) (*gorm.DB, error) {
	_, dialector := getDSNAndDialector(dbName)
	return gorm.Open(dialector, &gorm.Config{})
}

var (
	dbPool  = make(map[string]*gorm.DB)
	dbMutex sync.Mutex
)

// GetDBConnection mengelola pool koneksi database per nama database
func GetDBConnection(dbName string) (*gorm.DB, error) {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	// Jika koneksi sudah ada di pool, gunakan yang itu
	if db, exists := dbPool[dbName]; exists {
		return db, nil
	}

	// Kalau belum, buat koneksi baru
	_, dialector := getDSNAndDialector(dbName)
	db, err := gorm.Open(dialector, &gorm.Config{})
	if err != nil {
		return nil, err
	}

	// Simpan ke pool
	dbPool[dbName] = db
	return db, nil
}

func PrintActiveDBConnections() {
	dbMutex.Lock()
	defer dbMutex.Unlock()

	i := 0
	for dbName := range dbPool {
		fmt.Println("Active DB connection index : ", i+1, " : "+dbName)
		i++
	}
}

func getDSNAndDialector(dbName string) (string, gorm.Dialector) {
	switch config.DBDriver {
	case "postgres":
		dsn := fmt.Sprintf("host=%s user=%s password=%s dbname=%s port=%s sslmode=disable",
			config.DBHost, config.DBUser, config.DBPassword, dbName, config.DBPort)
		return dsn, postgres.Open(dsn)
	case "mysql":
		dsn := fmt.Sprintf("%s:%s@tcp(%s:%s)/%s?charset=utf8mb4&parseTime=True&loc=Local",
			config.DBUser, config.DBPassword, config.DBHost, config.DBPort, dbName)
		return dsn, mysql.Open(dsn)
	case "mssql":
		dsn := fmt.Sprintf("sqlserver://%s:%s@%s:%s?database=%s",
			config.DBUser, config.DBPassword, config.DBHost, config.DBPort, dbName)
		return dsn, sqlserver.Open(dsn)
	default:
		log.Fatalf("Unsupported DB_DRIVER: %s", config.DBDriver)
		return "", nil
	}
}

func checkDatabaseExists(db *gorm.DB, dbName string) (bool, error) {
	var exists bool
	switch config.DBDriver {
	case "postgres":
		err := db.Raw("SELECT EXISTS(SELECT 1 FROM pg_database WHERE datname = ?)", dbName).Scan(&exists).Error
		return exists, err
	case "mysql":
		err := db.Raw("SELECT SCHEMA_NAME FROM INFORMATION_SCHEMA.SCHEMATA WHERE SCHEMA_NAME = ?", dbName).Scan(&exists).Error
		return exists, err
	case "mssql":
		err := db.Raw("SELECT name FROM master.sys.databases WHERE name = ?", dbName).Scan(&exists).Error
		return exists, err
	default:
		return false, fmt.Errorf("unsupported DB driver")
	}
}

func createDatabase(db *gorm.DB, dbName string) error {
	return db.Exec(fmt.Sprintf("CREATE DATABASE %s", dbName)).Error
}

func GetAllTables() fiber.Handler {
	return func(c *fiber.Ctx) error {
		var req DBRequest
		if err := c.BodyParser(&req); err != nil {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
		}

		dbName := strings.TrimSpace(req.Name)
		if dbName == "" || !isValidDBName(dbName) {
			return c.Status(400).JSON(fiber.Map{"error": "Invalid database name"})
		}

		db, err := GetDBConnection(dbName)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to DB"})
		}

		exists, err := checkDatabaseExists(db, dbName)
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		if !exists {
			return c.Status(404).JSON(fiber.Map{"error": fmt.Sprintf("Database '%s' does not exist", dbName)})
		}

		var tables []string
		var query string

		switch config.DBDriver {
		case "mysql":
			query = `SELECT table_name FROM information_schema.tables WHERE table_schema = ?`
		case "postgres":
			query = `SELECT table_name FROM information_schema.tables WHERE table_schema = 'public' AND table_type = 'BASE TABLE'`
		case "mssql":
			query = `SELECT name FROM sys.tables WHERE schema_name(schema_id) = ?`
		default:
			return c.Status(500).JSON(fiber.Map{"error": "Unsupported DB driver"})
		}

		var rows *sql.Rows
		if config.DBDriver == "postgres" {
			rows, err = db.Debug().Raw(query).Rows()
		} else {
			rows, err = db.Debug().Raw(query, dbName).Rows()
		}
		if err != nil {
			return c.Status(500).JSON(fiber.Map{"error": err.Error()})
		}
		defer rows.Close()

		tables = make([]string, 0)
		for rows.Next() {
			var table string
			if err := rows.Scan(&table); err != nil {
				return c.Status(500).JSON(fiber.Map{"error": err.Error()})
			}
			tables = append(tables, table)
		}

		return c.JSON(fiber.Map{
			"success": true,
			"data": fiber.Map{
				"db":     dbName,
				"tables": tables,
			},
		})
	}
}

// func GetAllTables() fiber.Handler {
// 	return func(c *fiber.Ctx) error {
// 		// dbName := c.Query("db")

// 		var req DBRequest

// 		if err := c.BodyParser(&req); err != nil {
// 			return c.Status(400).JSON(fiber.Map{"error": "Invalid request body"})
// 		}

// 		dbName := strings.TrimSpace(req.Name)
// 		if dbName == "" || !isValidDBName(dbName) {
// 			return c.Status(400).JSON(fiber.Map{"error": "Invalid database name"})
// 		}

// 		db, err := OpenMasterConnection()
// 		if err != nil {
// 			return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to master DB"})
// 		}

// 		if dbName == "" {
// 			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{
// 				"error": "Query parameter 'db' is required",
// 			})
// 		}

// 		var tables []string
// 		query := `
// 			SELECT table_name
// 			FROM information_schema.tables
// 			WHERE table_schema = ?
// 		`

// 		rows, err := db.Raw(query, dbName).Rows()
// 		if err != nil {
// 			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 				"error": err.Error(),
// 			})
// 		}
// 		defer rows.Close()

// 		for rows.Next() {
// 			var table string
// 			if err := rows.Scan(&table); err != nil {
// 				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
// 					"error": err.Error(),
// 				})
// 			}
// 			tables = append(tables, table)
// 		}

// 		return c.JSON(fiber.Map{
// 			"success": true,
// 			"data": fiber.Map{
// 				"db":     dbName,
// 				"tables": tables,
// 			},
// 		})
// 	}
// }

func isValidDBName(name string) bool {
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_]+$`, name)
	return matched
}

func GetAllBusinessUnit(c *fiber.Ctx) error {
	db, err := OpenMasterConnection()
	if err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to connect to master DB"})
	}

	var businessUnits []models.BusinessUnit
	if err := db.Find(&businessUnits).Error; err != nil {
		return c.Status(500).JSON(fiber.Map{"error": "Failed to retrieve BusinessUnits"})
	}
	return c.JSON(fiber.Map{"success": true, "data": businessUnits})
}
