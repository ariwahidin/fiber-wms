// database/seeder.go
package database

import (
	"errors"
	"fiber-app/config"
	"fiber-app/controllers/idgen"
	"fiber-app/models"
	"fiber-app/models/notification"
	"fiber-app/models/report_builder"
	"fiber-app/models/report_mailer"
	"fiber-app/types"
	"fmt"
	"log"
	"time"

	"gorm.io/gorm"
)

func RunSeeders(db *gorm.DB) {
	SeedMenus(db)
	SeedUoms(db)
	// SeedWarehouse(db)
	SeedUserMaster(db)
	SeedCategory(db)
	// SeedDivision(db)
	SeedMasterCartons(db)
	SeedEmailNotification(db)
	SeedReportBuilder(db)
	seedReasonCodes(db)
	RunCompanyConfigMigration(db)
}

func SeedUnit(db *gorm.DB) {
	unit := models.BusinessUnit{
		DbName: config.DBUnit,
	}

	var existing models.BusinessUnit
	err := db.Where("db_name = ?", unit.DbName).First(&existing).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			if err := db.Create(&unit).Error; err != nil {
				log.Fatalf("Failed to create unit: %v", err)
			}
		} else {
			log.Fatalf("Unexpected DB error: %v", err)
		}
	}
}

func SeedCategory(db *gorm.DB) {
	categories := []models.Category{
		{
			Code: "PRODUCT",
			Name: "PRODUCT",
		},
		{
			Code: "SPAREPART",
			Name: "SPAREPART",
		},
	}

	for _, c := range categories {
		var existing models.Category
		if err := db.Where("name = ?", c.Name).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				db.Create(&c)
			}
		}
	}

}

func SeedUoms(db *gorm.DB) {
	uoms := []models.Uom{
		{Code: "PCS", Name: "PCS"},
	}

	for _, u := range uoms {
		var existing models.Uom
		if err := db.Where("code = ?", u.Code).First(&existing).Error; err != nil {
			if err == gorm.ErrRecordNotFound {
				u.ID = uint(types.SnowflakeID(idgen.GenerateID()))
				db.Create(&u)
			}
		}
	}
}

func SeedMenus(db *gorm.DB) error {
	menus := []models.Menu{
		{
			Name:      "Master Data",
			Path:      "#",
			Icon:      "Database",
			MenuOrder: 1,
		},
		{
			Name:      "Product",
			Path:      "/wms/master/product",
			Icon:      "Box",
			MenuOrder: 1,
			ParentID:  getMenuIDByName(db, "Master Data"), // ambil ID parent
		},
		{
			Name:      "Supplier",
			Path:      "/wms/master/supplier",
			Icon:      "Truck",
			MenuOrder: 2,
			ParentID:  getMenuIDByName(db, "Master Data"),
		},
		{
			Name:      "Handling",
			Path:      "/wms/master/handling",
			Icon:      "Truck",
			MenuOrder: 3,
			ParentID:  getMenuIDByName(db, "Master Data"),
		},
	}

	for _, menu := range menus {
		var existing models.Menu
		err := db.Where("name = ? AND path = ?", menu.Name, menu.Path).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&menu).Error; err != nil {
				log.Println("Gagal insert menu:", menu.Name, err)
			} else {
				log.Println("Insert menu:", menu.Name)
			}
		}
	}

	return nil
}

func SeedUserMaster(db *gorm.DB) {
	users := []models.User{
		{
			Username:  "admin",
			Password:  "admin",
			Name:      "Admin",
			Email:     "admin@example.com",
			BaseRoute: "/dashboard",
			// Role:     "admin",
		},
	}

	for _, user := range users {
		var existing models.User
		err := db.Where("email = ?", user.Email).First(&existing).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			if err := db.Create(&user).Error; err != nil {
				log.Println("Gagal insert user:", user.Username, err)
			} else {
				log.Println("Insert user:", user.Username)
			}
		}
	}
}

func getMenuIDByName(db *gorm.DB, name string) *uint {
	var parent models.Menu
	err := db.Where("name = ?", name).First(&parent).Error
	if err == nil {
		id := uint(parent.ID)
		return &id
	}
	return nil
}

