package inbound_controller

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"strconv"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ---------- 1. CHECK ALL (insert ke InboundBarcode) ----------

// CheckPutawayByInboundNo memvalidasi dan memproses proses putaway untuk sebuah inbound,
// lalu mengembalikan preview item yang berstatus "pending" (siap untuk di-confirm oleh FE).
//
// RIWAYAT OPTIMASI:
//
//  1. Query barcode & location yang tadinya dijalankan per-baris di dalam loop
//     (N query untuk N baris inboundDetail) diganti jadi preload sekali di awal,
//     lalu di-map di memory. Insert yang tadinya satu-per-satu diganti batch insert.
//
//  2. DITEMUKAN BUG DATA INTEGRITY: karena insert sebelumnya dijalankan satu-per-satu
//     TANPA transaction, kalau proses berhenti di tengah jalan (misal request kena
//     timeout dari Nginx/Cloudflare karena loop-nya lambat), row yang sudah sempat
//     ke-insert TETAP TERSIMPAN secara permanen, sedangkan sisanya hilang tanpa error
//     yang jelas ke user. Contoh nyata: dari 186 baris inboundDetail, cuma 76 barcode
//     yang berhasil kebentuk, 110 sisanya tidak diproses sama sekali.
//
//     FIX: seluruh proses (baca + tulis) sekarang dibungkus dalam SATU database
//     transaction (`c.DB.Transaction(...)`). Kalau ada kegagalan di titik manapun
//     (termasuk timeout / context canceled), SEMUA perubahan di-rollback — tidak ada
//     lagi kondisi "sebagian masuk, sebagian hilang". Baik semua berhasil, baik semua
//     batal, tidak ada hasil setengah-setengah.
//
//  3. Batch size untuk CreateInBatches dihitung berdasarkan limit SQL Server
//     (maksimal 2100 parameter per query) dibagi jumlah kolom InboundBarcode (35
//     kolom fisik, tidak termasuk relasi Product). 2100 / 35 = 60, dikurangi margin
//     aman 5 → batch size 55.
//
// PENTING: Logic bisnis dan urutan validasi tidak diubah. Ini murni perbaikan
// reliability (transaction) + optimasi performa (preload + batch insert).
// func (c *InboundController) CheckPutawayByInboundNo(ctx *fiber.Ctx) error {
// 	var payload struct {
// 		InboundNo string `json:"inbound_no"`
// 	}
// 	if err := ctx.BodyParser(&payload); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
// 	}

// 	// --- Ambil header inbound ---
// 	inboundHeader := models.InboundHeader{}
// 	if err := c.DB.Debug().First(&inboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// --- Ambil kebijakan inventory pemilik barang ---
// 	var invPolicy models.InventoryPolicy
// 	if err := c.DB.Debug().First(&invPolicy, "owner_code = ?", inboundHeader.OwnerCode).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// --- Ambil data warehouse (dipertahankan meski tidak dipakai langsung,
// 	//     sesuai perilaku kode asli) ---
// 	var warehouse models.Warehouse
// 	if err := c.DB.Debug().First(&warehouse, "code = ?", inboundHeader.WhsCode).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if invPolicy.RequirePutawayScan {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot putaway from your side, please putaway from scanner"})
// 	}

// 	// --- Ambil semua detail inbound sekaligus ---
// 	var inboundDetail []models.InboundDetail
// 	if err := c.DB.Debug().Where("inbound_id = ?", inboundHeader.ID).Find(&inboundDetail).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	inboundRepo := repositories.NewInboundRepository(c.DB)

// 	palletID, err := inboundRepo.GeneratePalletID(inboundHeader.InboundNo)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if !invPolicy.RequireReceiveScan {

// 		// Validasi: semua baris harus sudah punya location sebelum putaway.
// 		var allRcvLocationIsFilled bool = true
// 		for _, detail := range inboundDetail {
// 			if detail.Location == "" {
// 				allRcvLocationIsFilled = false
// 				break
// 			}
// 		}

// 		if !allRcvLocationIsFilled {
// 			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Please fill all receiving location before putaway"})
// 		}

// 		// =========================================================
// 		// FIX RELIABILITY: bungkus preload + insert dalam 1 transaction.
// 		// Kalau ada kegagalan/timeout di tengah jalan, semua rollback —
// 		// tidak ada lagi kondisi data "kepotong separuh" seperti sebelumnya.
// 		// =========================================================
// 		var missingLocationCode string

