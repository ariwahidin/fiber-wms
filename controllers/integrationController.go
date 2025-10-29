package controllers

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fiber-app/utils"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type IntegrationController struct {
	DB *gorm.DB
}

func NewIntegrationController(db *gorm.DB) *IntegrationController {
	return &IntegrationController{DB: db}
}

// type SOItem struct {
// 	SO_NO     string `json:"so_no"`
// 	ITEM_CODE string `json:"item_code"`
// 	ITEM_NAME string `json:"item_name"`
// 	QTY       string `json:"qty"`
// 	EXP_DATE  string `json:"exp_date"`
// 	BATCH_NO  string `json:"batch_no"`
// 	UNIT      string `json:"unit"`
// }

type SOItem struct {
	SO_NO     string `json:"so_no"`
	ITEM_CODE string `json:"item_code"`
	ITEM_NAME string `json:"item_name"`
	QTY       int    `json:"qty"` // ubah ke int
	EXP_DATE  string `json:"exp_date"`
	BATCH_NO  string `json:"batch_no"`
	UNIT      string `json:"unit"`
}

func (c *IntegrationController) CreateInboundFromCsv(ctx *fiber.Ctx) error {
	pendingDir := `D:\Source\INTEGRATION\SAP_TO_WMS\PENDING`
	processedDir := `D:\Source\INTEGRATION\SAP_TO_WMS\PROCESSED`
	errorDir := `D:\Source\INTEGRATION\SAP_TO_WMS\ERROR`
	logDir := `D:\Source\INTEGRATION\SAP_TO_WMS\LOGS`

	files, err := os.ReadDir(pendingDir)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Gagal membaca folder pending",
			"error":   err.Error(),
		})
	}

	if len(files) == 0 {
		return ctx.JSON(fiber.Map{"message": "Tidak ada file di folder pending"})
	}

	hasError := false // flag

	for _, file := range files {
		if filepath.Ext(file.Name()) != ".csv" {
			continue
		}

		filePath := filepath.Join(pendingDir, file.Name())
		logFile := filepath.Join(logDir, time.Now().Format("20060102")+".log")

		f, err := os.Open(filePath)
		if err != nil {
			writeLog(logFile, fmt.Sprintf("Gagal membuka file %s: %v", file.Name(), err))
			moveFile(filePath, filepath.Join(errorDir, file.Name()))
			hasError = true
			continue
		}
		defer f.Close()

		reader := csv.NewReader(f)
		_, _ = reader.Read() // skip header

		var items []SOItem
		for {
			record, err := reader.Read()
			if err == io.EOF {
				break
			}
			if err != nil {
				writeLog(logFile, fmt.Sprintf("Error membaca CSV %s: %v", file.Name(), err))
				moveFile(filePath, filepath.Join(errorDir, file.Name()))
				hasError = true
				break
			}

			if len(record) < 7 {
				writeLog(logFile, fmt.Sprintf("Data tidak valid di file %s: %+v", file.Name(), record))
				continue
			}

			qty, err := strconv.Atoi(record[3])
			if err != nil {
				writeLog(logFile, fmt.Sprintf("Qty tidak valid di file %s: %+v", file.Name(), record))
				hasError = true
				continue
			}

			items = append(items, SOItem{
				SO_NO:     record[0],
				ITEM_CODE: record[1],
				ITEM_NAME: record[2],
				QTY:       qty,
				EXP_DATE:  record[4],
				BATCH_NO:  record[5],
				UNIT:      record[6],
			})

		}

		// Contoh simulasi insert ke DB (atau kirim ke API lain)
		for _, item := range items {
			// kamu bisa ubah ke model DB sesuai kebutuhan
			fmt.Printf("Inbound SO %s - %s (%s) Qty: %s\n", item.SO_NO, item.ITEM_CODE, item.ITEM_NAME, item.QTY)
		}

		// Setelah selesai baca CSV
		f.Close() // pastikan ditutup sebelum dipindahkan

		// di sini kamu bisa panggil fungsi insert ke DB
		err = insertInboundToDB(c.DB, items)
		if err != nil {
			writeLog(logFile, fmt.Sprintf("Gagal insert DB untuk %s: %v", file.Name(), err))
			moveFile(filePath, filepath.Join(errorDir, file.Name()))
			utils.InsertLog(c.DB, models.IntegrationLog{
				ProcessName:  "INBOUND",
				FileName:     file.Name(),
				LogLevel:     "ERROR",
				Message:      fmt.Sprintf("Gagal insert DB untuk %s: %v", file.Name(), err),
				RecordKey:    logFile,
				SourceSystem: "SAP",
			})
			hasError = true
			continue
		}

		utils.InsertLog(c.DB, models.IntegrationLog{
			ProcessName:  "INBOUND",
			FileName:     file.Name(),
			LogLevel:     "INFO",
			Message:      fmt.Sprintf("Sukses proses file %s", file.Name()),
			RecordKey:    logFile,
			SourceSystem: "SAP",
		})

		dataJson, _ := json.MarshalIndent(items, "", "  ")
		writeLog(logFile, fmt.Sprintf("Sukses proses file %s: %s", file.Name(), string(dataJson)))
		moveFile(filePath, filepath.Join(processedDir, file.Name()))
	}

	if hasError {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Beberapa file gagal diproses, periksa log untuk detail.",
			"success": false,
		})
	}

	return ctx.JSON(fiber.Map{"message": "Create inbound from CSV successfully processed", "success": true})
}