func SeedMasterCartons(db *gorm.DB) error {
	masterCartons := []models.MasterCarton{
		{
			CartonCode:  "CTN-01",
			CartonName:  "Carton 01",
			Description: "Small carton for lightweight items",
			Length:      30,
			Width:       20,
			Height:      15,
			MaxWeight:   5,
			TareWeight:  0.5,
			IsActive:    true,
			IsDefault:   false,
			Material:    "Cardboard",
			Color:       "Brown",
		},
		// {
		// 	CartonCode:  "CTN-MEDIUM",
		// 	CartonName:  "Medium Box",
		// 	Description: "Standard medium-sized carton",
		// 	Length:      40,
		// 	Width:       30,
		// 	Height:      25,
		// 	MaxWeight:   10,
		// 	TareWeight:  0.8,
		// 	IsActive:    true,
		// 	IsDefault:   true, // Default carton
		// 	Material:    "Cardboard",
		// 	Color:       "Brown",
		// },
		// {
		// 	CartonCode:  "CTN-LARGE",
		// 	CartonName:  "Large Box",
		// 	Description: "Large carton for bulky items",
		// 	Length:      60,
		// 	Width:       40,
		// 	Height:      40,
		// 	MaxWeight:   20,
		// 	TareWeight:  1.2,
		// 	IsActive:    true,
		// 	IsDefault:   false,
		// 	Material:    "Cardboard",
		// 	Color:       "Brown",
		// },
		// {
		// 	CartonCode:  "CTN-XLARGE",
		// 	CartonName:  "Extra Large Box",
		// 	Description: "Extra large carton for very bulky items",
		// 	Length:      80,
		// 	Width:       60,
		// 	Height:      50,
		// 	MaxWeight:   30,
		// 	TareWeight:  2.0,
		// 	IsActive:    true,
		// 	IsDefault:   false,
		// 	Material:    "Cardboard",
		// 	Color:       "Brown",
		// },
		// {
		// 	CartonCode:  "CTN-CUSTOM",
		// 	CartonName:  "Custom Box",
		// 	Description: "Custom sized carton",
		// 	Length:      50,
		// 	Width:       35,
		// 	Height:      30,
		// 	MaxWeight:   15,
		// 	TareWeight:  1.0,
		// 	IsActive:    true,
		// 	IsDefault:   false,
		// 	Material:    "Cardboard",
		// 	Color:       "White",
		// },
	}

	for _, carton := range masterCartons {
		// Check if carton already exists
		var existing models.MasterCarton
		err := db.Where("carton_code = ?", carton.CartonCode).First(&existing).Error

		if err == gorm.ErrRecordNotFound {
			// Create new carton
			if err := db.Create(&carton).Error; err != nil {
				return err
			}
		}
	}

	return nil
}

func SeedReportMailer(db *gorm.DB) error {
	// Cek apakah sudah ada data (hindari double seed)
	var count int64
	db.Model(&report_mailer.Report{}).Count(&count)
	if count > 0 {
		fmt.Println("✅ Report mailer seed sudah ada, skip.")
		return nil
	}

	// 1. Seed Email Config dummy
	emailConfig := report_mailer.EmailConfig{
		Name:      "SMTP Default",
		Host:      "smtp.example.com",
		Port:      587,
		Username:  "noreply@example.com",
		Password:  "password123",
		FromName:  "WMS Report",
		FromEmail: "noreply@example.com",
		UseTLS:    true,
		IsActive:  true,
		CreatedBy: 1,
	}
	if err := db.Create(&emailConfig).Error; err != nil {
		return fmt.Errorf("seed email config error: %w", err)
	}

	// 2. Seed Report dummy
	// Query ini pakai tabel yang hampir pasti ada di SQL Server
	// Ganti dengan query sesuai kebutuhan Anda nanti dari UI
	report := report_mailer.Report{
		Name:        "Dummy Report - Stock Summary",
		Description: "Report dummy untuk testing. Silakan ganti query sesuai kebutuhan.",
		// Query:         "SELECT TOP 10 TABLE_NAME, TABLE_TYPE FROM INFORMATION_SCHEMA.TABLES ORDER BY TABLE_NAME",
		OutputMode:    report_mailer.OutputModeSingleFile,
		EmailConfigID: emailConfig.ID,
		ExcelTitle:    "Stock Summary Report",
		ExcelSubtitle: "Data ringkasan stok warehouse",
		IsActive:      true,
		CreatedBy:     1,
	}
	if err := db.Create(&report).Error; err != nil {
		return fmt.Errorf("seed report error: %w", err)
	}

	// 3. Seed Recipient dummy
	recipients := []report_mailer.ReportRecipient{
		{
			ReportID: report.ID,
			Email:    "manager@example.com",
			Name:     "Manager",
			Type:     report_mailer.RecipientTO,
		},
		{
			ReportID: report.ID,
			Email:    "supervisor@example.com",
			Name:     "Supervisor",
			Type:     report_mailer.RecipientCC,
		},
	}
	if err := db.Create(&recipients).Error; err != nil {
		return fmt.Errorf("seed recipients error: %w", err)
	}

	// 4. Seed Schedule dummy - Daily jam 08:00
	hour := 8
	minute := 0
	nextRun := time.Now().Truncate(24 * time.Hour).Add(time.Duration(hour) * time.Hour)
	schedule := report_mailer.ReportSchedule{
		ReportID:  report.ID,
		Frequency: report_mailer.FrequencyDaily,
		Hour:      hour,
		Minute:    minute,
		IsActive:  false, // default nonaktif dulu sampai email config diisi yang benar
		NextRunAt: &nextRun,
	}
	if err := db.Create(&schedule).Error; err != nil {
		return fmt.Errorf("seed schedule error: %w", err)
	}

	fmt.Println("✅ Report mailer seed berhasil.")
	return nil
}

