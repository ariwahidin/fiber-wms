package main

import (
	"encoding/csv"
	"fiber-app/config"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"gopkg.in/gomail.v2"
	"gorm.io/driver/sqlserver"
	"gorm.io/gorm"
)

// Konfigurasi database
func connectDB() (*gorm.DB, error) {

	dsn := "sqlserver://" + config.DBUser + ":" + config.DBPassword + "@" + config.DBHost + ":" + config.DBPort + "?database=" + config.DBName
	db, err := gorm.Open(sqlserver.Open(dsn), &gorm.Config{})

	if err != nil {
		fmt.Println("Error connecting to database:", err)
		return nil, err
	}

	return db, nil
}

// Proses semua file CSV di folder `unprocessed`
func processAllCSV(db *gorm.DB) {
	unprocessedFolder := "D:\\Golang Project\\backend-wms\\sap-data\\unprocessed"

	files, err := os.ReadDir(unprocessedFolder)
	if err != nil {
		log.Println("❌ Gagal membaca folder:", err)
		return
	}

	for _, file := range files {
		if filepath.Ext(file.Name()) == ".csv" {
			filePath := filepath.Join(unprocessedFolder, file.Name())
			fmt.Println("📂 Memproses:", filePath)

			processReceivingCSV(db, filePath)
		}
	}
}

func processFile(db *gorm.DB, filename string) {
	// Ambil hanya nama file tanpa path
	fileNameOnly := filepath.Base(filename)

	// Cek apakah file sudah pernah diproses
	var existingFile models.FileLog
	if err := db.Where("filename = ?", filepath.Base(filename)).First(&existingFile).Error; err == nil {
		log.Println("⚠️ File sudah pernah diproses, skip:", filename)
		return
	}

	info, err := os.Stat(filename)
	if err != nil {
		fmt.Println("❌ Gagal membaca file:", err)
		return
	}

	modifiedTime := info.ModTime() // Ambil waktu terakhir file diubah

	// Format tanggal untuk tampilan lebih rapi
	formattedTime := modifiedTime.Format("2006-01-02 15:04:05")

	fmt.Println("📂 Memproses file:", filename, "dengan waktu terakhir:", formattedTime)

	// Simpan nama file ke database setelah berhasil diproses
	db.Create(&models.FileLog{Filename: filepath.Base(filename), DateModified: modifiedTime})
	fmt.Println("✅ File berhasil diproses & disimpan:", filename)

	// Identifikasi jenis file berdasarkan pola nama
	switch {
	case strings.HasPrefix(fileNameOnly, "RCV_"):
		fmt.Println("📥 Processing Receiving File:", fileNameOnly)
		processReceivingCSV(db, filename)

	case strings.HasPrefix(fileNameOnly, "SHIPMENT_"):
		fmt.Println("🚚 Processing Shipment File:", fileNameOnly)
		// processShipmentCSV(filename)

	case strings.HasPrefix(fileNameOnly, "STOCK_"):
		fmt.Println("📦 Processing Inventory File:", fileNameOnly)
		// processStockCSV(filename)

	default:
		fmt.Println("⚠️ Unrecognized File:", fileNameOnly)
	}
}

func checkUnprocessedFiles(db *gorm.DB) {

	fmt.Println("📂 Memproses file unprocessed")

	unproccessed_folder := "D:\\Golang Project\\backend-wms\\sap-data\\unprocessed\\"
	files, err := filepath.Glob(unproccessed_folder + "*.csv")
	if err != nil {
		log.Fatal("❌ Gagal membaca folder:", err)
	}

	for _, file := range files {
		fmt.Println("📂 Memproses file:", file)
		processFile(db, file)
	}

}

