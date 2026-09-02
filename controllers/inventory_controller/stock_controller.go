package inventory_controller

import (
	"encoding/json"
	"fiber-app/models"
	"fmt"
	"strconv"
	"strings"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

// ===== Shared filter helper =====

// Query params yang berlaku untuk ketiga endpoint (cards, grouped, detail)
type baseFilters struct {
	Location string
	Category string
	Group    string
	QaStatus string
	Search   string
}

func parseBaseFilters(ctx *fiber.Ctx) baseFilters {
	return baseFilters{
		Location: ctx.Query("location"),
		Category: ctx.Query("category"),
		Group:    ctx.Query("group"),
		QaStatus: ctx.Query("qa_status"),
		Search:   ctx.Query("search"),
	}
}

// applyBaseFilters menempel WHERE ke query dasar (inventories JOIN products)
func applyBaseFilters(db *gorm.DB, f baseFilters) *gorm.DB {
	q := db.Where("inventories.qty_available > ?", 0)
	if f.Location != "" {
		q = q.Where("inventories.location = ?", f.Location)
	}
	if f.Category != "" {
		q = q.Where("products.category = ?", f.Category) // sesuaikan nama kolom Product
	}
	if f.Group != "" {
		q = q.Where("products.[group] = ?", f.Group) // sesuaikan nama kolom Product
	}
	if f.QaStatus != "" {
		q = q.Where("inventories.qa_status = ?", f.QaStatus)
	}
	if f.Search != "" {
		like := "%" + f.Search + "%"
		q = q.Where(
			"inventories.item_code LIKE ? OR products.item_name LIKE ? OR inventories.barcode LIKE ? OR inventories.location LIKE ? OR inventories.division_code LIKE ?",
			like, like, like, like, like,
		)
	}
	return q
}

// parseColumnFilters decode query param `filters` (JSON object: {"item_code":"ABC",...})
func parseColumnFilters(ctx *fiber.Ctx) map[string]string {
	raw := ctx.Query("filters")
	if raw == "" {
		return map[string]string{}
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return map[string]string{}
	}
	return m
}

func parsePaging(ctx *fiber.Ctx) (page, pageSize int) {
	page, _ = strconv.Atoi(ctx.Query("page", "1"))
	pageSize, _ = strconv.Atoi(ctx.Query("page_size", "50"))
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 500 {
		pageSize = 50
	}
	return
}

// ===== 1. Summary cards (agregat, cepat, tidak ikut paging) =====

func (c *InventoryController) GetAvailableSummaryCards(ctx *fiber.Ctx) error {
	f := parseBaseFilters(ctx)
	base := applyBaseFilters(
		c.DB.Table("inventories").Joins("JOIN products ON products.id = inventories.item_id"),
		f,
	)

	var totals struct {
		TotalAvailable float64
		TotalOnhand    float64
		TotalAllocated float64
	}
	if err := base.Session(&gorm.Session{}).
		Select(`
			COALESCE(SUM(inventories.qty_available), 0) as total_available,
			COALESCE(SUM(inventories.qty_onhand), 0)    as total_onhand,
			COALESCE(SUM(inventories.qty_allocated), 0) as total_allocated
		`).
		Scan(&totals).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to compute summary"})
	}

	var totalGroups int64
	if err := base.Session(&gorm.Session{}).
		Select(`COUNT(DISTINCT CONCAT_WS('|',
			inventories.item_code, inventories.whs_code, inventories.location, inventories.division_code,
			inventories.qa_status, inventories.rec_date, inventories.prod_date, inventories.exp_date, inventories.lot_number
		))`).
		Scan(&totalGroups).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to compute group count"})
	}

	var totalRecords int64
	base.Session(&gorm.Session{}).Count(&totalRecords)

	return ctx.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"total_groups":    totalGroups,
			"total_records":   totalRecords,
			"total_available": totals.TotalAvailable,
			"total_onhand":    totals.TotalOnhand,
			"total_allocated": totals.TotalAllocated,
		},
	})
}

// ===== 2. Grouped / Summary tab (paginated, server-side sort + filter) =====

var groupedRawColumns = map[string]string{
	"location":      "inventories.location",
	"item_code":     "inventories.item_code",
	"item_name":     "products.item_name",
	"category":      "products.category",
	"group":         "products.[group]",
	"qa_status":     "inventories.qa_status",
	"division_code": "inventories.division_code",
	"rec_date":      "inventories.rec_date",
	"prod_date":     "inventories.prod_date",
	"exp_date":      "inventories.exp_date",
	"lot_number":    "inventories.lot_number",
}

var groupedAggColumns = map[string]string{
	"total_qty_available": "SUM(inventories.qty_available)",
	"total_qty_onhand":    "SUM(inventories.qty_onhand)",
	"total_qty_allocated": "SUM(inventories.qty_allocated)",
	"inventory_count":     "COUNT(*)",
}