func SeedEmailNotification(db *gorm.DB) error {
	// Cek apakah sudah ada seed
	var count int64
	db.Model(&notification.EmailNotification{}).Count(&count)
	if count > 0 {
		return nil
	}

	// Ambil email config pertama yang aktif
	var configID uint = 1 // sesuaikan kalau perlu

	// Seed notifikasi outbound completed
	outboundNotif := notification.EmailNotification{
		Name:          "Outbound Completed",
		EventKey:      "outbound.completed",
		Description:   "Notifikasi dikirim ketika proses picking outbound selesai",
		EmailConfigID: configID,
		EmailSubject:  "[WMS] Outbound {{outbound_no}} Has Been Completed",
		EmailHeader:   "Outbound Completion Notice",
		EmailBody: `Dear Team,

<p>We would like to inform you that the following outbound order has been successfully completed.</p>

<table style="border-collapse:collapse;width:100%;font-size:13px;margin:16px 0;border:1px solid #E5E7EB;border-radius:8px;overflow:hidden;">
  <thead>
    <tr style="background:#F8FAFC;">
      <th style="text-align:left;padding:10px 16px;color:#6B7280;font-weight:600;border-bottom:1px solid #E5E7EB;width:40%;">Detail</th>
      <th style="text-align:left;padding:10px 16px;color:#6B7280;font-weight:600;border-bottom:1px solid #E5E7EB;">Information</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td style="padding:10px 16px;color:#6B7280;border-bottom:1px solid #F3F4F6;">Outbound No</td>
      <td style="padding:10px 16px;font-weight:600;color:#111827;border-bottom:1px solid #F3F4F6;">{{outbound_no}}</td>
    </tr>
    <tr style="background:#FAFAFA;">
      <td style="padding:10px 16px;color:#6B7280;border-bottom:1px solid #F3F4F6;">Owner</td>
      <td style="padding:10px 16px;color:#374151;border-bottom:1px solid #F3F4F6;">{{owner_code}}</td>
    </tr>
    <tr>
      <td style="padding:10px 16px;color:#6B7280;border-bottom:1px solid #F3F4F6;">Customer</td>
      <td style="padding:10px 16px;color:#374151;border-bottom:1px solid #F3F4F6;">{{customer_code}}</td>
    </tr>
    <tr style="background:#FAFAFA;">
      <td style="padding:10px 16px;color:#6B7280;border-bottom:1px solid #F3F4F6;">Deliver To</td>
      <td style="padding:10px 16px;color:#374151;border-bottom:1px solid #F3F4F6;">{{deliv_to}}</td>
    </tr>
    <tr>
      <td style="padding:10px 16px;color:#6B7280;border-bottom:1px solid #F3F4F6;">Picker</td>
      <td style="padding:10px 16px;color:#374151;border-bottom:1px solid #F3F4F6;">{{picker_name}}</td>
    </tr>
    <tr style="background:#FAFAFA;">
      <td style="padding:10px 16px;color:#6B7280;border-bottom:1px solid #F3F4F6;">Truck No</td>
      <td style="padding:10px 16px;color:#374151;border-bottom:1px solid #F3F4F6;">{{truck_no}}</td>
    </tr>
    <tr>
      <td style="padding:10px 16px;color:#6B7280;border-bottom:1px solid #F3F4F6;">Driver</td>
      <td style="padding:10px 16px;color:#374151;border-bottom:1px solid #F3F4F6;">{{driver}}</td>
    </tr>
    <tr style="background:#FAFAFA;">
      <td style="padding:10px 16px;color:#6B7280;">Completed At</td>
      <td style="padding:10px 16px;color:#374151;">{{complete_time}}</td>
    </tr>
  </tbody>
</table>

<p style="color:#374151;">Please ensure all post-completion procedures are followed accordingly.</p>
<p style="color:#374151;">Thank you.</p>`,
		EmailFooter: `© {{year}} Warehouse Management System. This is an automated message — please do not reply directly to this email.`,
		HeaderColor: "#1E3A5F",
		IsActive:    false, // default nonaktif, user aktifkan manual
	}

	if err := db.Create(&outboundNotif).Error; err != nil {
		return err
	}

	// Seed notifikasi inbound completed
	inboundNotif := notification.EmailNotification{
		Name:          "Inbound Completed",
		EventKey:      "inbound.completed",
		Description:   "Notifikasi dikirim ketika proses penerimaan inbound selesai",
		EmailConfigID: configID,
		EmailSubject:  "[WMS] Inbound {{inbound_no}} Has Been Completed",
		EmailHeader:   "Inbound Completion Notice",
		EmailBody: `Dear Team,

<p>The following inbound receipt has been successfully completed and inventory has been updated accordingly.</p>

<table style="border-collapse:collapse;width:100%;font-size:13px;margin:16px 0;border:1px solid #E5E7EB;border-radius:8px;overflow:hidden;">
  <thead>
    <tr style="background:#F8FAFC;">
      <th style="text-align:left;padding:10px 16px;color:#6B7280;font-weight:600;border-bottom:1px solid #E5E7EB;width:40%;">Detail</th>
      <th style="text-align:left;padding:10px 16px;color:#6B7280;font-weight:600;border-bottom:1px solid #E5E7EB;">Information</th>
    </tr>
  </thead>
  <tbody>
    <tr>
      <td style="padding:10px 16px;color:#6B7280;border-bottom:1px solid #F3F4F6;">Inbound No</td>
      <td style="padding:10px 16px;font-weight:600;color:#111827;border-bottom:1px solid #F3F4F6;">{{inbound_no}}</td>
    </tr>
    <tr style="background:#FAFAFA;">
      <td style="padding:10px 16px;color:#6B7280;border-bottom:1px solid #F3F4F6;">Owner</td>
      <td style="padding:10px 16px;color:#374151;border-bottom:1px solid #F3F4F6;">{{owner_code}}</td>
    </tr>
    <tr>
      <td style="padding:10px 16px;color:#6B7280;border-bottom:1px solid #F3F4F6;">Warehouse</td>
      <td style="padding:10px 16px;color:#374151;border-bottom:1px solid #F3F4F6;">{{whs_code}}</td>
    </tr>
    <tr style="background:#FAFAFA;">
      <td style="padding:10px 16px;color:#6B7280;">Completed At</td>
      <td style="padding:10px 16px;color:#374151;">{{complete_time}}</td>
    </tr>
  </tbody>
</table>

<p style="color:#374151;">Thank you.</p>`,
		EmailFooter: `© {{year}} Warehouse Management System. This is an automated message — please do not reply directly to this email.`,
		HeaderColor: "#065F46",
		IsActive:    false,
	}

	if err := db.Create(&inboundNotif).Error; err != nil {
		return err
	}

	return nil
}