// 		txErr := c.DB.Transaction(func(tx *gorm.DB) error {

// 			// Preload semua InboundBarcode untuk inbound ini dalam satu query,
// 			// lalu group per InboundDetailId di memory.
// 			var allBarcodesForInbound []models.InboundBarcode
// 			if err := tx.Where("inbound_id = ?", inboundHeader.ID).Find(&allBarcodesForInbound).Error; err != nil {
// 				return err
// 			}

// 			barcodesByDetailID := make(map[int][]models.InboundBarcode, len(inboundDetail))
// 			for _, bc := range allBarcodesForInbound {
// 				barcodesByDetailID[bc.InboundDetailId] = append(barcodesByDetailID[bc.InboundDetailId], bc)
// 			}

// 			// Preload semua Location yang dibutuhkan (unik) dalam satu query "IN (...)".
// 			locationCodeSet := make(map[string]struct{})
// 			for _, detail := range inboundDetail {
// 				locationCodeSet[detail.Location] = struct{}{}
// 			}
// 			locationCodes := make([]string, 0, len(locationCodeSet))
// 			for code := range locationCodeSet {
// 				locationCodes = append(locationCodes, code)
// 			}

// 			var locations []models.Location
// 			if err := tx.Where("location_code IN ?", locationCodes).Find(&locations).Error; err != nil {
// 				return err
// 			}

// 			locationByCode := make(map[string]models.Location, len(locations))
// 			for _, loc := range locations {
// 				locationByCode[loc.LocationCode] = loc
// 			}

// 			createdBy := int(ctx.Locals("userID").(float64))

// 			// Kumpulkan semua barcode baru ke slice dulu, insert sekali di akhir.
// 			newBarcodes := make([]models.InboundBarcode, 0, len(inboundDetail))

// 			for _, detail := range inboundDetail {

// 				inboundBarcodesCheck := barcodesByDetailID[int(detail.ID)]

// 				totalQtyReq := detail.Quantity
// 				totalQtyScanned := float64(0)
// 				for _, inboundBarcode := range inboundBarcodesCheck {
// 					totalQtyScanned += inboundBarcode.Quantity
// 				}
// 				newQtyScanned := totalQtyReq - totalQtyScanned

// 				// Catatan: logic ini identik dengan versi asli — inputSerialNumber
// 				// selalu bernilai detail.SerialNumber pada kedua cabang kondisi.
// 				inputSerialNumber := detail.SerialNumber
// 				if invPolicy.UseSerialNumber && detail.SerialNumber != "" {
// 					inputSerialNumber = detail.SerialNumber
// 				}

// 				location, ok := locationByCode[detail.Location]
// 				if !ok {
// 					// Simpan kode lokasi yang bermasalah, lalu return error khusus
// 					// (bukan langsung response HTTP) supaya GORM tahu harus
// 					// ROLLBACK transaction ini sepenuhnya.
// 					missingLocationCode = detail.Location
// 					return gorm.ErrRecordNotFound
// 				}

// 				if newQtyScanned > 0 {
// 					newBarcodes = append(newBarcodes, models.InboundBarcode{
// 						InboundId:       int(inboundHeader.ID),
// 						InboundDetailId: int(detail.ID),
// 						ItemCode:        detail.ItemCode,
// 						ItemID:          detail.ItemId,
// 						ScanData:        detail.Barcode,
// 						Barcode:         detail.Barcode,
// 						CartonNumber:    detail.CartonNumber,
// 						CaseNumber:      detail.CaseNumber,
// 						SerialNumber:    inputSerialNumber,
// 						Pallet:          palletID,
// 						Location:        location.LocationCode,
// 						Quantity:        newQtyScanned,
// 						WhsCode:         detail.WhsCode,
// 						OwnerCode:       detail.OwnerCode,
// 						DivisionCode:    detail.DivisionCode,
// 						QaStatus:        detail.QaStatus,
// 						Status:          "pending",
// 						Uom:             detail.Uom,
// 						RecDate:         detail.RecDate,
// 						ProdDate:        detail.ProdDate,
// 						ExpDate:         detail.ExpDate,
// 						LotNumber:       detail.LotNumber,
// 						CreatedBy:       createdBy,
// 					})
// 				}
// 			}