// Struct agar mudah passing dari CSV
type InboundCsvData struct {
	SONo      string
	ItemCode  string
	ItemName  string
	Qty       float64
	ExpDate   string
	BatchNo   string
	UOM       string
	OwnerCode string
	WhsCode   string
}

func insertInboundToDB(db *gorm.DB, data []SOItem) error {
	if len(data) == 0 {
		return errors.New("data kosong")
	}

	// === Start Transaction ===
	tx := db.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Hardcode sementara (karena CSV belum punya field ini)
	userID := 1
	supplierCode := "FF_SUPPLIER"
	ownerCode := "FFID"
	whsCode := "CKY"
	inboundType := "SAP_INTEGRATION"

	var supplier models.Supplier
	if err := tx.First(&supplier, "supplier_code = ?", supplierCode).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("gagal ambil data supplier: %v", err)
	}

	var headerExists models.InboundHeader
	if err := tx.First(&headerExists, "receipt_id = ?", data[0].SO_NO).Error; err == nil {
		tx.Rollback()
		return fmt.Errorf("inbound header dengan receipt id %s sudah ada", data[0].SO_NO)
	}

	// === Generate inbound no ===
	repo := repositories.NewInboundRepository(tx)
	inboundNo, err := repo.GenerateInboundNo()
	if err != nil {
		tx.Rollback()
		return fmt.Errorf("gagal generate inbound no: %v", err)
	}

	// === Insert Header ===
	header := models.InboundHeader{
		InboundNo:   inboundNo,
		InboundDate: "2025-10-01",
		ReceiptID:   data[0].SO_NO,
		SupplierId:  int(supplier.ID),
		Supplier:    supplierCode,
		Status:      "open",
		CreatedBy:   userID,
		UpdatedBy:   userID,
		Type:        inboundType,
		WhsCode:     whsCode,
		OwnerCode:   ownerCode,
		Origin:      "SAP1",
		Remarks:     "Generated automatically from SAP CSV",
		Integration: true,
	}
	if err := tx.Create(&header).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("gagal insert inbound header: %v", err)
	}

	// === Insert Reference (gunakan SO_NO dari CSV) ===
	ref := models.InboundReference{
		InboundId: header.ID,
		RefNo:     data[0].SO_NO,
	}
	if err := tx.Create(&ref).Error; err != nil {
		tx.Rollback()
		return fmt.Errorf("gagal insert inbound reference: %v", err)
	}

	// === Insert Detail per baris CSV ===
	for _, item := range data {
		var product models.Product
		if err := tx.First(&product, "item_code = ?", item.ITEM_CODE).Error; err != nil {
			if errors.Is(err, gorm.ErrRecordNotFound) {
				tx.Rollback()
				return fmt.Errorf("product tidak ditemukan: %s", item.ITEM_CODE)
			}
			tx.Rollback()
			return fmt.Errorf("gagal ambil data produk: %v", err)
		}

		expDateParsed, _ := time.Parse("2006-01-02", item.EXP_DATE)

		detail := models.InboundDetail{
			InboundNo:     inboundNo,
			InboundId:     int(header.ID),
			ItemCode:      item.ITEM_CODE,
			ItemId:        product.ID,
			ProductNumber: product.ProductNumber,
			Barcode:       product.Barcode,
			Uom:           item.UNIT,
			Quantity:      int(item.QTY),
			QaStatus:      "A",
			WhsCode:       whsCode,
			RecDate:       time.Now().Local().Format("2006-01-02"),
			ExpDate:       expDateParsed.Local().Format("2006-01-02"),
			LotNumber:     item.BATCH_NO,
			IsSerial:      product.HasSerial,
			RefId:         int(ref.ID),
			RefNo:         data[0].SO_NO,
			OwnerCode:     ownerCode,
			DivisionCode:  "FF_DIV",
			CreatedBy:     userID,
			UpdatedBy:     userID,
		}

		if err := tx.Create(&detail).Error; err != nil {
			tx.Rollback()
			return fmt.Errorf("gagal insert inbound detail untuk %s: %v", item.ITEM_CODE, err)
		}
	}

	// === Commit ===
	if err := tx.Commit().Error; err != nil {
		return fmt.Errorf("gagal commit transaksi: %v", err)
	}

	return nil
}