func SeedReportBuilder(db *gorm.DB) error {
	reports := []struct {
		report report_builder.RptReport
		fields []report_builder.RptReportField
	}{
		{
			report: report_builder.RptReport{
				ReportCode: "INVENTORY",
				ReportName: "Inventory Stock",
				ReportType: "QUERY",
				BaseQuery: `SELECT
					a.whs_code, a.location, a.owner_code, a.rec_date,
					b.item_code, b.item_name, b.category, b.cbm,
					a.qa_status, a.uom,
					SUM(a.qty_origin) AS qty_in,
					SUM(a.qty_onhand) AS qty_onhand,
					SUM(a.qty_available) AS qty_available,
					SUM(a.qty_allocated) AS qty_allocated,
					SUM(a.qty_shipped) AS qty_out,
					b.cbm * SUM(a.qty_available) AS cbm_total
				FROM inventories a
				INNER JOIN products b ON a.item_id = b.id
				WHERE a.qty_origin > 0
				GROUP BY a.whs_code, a.location, b.item_code, b.item_name,
					a.qa_status, a.uom, a.owner_code, a.rec_date, b.category, b.cbm`,
				IsActive: true,
			},
			fields: []report_builder.RptReportField{
				{FieldKey: "whs_code", FieldLabel: "Warehouse", FieldType: "STRING", IsFilterable: true, IsSortable: true, SortOrder: 1},
				{FieldKey: "location", FieldLabel: "Location", FieldType: "STRING", IsFilterable: true, IsSortable: true, SortOrder: 2},
				{FieldKey: "owner_code", FieldLabel: "Owner", FieldType: "STRING", IsFilterable: true, IsSortable: true, SortOrder: 3},
				{FieldKey: "item_code", FieldLabel: "Item Code", FieldType: "STRING", IsFilterable: true, IsSortable: true, SortOrder: 4},
				{FieldKey: "item_name", FieldLabel: "Item Name", FieldType: "STRING", IsFilterable: true, IsSortable: true, SortOrder: 5},
				{FieldKey: "category", FieldLabel: "Category", FieldType: "STRING", IsFilterable: true, IsSortable: true, SortOrder: 6},
				{FieldKey: "uom", FieldLabel: "UOM", FieldType: "STRING", IsFilterable: true, IsSortable: true, SortOrder: 7},
				{FieldKey: "qa_status", FieldLabel: "QA Status", FieldType: "STRING", IsFilterable: true, IsSortable: true, SortOrder: 8},
				{FieldKey: "rec_date", FieldLabel: "Receive Date", FieldType: "DATE", IsFilterable: true, IsSortable: true, SortOrder: 9},
				{FieldKey: "qty_in", FieldLabel: "Qty In", FieldType: "NUMBER", IsFilterable: false, IsSortable: true, SortOrder: 10},
				{FieldKey: "qty_onhand", FieldLabel: "Qty On Hand", FieldType: "NUMBER", IsFilterable: false, IsSortable: true, SortOrder: 11},
				{FieldKey: "qty_available", FieldLabel: "Qty Available", FieldType: "NUMBER", IsFilterable: false, IsSortable: true, SortOrder: 12},
				{FieldKey: "qty_allocated", FieldLabel: "Qty Allocated", FieldType: "NUMBER", IsFilterable: false, IsSortable: true, SortOrder: 13},
				{FieldKey: "qty_out", FieldLabel: "Qty Out", FieldType: "NUMBER", IsFilterable: false, IsSortable: true, SortOrder: 14},
				{FieldKey: "cbm", FieldLabel: "CBM/pcs", FieldType: "NUMBER", IsFilterable: false, IsSortable: false, SortOrder: 15},
				{FieldKey: "cbm_total", FieldLabel: "CBM Total", FieldType: "NUMBER", IsFilterable: false, IsSortable: true, SortOrder: 16},
			},
		},
		{
			report: report_builder.RptReport{
				ReportCode:   "PICKING_LIST",
				ReportName:   "Picking List",
				ReportType:   "DOCUMENT",
				DocumentType: "PICKING_LIST",
				BaseQuery: `SELECT
					oh.doc_no, oh.doc_date, oh.owner_code, oh.whs_code,
					c.item_code, c.item_name, c.uom,
					b.qty_request, b.qty_picked,
					b.location, b.batch_no
				FROM outbound_headers oh
				INNER JOIN outbound_details b ON oh.id = b.outbound_id
				INNER JOIN products c ON b.item_id = c.id
				WHERE oh.id = :outbound_id
				ORDER BY b.location, c.item_code`,
				IsActive: true,
			},
			fields: []report_builder.RptReportField{
				{FieldKey: "doc_no", FieldLabel: "Document No", FieldType: "STRING", IsFilterable: false, IsSortable: false, SortOrder: 1},
				{FieldKey: "doc_date", FieldLabel: "Document Date", FieldType: "DATE", IsFilterable: false, IsSortable: false, SortOrder: 2},
				{FieldKey: "owner_code", FieldLabel: "Owner", FieldType: "STRING", IsFilterable: false, IsSortable: false, SortOrder: 3},
				{FieldKey: "whs_code", FieldLabel: "Warehouse", FieldType: "STRING", IsFilterable: false, IsSortable: false, SortOrder: 4},
				{FieldKey: "location", FieldLabel: "Location", FieldType: "STRING", IsFilterable: false, IsSortable: false, SortOrder: 5},
				{FieldKey: "item_code", FieldLabel: "Item Code", FieldType: "STRING", IsFilterable: false, IsSortable: false, SortOrder: 6},
				{FieldKey: "item_name", FieldLabel: "Item Name", FieldType: "STRING", IsFilterable: false, IsSortable: false, SortOrder: 7},
				{FieldKey: "uom", FieldLabel: "UOM", FieldType: "STRING", IsFilterable: false, IsSortable: false, SortOrder: 8},
				{FieldKey: "qty_request", FieldLabel: "Qty Request", FieldType: "NUMBER", IsFilterable: false, IsSortable: false, SortOrder: 9},
				{FieldKey: "qty_picked", FieldLabel: "Qty Picked", FieldType: "NUMBER", IsFilterable: false, IsSortable: false, SortOrder: 10},
				{FieldKey: "batch_no", FieldLabel: "Batch No", FieldType: "STRING", IsFilterable: false, IsSortable: false, SortOrder: 11},
			},
		},
	}

	for _, item := range reports {
		// Skip kalau sudah ada
		var existing report_builder.RptReport
		if err := db.Where("report_code = ?", item.report.ReportCode).First(&existing).Error; err == nil {
			continue
		}

		// Create report
		if err := db.Create(&item.report).Error; err != nil {
			return err
		}

		// Create fields
		for i := range item.fields {
			item.fields[i].ReportID = item.report.ID
		}
		if err := db.Create(&item.fields).Error; err != nil {
			return err
		}
	}

	return nil
}