// 			// Batch insert. SQL Server membatasi maksimal 2100 parameter per query.
// 			// InboundBarcode punya 35 kolom fisik, jadi batch size dihitung dari situ,
// 			// bukan angka bebas.
// 			const inboundBarcodeColumnCount = 35
// 			const mssqlMaxParams = 2100
// 			safeBatchSize := (mssqlMaxParams / inboundBarcodeColumnCount) - 5 // margin aman, hasil: 55

// 			if len(newBarcodes) > 0 {
// 				if err := tx.Debug().CreateInBatches(newBarcodes, safeBatchSize).Error; err != nil {
// 					return err
// 				}
// 			}

// 			// return nil di sini artinya semua langkah di atas berhasil →
// 			// GORM otomatis COMMIT transaction ini.
// 			return nil
// 		})

// 		// Kalau transaction gagal di titik manapun (termasuk timeout/context
// 		// canceled), seluruh perubahan sudah otomatis di-rollback oleh GORM.
// 		if txErr != nil {
// 			if missingLocationCode != "" {
// 				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Location " + missingLocationCode + " not registered in system"})
// 			}
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": txErr.Error()})
// 		}
// 	}

// 	// --- Ambil semua barcode berstatus pending untuk preview di FE ---
// 	var inboundBarcodes []models.InboundBarcode
// 	if err := c.DB.Debug().Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").Find(&inboundBarcodes).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if len(inboundBarcodes) == 0 {
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scanned pending item not found"})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success":    true,
// 		"inbound_no": inboundHeader.InboundNo,
// 		"pallet":     palletID,
// 		"items":      inboundBarcodes,
// 	})
// }

// CheckPutawayByInboundNo memvalidasi dan memproses proses putaway untuk sebuah inbound,
// lalu mengembalikan preview item yang berstatus "pending" (siap untuk di-confirm oleh FE).
//
// CATATAN PERUBAHAN:
// Logic dan struktur kode ini SENGAJA dipertahankan PERSIS seperti versi original
// (N+1 query per baris inboundDetail tetap ada, tidak di-preload/di-batch).
// SATU-SATUNYA perubahan: seluruh proses baca + tulis di dalam loop dibungkus
// dalam SATU database transaction (`c.DB.Transaction(...)`).
//
// Tujuannya murni untuk keamanan data (data integrity), bukan performa:
//   - Sebelumnya, insert dijalankan satu-per-satu tanpa transaction. Kalau proses
//     berhenti di tengah jalan (misal request timeout dari Nginx/Cloudflare karena
//     loop-nya lambat saat data banyak), baris yang sudah sempat ke-insert TETAP
//     TERSIMPAN permanen, sedangkan sisanya hilang tanpa error yang jelas ke user.
//   - Dengan transaction, kalau ada kegagalan di titik manapun di dalam loop
//     (termasuk saat koneksi terputus / context timeout), SEMUA perubahan pada
//     eksekusi tersebut di-ROLLBACK oleh GORM. Hasilnya cuma dua kemungkinan:
//     semua baris berhasil diproses, atau tidak ada satupun yang tersimpan.
//     Tidak ada lagi kondisi data "kepotong separuh".
//
// Konsekuensi: karena masih N+1 query per baris, function ini tetap akan terasa
// lambat kalau inboundDetail sangat banyak (ratusan baris) — itu trade-off yang
// disengaja sesuai keputusan untuk mempertahankan struktur kode asli.
// func (c *InboundController) CheckPutawayByInboundNo(ctx *fiber.Ctx) error {
// 	var payload struct {
// 		InboundNo string `json:"inbound_no"`
// 	}
// 	if err := ctx.BodyParser(&payload); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
// 	}

// 	inboundHeader := models.InboundHeader{}
// 	if err := c.DB.Debug().First(&inboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var invPolicy models.InventoryPolicy
// 	if err := c.DB.Debug().First(&invPolicy, "owner_code = ?", inboundHeader.OwnerCode).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var warehouse models.Warehouse
// 	if err := c.DB.Debug().First(&warehouse, "code = ?", inboundHeader.WhsCode).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if invPolicy.RequirePutawayScan {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot putaway from your side, please putaway from scanner"})
// 	}

// 	var inboundDetail []models.InboundDetail
// 	if err := c.DB.Debug().Where("inbound_id = ?", inboundHeader.ID).Find(&inboundDetail).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	inboundRepo := repositories.NewInboundRepository(c.DB)

// 	palletID, err := inboundRepo.GeneratePalletID(inboundHeader.InboundNo)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if !invPolicy.RequireReceiveScan {