// --- Helper ---

// func moveFile(src, dst string) {
// 	os.MkdirAll(filepath.Dir(dst), os.ModePerm)
// 	os.Rename(src, dst)
// }

// func moveFile(src, dst string) {
// 	os.MkdirAll(filepath.Dir(dst), os.ModePerm)

// 	// Tutup dulu file jika masih terbuka oleh proses lain
// 	// (tidak perlu kalau sudah di luar blok open)
// 	err := os.Rename(src, dst)
// 	if err == nil {
// 		return
// 	}

// 	// Jika gagal rename (misal beda drive), fallback ke copy & delete
// 	srcFile, err1 := os.Open(src)
// 	if err1 != nil {
// 		fmt.Printf("Gagal membuka sumber %s: %v\n", src, err1)
// 		return
// 	}
// 	defer srcFile.Close()

// 	dstFile, err2 := os.Create(dst)
// 	if err2 != nil {
// 		fmt.Printf("Gagal membuat file tujuan %s: %v\n", dst, err2)
// 		return
// 	}
// 	defer dstFile.Close()

// 	_, err3 := io.Copy(dstFile, srcFile)
// 	if err3 != nil {
// 		fmt.Printf("Gagal menyalin file: %v\n", err3)
// 		return
// 	}

// 	srcFile.Close()
// 	os.Remove(src)
// }

func moveFile(src, dst string) {
	os.MkdirAll(filepath.Dir(dst), os.ModePerm)

	// Coba rename langsung
	err := os.Rename(src, dst)
	if err == nil {
		return
	}

	// Jika gagal rename (misal beda drive), fallback copy + delete
	srcFile, err := os.Open(src)
	if err != nil {
		fmt.Printf("Gagal membuka sumber %s: %v\n", src, err)
		return
	}
	defer srcFile.Close()

	dstFile, err := os.Create(dst)
	if err != nil {
		fmt.Printf("Gagal membuat file tujuan %s: %v\n", dst, err)
		return
	}
	defer dstFile.Close()

	_, err = io.Copy(dstFile, srcFile)
	if err != nil {
		fmt.Printf("Gagal menyalin file %s ke %s: %v\n", src, dst, err)
		return
	}

	// Pastikan file tertutup sebelum hapus
	srcFile.Close()
	time.Sleep(100 * time.Millisecond)

	err = os.Remove(src)
	if err != nil {
		fmt.Printf("Gagal menghapus file sumber %s: %v\n", src, err)
	} else {
		fmt.Printf("File %s berhasil dipindahkan ke %s\n", src, dst)
	}
}

func writeLog(path, message string) {
	os.MkdirAll(filepath.Dir(path), os.ModePerm)
	f, _ := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	defer f.Close()
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	f.WriteString(fmt.Sprintf("[%s] %s\n", timestamp, message))
}