func RunCompanyConfigMigration(db *gorm.DB) error {
	// err := db.AutoMigrate(&models.CompanyConfig{})
	// if err != nil {
	// 	log.Fatalf("Failed to migrate CompanyConfig: %v", err)
	// }
	// log.Println("CompanyConfig migration completed")

	// Seed default config kalau tabel masih kosong
	var count int64
	db.Model(&models.CompanyConfig{}).Count(&count)
	if count == 0 {
		seed := models.CompanyConfig{
			CompanyName:  "PT Yusen Logistics Interlink Indonesia",
			CompanyShort: "Yusen Logistics",
			AppName:      "YuTrackWMS",
			Tagline:      "Track Everything in Warehouse",
			LogoURL:      "/uploads/company/logo/default.png",
			PrimaryColor: "#041F5F",
			AccentColor:  "#1A50C8",
			LoginTheme:   "ThemeModern",
			LoginSlides: models.JSONSlides{
				{ImageURL: "/images/wms_cover.jpeg", Title: "Warehouse Management", Subtitle: "Efficient inventory control"},
				{ImageURL: "/images/truck_yusen2.jpeg", Title: "Transport Management", Subtitle: "Seamless delivery tracking"},
				{ImageURL: "/images/warehouse_staff.jpeg", Title: "Smart Logistics", Subtitle: "End-to-end solutions"},
				{ImageURL: "/images/drone.jpeg", Title: "Scalable System", Subtitle: "Grow your warehouse without complexity"},
				{ImageURL: "/images/tms_cover.jpeg", Title: "System Integration", Subtitle: "Connected transport and warehouse flow"},
			},
		}
		if err := db.Create(&seed).Error; err != nil {
			log.Printf("Failed to seed CompanyConfig: %v", err)
		} else {
			log.Println("CompanyConfig seeded with default values")
		}
	}
	return nil
}

func seedReasonCodes(db *gorm.DB) {
	codes := []models.AdjustmentReasonCode{
		{Code: "DMGD", Description: "Damaged goods", Direction: "out", RequireNote: true},
		{Code: "EXPD", Description: "Expired / past expiry date", Direction: "out", RequireNote: true},
		{Code: "OPNAME", Description: "Stock count result", Direction: "both", RequireNote: false},
		{Code: "RECV_ERR", Description: "Receiving discrepancy", Direction: "both", RequireNote: true},
		{Code: "SYS_ERR", Description: "System correction", Direction: "both", RequireNote: true},
		{Code: "SHRINK", Description: "Unknown loss / shrinkage", Direction: "out", RequireNote: true},
		{Code: "FOUND", Description: "Found unrecorded stock", Direction: "in", RequireNote: true},
		{Code: "PROD_LOSS", Description: "Production / repacking loss", Direction: "out", RequireNote: false},
	}
	for _, c := range codes {
		db.Where("code = ?", c.Code).FirstOrCreate(&c)
	}
}