// 		var allRcvLocationIsFilled bool = true
// 		for _, detail := range inboundDetail {
// 			if detail.Location == "" {
// 				allRcvLocationIsFilled = false
// 				break
// 			}
// 		}

// 		if !allRcvLocationIsFilled {
// 			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Please fill all receiving location before putaway"})
// 		}

// 		// =========================================================
// 		// SATU-SATUNYA PERUBAHAN: bungkus loop asli (baca + tulis)
// 		// dalam satu transaction. Isi loop di dalamnya TIDAK diubah,
// 		// hanya c.DB diganti jadi tx supaya semua query ikut masuk
// 		// ke transaction yang sama.
// 		// =========================================================

// 		// notFoundLocationMsg dipakai untuk membawa pesan error location
// 		// yang spesifik keluar dari closure transaction, supaya response
// 		// ke user tetap sama persis seperti versi original.
// 		var notFoundLocationMsg string

// 		txErr := c.DB.Transaction(func(tx *gorm.DB) error {
// 			for _, detail := range inboundDetail {

// 				var inboundBarcodesCheck []models.InboundBarcode
// 				if err := tx.Debug().Where("inbound_detail_id = ?", detail.ID).Find(&inboundBarcodesCheck).Error; err != nil {
// 					return err
// 				}

// 				totalQtyReq := detail.Quantity
// 				totalQtyScanned := float64(0)
// 				newQtyScanned := float64(0)

// 				for _, inboundBarcode := range inboundBarcodesCheck {
// 					totalQtyScanned += inboundBarcode.Quantity
// 				}

// 				newQtyScanned = totalQtyReq - totalQtyScanned

// 				inputSerialNumber := detail.SerialNumber
// 				if invPolicy.UseSerialNumber && detail.SerialNumber != "" {
// 					inputSerialNumber = detail.SerialNumber
// 				}

// 				var location models.Location
// 				if err := tx.Debug().First(&location, "location_code = ?", detail.Location).Error; err != nil {
// 					if errors.Is(err, gorm.ErrRecordNotFound) {
// 						// Simpan pesan errornya, lalu tetap return error supaya
// 						// GORM tahu transaction ini harus di-ROLLBACK.
// 						notFoundLocationMsg = "Location " + detail.Location + " not registered in system"
// 						return err
// 					}
// 					return err
// 				}

// 				if newQtyScanned > 0 {
// 					newInboundBarcode := models.InboundBarcode{
// 						InboundId:       int(inboundHeader.ID),
// 						InboundDetailId: int(detail.ID),
// 						ItemCode:        detail.ItemCode,
// 						ItemID:          detail.ItemId,
// 						ScanData:        detail.Barcode,
// 						Barcode:         detail.Barcode,
// 						CartonNumber:    detail.CartonNumber,
// 						CaseNumber:      detail.CaseNumber,
// 						SerialNumber:    inputSerialNumber,
// 						Pallet:          palletID,
// 						Location:        location.LocationCode,
// 						Quantity:        newQtyScanned,
// 						WhsCode:         detail.WhsCode,
// 						OwnerCode:       detail.OwnerCode,
// 						DivisionCode:    detail.DivisionCode,
// 						QaStatus:        detail.QaStatus,
// 						Status:          "pending",
// 						Uom:             detail.Uom,
// 						RecDate:         detail.RecDate,
// 						ProdDate:        detail.ProdDate,
// 						ExpDate:         detail.ExpDate,
// 						LotNumber:       detail.LotNumber,
// 						CreatedBy:       int(ctx.Locals("userID").(float64)),
// 					}

// 					if err := tx.Debug().Create(&newInboundBarcode).Error; err != nil {
// 						return err
// 					}
// 				}
// 			}

// 			// return nil di sini artinya semua baris berhasil diproses →
// 			// GORM otomatis COMMIT seluruh perubahan dalam transaction ini.
// 			return nil
// 		})

// 		// Kalau txErr != nil, GORM sudah otomatis ROLLBACK semua perubahan
// 		// yang terjadi di dalam transaction ini — tidak ada data yang
// 		// "kepotong separuh" seperti pada versi tanpa transaction.
// 		if txErr != nil {
// 			if notFoundLocationMsg != "" {
// 				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": notFoundLocationMsg})
// 			}
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": txErr.Error()})
// 		}
// 	}