func processReceivingCSV(db *gorm.DB, filename string) {
	file, err := os.Open(filename)
	if err != nil {
		fmt.Println("❌ Gagal membuka file:", err)
		return
	}

	reader := csv.NewReader(file)
	records, err := reader.ReadAll()
	if err != nil {
		fmt.Println("❌ Gagal membaca file CSV:", err)
		file.Close() // Pastikan file ditutup jika gagal membaca
		return
	}

	file.Close() // Tutup file sebelum dipindahkan!

	var inboundDetails []models.InboundDetail

	for i, record := range records {
		if i == 0 {
			continue // Skip header
		}

		var supplier models.Supplier
		var product models.Product
		// var wh_code models.WarehouseCode

		quantity, _ := strconv.Atoi(record[4])

		data := models.Receiving{
			InboundID:     record[0],
			PO_Number:     record[1],
			Material:      record[2],
			Description:   record[3],
			Quantity:      quantity,
			UOM:           record[5],
			Warehouse:     record[6],
			ReceivingDate: record[7],
			Supplier:      record[8],
			Filename:      filepath.Base(filename),
		}

		// Supplier
		db.Where("supplier_code = ?", data.Supplier).First(&supplier)
		if supplier.ID == 0 {
			supplier = models.Supplier{
				SupplierCode: data.Supplier,
				SupplierName: data.Supplier,
			}
			db.Create(&supplier)
		}

		// Product
		db.Where("item_code = ?", data.Material).First(&product)
		if product.ID == 0 {
			product = models.Product{
				ItemCode: data.Material,
				GMC:      data.Material,
				Barcode:  data.Material,
				ItemName: data.Description,
			}
			db.Create(&product)
		} else {
			fmt.Println("📌 Produk sudah ada:", product.ItemCode)
		}

		// Warehouse Code
		db.Where("warehouse_code = ?", data.Warehouse).First(&wh_code)
		if wh_code.ID == 0 {
			wh_code = models.WarehouseCode{
				WarehouseCode: data.Warehouse,
			}
			db.Create(&wh_code)
		}

		db.Create(&data)

		inboundDetails = append(inboundDetails, models.InboundDetail{
			InboundId: 0,
			ItemID:    int(product.ID),
			ItemCode:  product.ItemCode,
			Quantity:  data.Quantity,
			Uom:       data.UOM,
			WhsCode:   data.Warehouse,
			RecDate:   data.ReceivingDate,
			Status:    "open",
		})

		fmt.Println("✅ Data Inserted:", data)
	}

	fmt.Println("✅ Data Detail:", inboundDetails)

	// Inisialisasi repository
	repo := repositories.NewInboundRepository(db)

	// Membuat inbound baru
	inboundHeader := models.InboundHeader{}
	inboundHeader.InboundDate = time.Now().Format("2006-01-02 15:04:05")
	inboundHeader.SupplierCode = records[1][8]
	inboundHeader.PoNo = records[1][1]
	newInbound, err := repo.CreateInboundOpen(inboundHeader, inboundDetails)
	if err != nil {
		fmt.Println("❌ Gagal membuat inbound:", err)
	}

	sendEmailNotification([]string{"ari.wahidin@id.yusen-logistics.com"}, newInbound.Code)

	// **Tutup file sebelum pindah**
	time.Sleep(1 * time.Second) // Tunggu sebentar untuk memastikan file tidak terkunci

	const processedFolder = "D:\\Golang Project\\backend-wms\\sap-data\\processed"

	// Pastikan folder `processed` ada
	if _, err := os.Stat(processedFolder); os.IsNotExist(err) {
		err := os.MkdirAll(processedFolder, os.ModePerm)
		if err != nil {
			log.Fatalf("❌ Gagal membuat folder processed: %v", err)
		}
	}

	// Pindahkan file ke folder processed
	processedFilePath := filepath.Join(processedFolder, filepath.Base(filename))

	err = os.Rename(filename, processedFilePath)
	if err != nil {
		fmt.Println("⚠️  Rename gagal, coba metode copy & delete...")
		err = copyAndDeleteFile(filename, processedFilePath)
		if err != nil {
			log.Fatalf("❌ Gagal memindahkan file ke folder processed: %v", err)
		}
	}

	fmt.Println("✅ Inbound Created:", newInbound)
}

func copyAndDeleteFile(src, dst string) error {
	sourceFile, err := os.Open(src)
	if err != nil {
		return err
	}
	defer sourceFile.Close()

	destinationFile, err := os.Create(dst)
	if err != nil {
		return err
	}
	defer destinationFile.Close()

	_, err = io.Copy(destinationFile, sourceFile)
	if err != nil {
		return err
	}

	return os.Remove(src) // Hapus file lama setelah berhasil disalin
}

func sendEmailNotification(toEmails []string, inboundID string) error {
	// Konfigurasi SMTP
	smtpHost := "c6.icoremail.net"
	smtpPort := 465
	senderEmail := "wmsadm@puninar.co.id"
	senderPassword := "wms2k2!PY"

	// Format isi email
	subject := "📦 New Inbound " + inboundID
	body := fmt.Sprintf(`
		<html>
			<body>
				<h3>New Inbound is created</h3>
				<p>ID Inbound: <strong>%s</strong></p>
				<p>This is an auto-generated email. Please do not reply to this email or its recipients.</p>
			</body>
		</html>
	`, inboundID)

	// Setup email
	msg := gomail.NewMessage()
	msg.SetHeader("From", senderEmail)
	msg.SetHeader("To", toEmails...) // Mengirim ke banyak email
	msg.SetHeader("Subject", subject)
	msg.SetBody("text/html", body)

	// Kirim email
	dialer := gomail.NewDialer(smtpHost, smtpPort, senderEmail, senderPassword)
	if err := dialer.DialAndSend(msg); err != nil {
		fmt.Println("❌ Gagal mengirim email:", err)
		return err
	}

	fmt.Println("✅ Email notifikasi terkirim ke:", toEmails)
	return nil
}
func main() {
	db, err := connectDB()
	if err != nil {
		log.Fatalf("❌ Gagal konek ke database: %v", err)
	}

	fmt.Println("🚀 Processor CSV berjalan...")

	checkUnprocessedFiles(db)

	// sendEmailNotification([]string{"ari.wahidin@id.yusen-logistics.com"}, "TEST-IBD979886897")

	fmt.Println("✅ Semua file CSV diproses!")
}
