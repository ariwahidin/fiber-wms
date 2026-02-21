package database

import (
	"fmt"
	"os"

	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

// OpenReadOnlyDB membuka koneksi ke SQL Server menggunakan
// user read-only khusus untuk Report Mailer.
// Credential diambil dari environment variable.
func OpenReadOnlyDB() (*gorm.DB, error) {
	host := os.Getenv("DB_HOST")
	port := os.Getenv("DB_PORT")
	dbName := os.Getenv("DB_UNIT") // pakai nama database unit yang sama

	// User read-only khusus report mailer
	// Berbeda dari user utama WMS
	roUser := os.Getenv("REPORT_DB_USER")
	roPass := os.Getenv("REPORT_DB_PASSWORD")

	if roUser == "" || roPass == "" {
		return nil, fmt.Errorf("REPORT_DB_USER atau REPORT_DB_PASSWORD belum diset di .env")
	}

	dsn := fmt.Sprintf(
		"sqlserver://%s:%s@%s:%s?database=%s",
		roUser, roPass, host, port, dbName,
	)

	db, err := gorm.Open(sqlserver.Open(dsn), &gorm.Config{
		Logger: logger.Default.LogMode(logger.Silent),
	})
	if err != nil {
		return nil, fmt.Errorf("gagal koneksi read-only DB: %w", err)
	}

	// Verifikasi koneksi
	sqlDB, err := db.DB()
	if err != nil {
		return nil, err
	}

	if err := sqlDB.Ping(); err != nil {
		return nil, fmt.Errorf("read-only DB tidak dapat dijangkau: %w", err)
	}

	// Pool connection — lebih kecil karena hanya untuk report
	sqlDB.SetMaxOpenConns(5)
	sqlDB.SetMaxIdleConns(2)

	return db, nil
}