// 	// ambil semua yang pending, buat ditampilin ke FE sebagai preview sebelum confirm
// 	var inboundBarcodes []models.InboundBarcode
// 	if err := c.DB.Debug().Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").Find(&inboundBarcodes).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if len(inboundBarcodes) == 0 {
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scanned pending item not found"})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success":    true,
// 		"inbound_no": inboundHeader.InboundNo,
// 		"pallet":     palletID,
// 		"items":      inboundBarcodes,
// 	})
// }

// func (c *InboundController) CheckPutawayByInboundNo(ctx *fiber.Ctx) error {
// 	var payload struct {
// 		InboundNo string `json:"inbound_no"`
// 	}
// 	if err := ctx.BodyParser(&payload); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
// 	}

// 	inboundHeader := models.InboundHeader{}
// 	if err := c.DB.Debug().First(&inboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var invPolicy models.InventoryPolicy
// 	if err := c.DB.Debug().First(&invPolicy, "owner_code = ?", inboundHeader.OwnerCode).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var warehouse models.Warehouse
// 	if err := c.DB.Debug().First(&warehouse, "code = ?", inboundHeader.WhsCode).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if invPolicy.RequirePutawayScan {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot putaway from your side, please putaway from scanner"})
// 	}

// 	var inboundDetail []models.InboundDetail
// 	if err := c.DB.Debug().Where("inbound_id = ?", inboundHeader.ID).Find(&inboundDetail).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	inboundRepo := repositories.NewInboundRepository(c.DB)

// 	palletID, err := inboundRepo.GeneratePalletID(inboundHeader.InboundNo)
// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if !invPolicy.RequireReceiveScan {

// 		var allRcvLocationIsFilled bool = true
// 		for _, detail := range inboundDetail {
// 			if detail.Location == "" {
// 				allRcvLocationIsFilled = false
// 				break
// 			}
// 		}

// 		if !allRcvLocationIsFilled {
// 			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Please fill all receiving location before putaway"})
// 		}

// 		for _, detail := range inboundDetail {

// 			var inboundBarcodesCheck []models.InboundBarcode
// 			if err := c.DB.Debug().Where("inbound_detail_id = ?", detail.ID).Find(&inboundBarcodesCheck).Error; err != nil {
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 			}

// 			totalQtyReq := detail.Quantity
// 			totalQtyScanned := float64(0)
// 			newQtyScanned := float64(0)

// 			for _, inboundBarcode := range inboundBarcodesCheck {
// 				totalQtyScanned += inboundBarcode.Quantity
// 			}

// 			newQtyScanned = totalQtyReq - totalQtyScanned

// 			inputSerialNumber := detail.SerialNumber
// 			if invPolicy.UseSerialNumber && detail.SerialNumber != "" {
// 				inputSerialNumber = detail.SerialNumber
// 			}

// 			var location models.Location
// 			if err := c.DB.Debug().First(&location, "location_code = ?", detail.Location).Error; err != nil {
// 				if errors.Is(err, gorm.ErrRecordNotFound) {
// 					return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Location " + detail.Location + " not registered in system"})
// 				}
// 				return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 			}

// 			if newQtyScanned > 0 {
// 				newInboundBarcode := models.InboundBarcode{
// 					InboundId:       int(inboundHeader.ID),
// 					InboundDetailId: int(detail.ID),
// 					ItemCode:        detail.ItemCode,
// 					ItemID:          detail.ItemId,
// 					ScanData:        detail.Barcode,
// 					Barcode:         detail.Barcode,
// 					CartonNumber:    detail.CartonNumber,
// 					CaseNumber:      detail.CaseNumber,
// 					SerialNumber:    inputSerialNumber,
// 					Pallet:          palletID,
// 					Location:        location.LocationCode,
// 					Quantity:        newQtyScanned,
// 					WhsCode:         detail.WhsCode,
// 					OwnerCode:       detail.OwnerCode,
// 					DivisionCode:    detail.DivisionCode,
// 					QaStatus:        detail.QaStatus,
// 					Status:          "pending",
// 					Uom:             detail.Uom,
// 					RecDate:         detail.RecDate,
// 					ProdDate:        detail.ProdDate,
// 					ExpDate:         detail.ExpDate,
// 					LotNumber:       detail.LotNumber,
// 					CreatedBy:       int(ctx.Locals("userID").(float64)),
// 				}

// 				if err := c.DB.Debug().Create(&newInboundBarcode).Error; err != nil {
// 					return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 				}
// 			}
// 		}
// 	}