func (c *InventoryController) GetAvailableGrouped(ctx *fiber.Ctx) error {
	f := parseBaseFilters(ctx)
	colFilters := parseColumnFilters(ctx)
	page, pageSize := parsePaging(ctx)

	sortBy := ctx.Query("sort_by", "item_code")
	sortDir := strings.ToLower(ctx.Query("sort_dir", "asc"))
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "asc"
	}

	groupByCols := []string{
		"inventories.location", "inventories.item_code", "products.item_name", "inventories.barcode",
		"products.category", "products.[group]", "inventories.qa_status", "inventories.division_code",
		"inventories.uom", "inventories.rec_date", "inventories.prod_date", "inventories.exp_date", "inventories.lot_number",
	}

	base := applyBaseFilters(
		c.DB.Table("inventories").Joins("JOIN products ON products.id = inventories.item_id"),
		f,
	)

	// WHERE untuk kolom mentah (bukan agregat)
	for key, val := range colFilters {
		if val == "" {
			continue
		}
		if col, ok := groupedRawColumns[key]; ok {
			base = base.Where(fmt.Sprintf("%s LIKE ?", col), "%"+val+"%")
		}
	}

	// HAVING untuk kolom agregat
	having := ""
	havingArgs := []interface{}{}
	for key, val := range colFilters {
		if val == "" {
			continue
		}
		if expr, ok := groupedAggColumns[key]; ok {
			if having != "" {
				having += " AND "
			}
			having += fmt.Sprintf("CAST(%s AS CHAR) LIKE ?", expr)
			havingArgs = append(havingArgs, "%"+val+"%")
		}
	}

	grouped := base.Session(&gorm.Session{}).
		Select(strings.Join(groupByCols, ", ") + `,
			SUM(inventories.qty_available) as total_qty_available,
			SUM(inventories.qty_onhand)    as total_qty_onhand,
			SUM(inventories.qty_allocated) as total_qty_allocated,
			COUNT(*)                       as inventory_count
		`).
		Group(strings.Join(groupByCols, ", "))
	if having != "" {
		grouped = grouped.Having(having, havingArgs...)
	}

	// Total count (jumlah grup, bukan jumlah record)
	var totalGroups int64
	countQuery := c.DB.Table("(?) as g", grouped).Session(&gorm.Session{})
	countQuery.Count(&totalGroups)

	// Order + paging
	orderCol := sortBy
	if col, ok := groupedRawColumns[sortBy]; ok {
		orderCol = col
	} else if expr, ok := groupedAggColumns[sortBy]; ok {
		orderCol = expr
	} else {
		orderCol = "inventories.item_code"
	}

	type row struct {
		Location          string  `json:"location"`
		ItemCode          string  `json:"item_code"`
		ItemName          string  `json:"item_name"`
		Barcode           string  `json:"barcode"`
		Category          string  `json:"category"`
		Group             string  `json:"group"`
		QaStatus          string  `json:"qa_status"`
		DivisionCode      string  `json:"division_code"`
		Uom               string  `json:"uom"`
		RecDate           string  `json:"rec_date"`
		ProdDate          string  `json:"prod_date"`
		ExpDate           string  `json:"exp_date"`
		LotNumber         string  `json:"lot_number"`
		TotalQtyAvailable float64 `json:"total_qty_available"`
		TotalQtyOnhand    float64 `json:"total_qty_onhand"`
		TotalQtyAllocated float64 `json:"total_qty_allocated"`
		InventoryCount    int64   `json:"inventory_count"`
	}
	var rows []row
	if err := grouped.
		Order(fmt.Sprintf("%s %s", orderCol, sortDir)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Scan(&rows).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch grouped inventory"})
	}

	return ctx.JSON(fiber.Map{
		"success":   true,
		"data":      rows,
		"total":     totalGroups,
		"page":      page,
		"page_size": pageSize,
	})
}

// ===== 3. Detail tab (paginated, server-side sort + filter) =====

var detailColumns = map[string]string{
	"inventory_number": "inventories.inventory_number",
	"location":         "inventories.location",
	"whs_code":         "inventories.whs_code",
	"division_code":    "inventories.division_code",
	"owner_code":       "inventories.owner_code",
	"item_code":        "inventories.item_code",
	"item_name":        "products.item_name",
	"barcode":          "inventories.barcode",
	"category":         "products.category",
	"group":            "products.[group]",
	"qa_status":        "inventories.qa_status",
	"lot_number":       "inventories.lot_number",
	"pallet":           "inventories.pallet",
	"carton_number":    "inventories.carton_number",
	"case_number":      "inventories.case_number",
	"serial_number":    "inventories.serial_number",
	"qty_onhand":       "inventories.qty_onhand",
	"qty_available":    "inventories.qty_available",
	"qty_allocated":    "inventories.qty_allocated",
	"qty_suspend":      "inventories.qty_suspend",
	"qty_shipped":      "inventories.qty_shipped",
}