// 	// ambil semua yang pending, buat ditampilin ke FE sebagai preview sebelum confirm
// 	var inboundBarcodes []models.InboundBarcode
// 	if err := c.DB.Debug().Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").Find(&inboundBarcodes).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if len(inboundBarcodes) == 0 {
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scanned pending item not found"})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success":    true,
// 		"inbound_no": inboundHeader.InboundNo,
// 		"pallet":     palletID,
// 		"items":      inboundBarcodes,
// 	})
// }

func (c *InboundController) CheckPutawayByInboundNo(ctx *fiber.Ctx) error {
	var payload struct {
		InboundNo string `json:"inbound_no"`
	}
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	inboundHeader := models.InboundHeader{}
	if err := c.DB.Debug().First(&inboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var invPolicy models.InventoryPolicy
	if err := c.DB.Debug().First(&invPolicy, "owner_code = ?", inboundHeader.OwnerCode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var warehouse models.Warehouse
	if err := c.DB.Debug().First(&warehouse, "code = ?", inboundHeader.WhsCode).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if invPolicy.RequirePutawayScan {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Cannot putaway from your side, please putaway from scanner"})
	}

	var inboundDetail []models.InboundDetail
	if err := c.DB.Debug().Where("inbound_id = ?", inboundHeader.ID).Find(&inboundDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	inboundRepo := repositories.NewInboundRepository(c.DB)

	palletID, err := inboundRepo.GeneratePalletID(inboundHeader.InboundNo)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if !invPolicy.RequireReceiveScan {

		var allRcvLocationIsFilled bool = true
		for _, detail := range inboundDetail {
			if detail.Location == "" {
				allRcvLocationIsFilled = false
				break
			}
		}

		if !allRcvLocationIsFilled {
			return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Please fill all receiving location before putaway"})
		}

		// notFoundLocationMsg dipakai untuk membawa pesan error location
		// yang spesifik keluar dari closure transaction, supaya response
		// ke user tetap sama persis seperti versi original.
		var notFoundLocationMsg string

		txErr := c.DB.Transaction(func(tx *gorm.DB) error {
			for _, detail := range inboundDetail {

				var inboundBarcodesCheck []models.InboundBarcode
				if err := tx.Debug().Where("inbound_detail_id = ?", detail.ID).Find(&inboundBarcodesCheck).Error; err != nil {
					return err
				}

				var location models.Location
				if err := tx.Debug().First(&location, "location_code = ?", detail.Location).Error; err != nil {
					if errors.Is(err, gorm.ErrRecordNotFound) {
						// Simpan pesan errornya, lalu tetap return error supaya
						// GORM tahu transaction ini harus di-ROLLBACK.
						notFoundLocationMsg = "Location " + detail.Location + " not registered in system"
						return err
					}
					return err
				}

				// Ambil serial number untuk detail ini (kalau ada), tanpa syarat policy.
				// Kalau detail ini punya data di InboundSerial, selalu split
				// 1 InboundBarcode per serial dengan qty = 1.
				var inboundSerials []models.InboundSerial
				if err := tx.Debug().Where("inbound_detail_id = ?", detail.ID).Order("id ASC").Find(&inboundSerials).Error; err != nil {
					return err
				}

				if len(inboundSerials) > 0 {
					// ==== Punya serial: 1 InboundBarcode per serial, qty selalu 1 ====

					scannedSerials := make(map[string]bool, len(inboundBarcodesCheck))
					for _, b := range inboundBarcodesCheck {
						if b.SerialNumber != "" {
							scannedSerials[b.SerialNumber] = true
						}
					}

					for _, serial := range inboundSerials {
						if scannedSerials[serial.SerialNumber] {
							continue // serial ini sudah punya InboundBarcode
						}

						newInboundBarcode := models.InboundBarcode{
							InboundId:       int(inboundHeader.ID),
							InboundDetailId: detail.ID,
							ItemCode:        detail.ItemCode,
							ItemID:          detail.ItemId,
							ScanData:        detail.Barcode,
							Barcode:         detail.Barcode,
							CartonNumber:    detail.CartonNumber,
							CaseNumber:      detail.CaseNumber,
							SerialNumber:    serial.SerialNumber,
							Pallet:          palletID,
							Location:        location.LocationCode,
							Quantity:        1,
							WhsCode:         detail.WhsCode,
							OwnerCode:       detail.OwnerCode,
							DivisionCode:    detail.DivisionCode,
							QaStatus:        detail.QaStatus,
							Status:          "pending",
							Uom:             detail.Uom,
							RecDate:         detail.RecDate,
							ProdDate:        detail.ProdDate,
							ExpDate:         detail.ExpDate,
							LotNumber:       detail.LotNumber,
							CreatedBy:       int(ctx.Locals("userID").(float64)),
						}

						if err := tx.Debug().Create(&newInboundBarcode).Error; err != nil {
							return err
						}
					}

					continue
				}

				// ==== Tidak punya serial: behavior lama, tidak berubah ====

				totalQtyReq := detail.Quantity
				totalQtyScanned := float64(0)
				newQtyScanned := float64(0)

				for _, inboundBarcode := range inboundBarcodesCheck {
					totalQtyScanned += inboundBarcode.Quantity
				}

				newQtyScanned = totalQtyReq - totalQtyScanned

				inputSerialNumber := detail.SerialNumber
				if invPolicy.UseSerialNumber && detail.SerialNumber != "" {
					inputSerialNumber = detail.SerialNumber
				}

				if newQtyScanned > 0 {
					newInboundBarcode := models.InboundBarcode{
						InboundId:       int(inboundHeader.ID),
						InboundDetailId: detail.ID,
						ItemCode:        detail.ItemCode,
						ItemID:          detail.ItemId,
						ScanData:        detail.Barcode,
						Barcode:         detail.Barcode,
						CartonNumber:    detail.CartonNumber,
						CaseNumber:      detail.CaseNumber,
						SerialNumber:    inputSerialNumber,
						Pallet:          palletID,
						Location:        location.LocationCode,
						Quantity:        newQtyScanned,
						WhsCode:         detail.WhsCode,
						OwnerCode:       detail.OwnerCode,
						DivisionCode:    detail.DivisionCode,
						QaStatus:        detail.QaStatus,
						Status:          "pending",
						Uom:             detail.Uom,
						RecDate:         detail.RecDate,
						ProdDate:        detail.ProdDate,
						ExpDate:         detail.ExpDate,
						LotNumber:       detail.LotNumber,
						CreatedBy:       int(ctx.Locals("userID").(float64)),
					}

					if err := tx.Debug().Create(&newInboundBarcode).Error; err != nil {
						return err
					}
				}
			}

			// return nil di sini artinya semua baris berhasil diproses →
			// GORM otomatis COMMIT seluruh perubahan dalam transaction ini.
			return nil
		})

		// Kalau txErr != nil, GORM sudah otomatis ROLLBACK semua perubahan
		// yang terjadi di dalam transaction ini — tidak ada data yang
		// "kepotong separuh" seperti pada versi tanpa transaction.
		if txErr != nil {
			if notFoundLocationMsg != "" {
				return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": notFoundLocationMsg})
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": txErr.Error()})
		}
	}

	// ambil semua yang pending, buat ditampilin ke FE sebagai preview sebelum confirm
	var inboundBarcodes []models.InboundBarcode
	if err := c.DB.Debug().Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundBarcodes) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scanned pending item not found"})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success":    true,
		"inbound_no": inboundHeader.InboundNo,
		"pallet":     palletID,
		"items":      inboundBarcodes,
	})
}

// ---------- 2. CONFIRM PUTAWAY (jalanin servicePutawayPerItem) ----------

func (c *InboundController) ConfirmPutawayByInboundNo(ctx *fiber.Ctx) error {
	var payload struct {
		InboundNo string `json:"inbound_no"`
	}
	if err := ctx.BodyParser(&payload); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid payload"})
	}

	inboundHeader := models.InboundHeader{}
	if err := c.DB.Debug().First(&inboundHeader, "inbound_no = ?", payload.InboundNo).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Inbound not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var inboundBarcodes []models.InboundBarcode
	if err := c.DB.Debug().Where("inbound_id = ? AND status = ?", inboundHeader.ID, "pending").Find(&inboundBarcodes).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if len(inboundBarcodes) == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Scanned pending item not found"})
	}

	for _, barcode := range inboundBarcodes {
		barcodeIDStr := strconv.Itoa(int(barcode.ID))

		if err := c.servicePutawayPerItem(ctx, barcodeIDStr); err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error":   err.Error(),
				"message": err.Error(),
			})
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Putaway inbound " + inboundHeader.InboundNo + " successfully"})
}