func (c *InventoryController) GetAvailableInventoryDetail(ctx *fiber.Ctx) error {
	f := parseBaseFilters(ctx)
	colFilters := parseColumnFilters(ctx)
	page, pageSize := parsePaging(ctx)

	sortBy := ctx.Query("sort_by", "item_code")
	sortDir := strings.ToLower(ctx.Query("sort_dir", "asc"))
	if sortDir != "asc" && sortDir != "desc" {
		sortDir = "asc"
	}

	base := applyBaseFilters(
		c.DB.Table("inventories").Joins("JOIN products ON products.id = inventories.item_id"),
		f,
	)

	for key, val := range colFilters {
		if val == "" {
			continue
		}
		if col, ok := detailColumns[key]; ok {
			base = base.Where(fmt.Sprintf("%s LIKE ?", col), "%"+val+"%")
		}
	}

	var total int64
	base.Session(&gorm.Session{}).Count(&total)

	orderCol, ok := detailColumns[sortBy]
	if !ok {
		orderCol = "inventories.item_code"
	}

	var inventories []models.Inventory
	if err := base.
		Preload("Product").
		Order(fmt.Sprintf("%s %s", orderCol, sortDir)).
		Limit(pageSize).
		Offset((page - 1) * pageSize).
		Find(&inventories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch inventory detail"})
	}

	result := make([]InventoryDetailRow, 0, len(inventories))
	for _, inv := range inventories {
		result = append(result, InventoryDetailRow{
			InventoryNumber: inv.InventoryNumber,
			Location:        inv.Location,
			WhsCode:         inv.WhsCode,
			DivisionCode:    inv.DivisionCode,
			OwnerCode:       inv.OwnerCode,
			ItemCode:        inv.ItemCode,
			ItemName:        inv.Product.ItemName,
			Barcode:         inv.Barcode,
			Category:        inv.Product.Category,
			Group:           inv.Product.Group,
			QaStatus:        inv.QaStatus,
			Uom:             inv.Uom,
			RecDate:         inv.RecDate,
			ProdDate:        inv.ProdDate,
			ExpDate:         inv.ExpDate,
			LotNumber:       inv.LotNumber,
			Pallet:          inv.Pallet,
			CartonNumber:    inv.CartonNumber,
			CaseNumber:      inv.CaseNumber,
			SerialNumber:    inv.SerialNumber,
			QtyOrigin:       inv.QtyOrigin,
			QtyOnhand:       inv.QtyOnhand,
			QtyAvailable:    inv.QtyAvailable,
			QtyAllocated:    inv.QtyAllocated,
			QtySuspend:      inv.QtySuspend,
			QtyShipped:      inv.QtyShipped,
		})
	}

	return ctx.JSON(fiber.Map{
		"success":   true,
		"data":      result,
		"total":     total,
		"page":      page,
		"page_size": pageSize,
	})
}

// ===== 4. Filter options (distinct values untuk dropdown) =====

func (c *InventoryController) GetFilterOptions(ctx *fiber.Ctx) error {
	f := parseBaseFilters(ctx)
	base := applyBaseFilters(
		c.DB.Table("inventories").Joins("JOIN products ON products.id = inventories.item_id"),
		f,
	)

	var locations, categories, groups, qaStatuses []string

	if err := base.Session(&gorm.Session{}).
		Distinct("inventories.location").
		Where("inventories.location IS NOT NULL AND inventories.location != ''").
		Order("inventories.location ASC").
		Pluck("inventories.location", &locations).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch location options"})
	}

	if err := base.Session(&gorm.Session{}).
		Distinct("products.category").
		Where("products.category IS NOT NULL AND products.category != ''").
		Order("products.category ASC").
		Pluck("products.category", &categories).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch category options"})
	}

	// if err := base.Session(&gorm.Session{}).
	// 	Distinct("products.group").
	// 	Where("products.group IS NOT NULL AND products.group != ''").
	// 	Order("products.group ASC").
	// 	Pluck("products.group", &groups).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch group options"})
	// }

	if err := base.Session(&gorm.Session{}).
		Distinct("products.[group]").
		Where("products.[group] IS NOT NULL AND products.[group] != ''").
		Order("products.[group] ASC").
		Pluck("products.[group]", &groups).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch group options"})
	}

	if err := base.Session(&gorm.Session{}).
		Distinct("inventories.qa_status").
		Where("inventories.qa_status IS NOT NULL AND inventories.qa_status != ''").
		Order("inventories.qa_status ASC").
		Pluck("inventories.qa_status", &qaStatuses).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"success": false, "message": "Failed to fetch qa_status options"})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data": fiber.Map{
			"location":  locations,
			"category":  categories,
			"group":     groups,
			"qa_status": qaStatuses,
		},
	})
}
