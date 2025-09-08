package controllers

import (
	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type HandlingController struct {
	DB *gorm.DB
}

type payloadItemHandling struct {
	ItemCode  string   `json:"item_code" validate:"required,min=3"`
	Handlings []string `json:"handlings" validate:"required,min=1"`
}

func NewHandlingController(db *gorm.DB) *HandlingController {
	return &HandlingController{DB: db}
}

func (c *HandlingController) Create(ctx *fiber.Ctx) error {
	var handlingInput struct {
		Name    string `json:"name" validate:"required,min=3"`
		RateIdr int    `json:"rate_idr" validate:"required"`
	}

	if err := ctx.BodyParser(&handlingInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	handling := models.MainVas{
		Name:      handlingInput.Name,
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&handling).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Insert ke Handling rate
	handlingRate := models.VasRate{
		MainVasId: int(handling.ID),
		RateIdr:   handlingInput.RateIdr,
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&handlingRate).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Insert ke Vas
	handlingCombine := models.Vas{
		Name:      handlingInput.Name,
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&handlingCombine).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// insert ke Vas Detail
	handlingCombineDetail := models.VasDetail{
		VasId:     int(handlingCombine.ID),
		MainVasId: int(handling.ID),
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&handlingCombineDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Vas created successfully", "data": handling})
}

// func (c *HandlingController) GetAll(ctx *fiber.Ctx) error {

// 	type handlingResponse struct {
// 		ID        uint      `json:"id"`
// 		Name      string    `json:"name"`
// 		Type      string    `json:"type"`
// 		RateIdr   int       `json:"rate_idr"`
// 		UpdatedAt time.Time `json:"updated_at"`
// 	}

// 	var result []handlingResponse

// 	handlingRepo := repositories.NewHandlingRepository(c.DB)
// 	handlings, err := handlingRepo.GetHandlingRates()

// 	if err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	if len(handlings) == 0 {
// 		result = []handlingResponse{}
// 		// return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Handlings not found", "data": result})
// 	}

// 	// mapping handlingResponse dari handling
// 	for _, handling := range handlings {
// 		result = append(result, handlingResponse{
// 			ID:        uint(handling.HandlingID),
// 			Name:      handling.Name,
// 			Type:      handling.Type,
// 			RateIdr:   handling.RateIDR,
// 			UpdatedAt: handling.UpdatedAt,
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Vas found", "data": result})
// }

// func (c *HandlingController) GetAllOriginHandling(ctx *fiber.Ctx) error {

// 	type handlingResponse struct {
// 		ID        uint      `json:"id"`
// 		Name      string    `json:"name"`
// 		Type      string    `json:"type"`
// 		UpdatedAt time.Time `json:"updated_at"`
// 	}

// 	var result []models.Handling
// 	if err := c.DB.Where("type = ?", "single").Find(&result).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	var response []handlingResponse
// 	// mapping handlingResponse dari handling
// 	for _, handling := range result {
// 		response = append(response, handlingResponse{
// 			ID:        handling.ID,
// 			Name:      handling.Name,
// 			Type:      handling.Type,
// 			UpdatedAt: handling.UpdatedAt,
// 		})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handlings found", "data": response})
// }

// func (c *HandlingController) GetByID(ctx *fiber.Ctx) error {
// 	id, err := ctx.ParamsInt("id")
// 	if err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
// 	}

// 	var handling models.Handling
// 	if err := c.DB.First(&handling, id).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Handling not found"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handling found", "data": handling})
// }

// func (c *HandlingController) Update(ctx *fiber.Ctx) error {
// 	id, err := ctx.ParamsInt("id")

// 	// Check if the ID is valid
// 	if err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
// 	}

// 	var handlingInput struct {
// 		Name    string `json:"name" validate:"required,min=3"`
// 		RateIdr int    `json:"rate_idr" validate:"required"`
// 	}

// 	var handling models.Handling

// 	// Check if the handling exists
// 	if err := c.DB.Where("id = ? AND type = ?", id, "single").First(&handling).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Handling not found for update"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Parse the request body
// 	if err := ctx.BodyParser(&handlingInput); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Validate the input
// 	validate := validator.New()
// 	if err := validate.Struct(handlingInput); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	userID := int(ctx.Locals("userID").(float64))
// 	currentTime := time.Now()

// 	// Update the handling
// 	sqlUpdate := `UPDATE handlings SET name = ?, updated_by = ?, updated_at = ? WHERE id = ?`
// 	if err := c.DB.Exec(sqlUpdate, handlingInput.Name, userID, currentTime, id).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Insert ke Handling Rate
// 	sqlInsert := `INSERT INTO handling_rates (handling_id, name, rate_idr, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
// 	if err := c.DB.Exec(sqlInsert, id, handlingInput.Name, handlingInput.RateIdr, userID, currentTime, currentTime).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Return the updated handling
// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handling updated successfully", "data": handlingInput})
// }

// func (c *HandlingController) Delete(ctx *fiber.Ctx) error {
// 	id, err := ctx.ParamsInt("id")
// 	if err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
// 	}

// 	var handling models.Handling
// 	// Check if the handling exists
// 	if err := c.DB.Where("id = ? AND type = ?", id, "single").First(&handling).Error; err != nil {
// 		if errors.Is(err, gorm.ErrRecordNotFound) {
// 			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Handling not found for update"})
// 		}
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	userID := int(ctx.Locals("userID").(float64))
// 	curentTime := time.Now()
// 	sqlDelete := `UPDATE handlings SET deleted_by = ?, deleted_at = ? WHERE id = ?`
// 	if err := c.DB.Exec(sqlDelete, userID, curentTime, id).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handling deleted successfully", "data": handling})
// }

// func (c *HandlingController) CreateCombineHandling(ctx *fiber.Ctx) error {

// 	fmt.Println("Raw Body:", string(ctx.Body()))

// 	type Item struct {
// 		Value int    `json:"value"`
// 		Label string `json:"label"`
// 	}

// 	type RequestPayload struct {
// 		Combine []Item `json:"combine"`
// 	}

// 	// Debug: Print raw body
// 	rawBody := string(ctx.Body())
// 	fmt.Println("Raw Body:", rawBody)

// 	var payload RequestPayload

// 	// Coba parsing JSON ke struct
// 	if err := ctx.BodyParser(&payload); err != nil {
// 		log.Println("Error parsing JSON:", err)
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// check payload harus minimum 2 item
// 	if len(payload.Combine) < 2 {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Combine handling must have at least 2 items"})
// 	}

// 	// Debug: Print hasil parsing
// 	fmt.Printf("Parsed Items: %+v\n", payload.Combine)

// 	// Slice untuk menyimpan label saja
// 	var labels []string

// 	for _, item := range payload.Combine {
// 		labels = append(labels, item.Label)
// 	}

// 	// Gabungkan dengan koma
// 	result := strings.Join(labels, ", ")

// 	fmt.Println(result)

// 	// Insert ke Handling
// 	handling := models.Handling{
// 		Name:      result,
// 		Type:      "combine",
// 		CreatedBy: int(ctx.Locals("userID").(float64)),
// 	}

// 	if err := c.DB.Create(&handling).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Insert Ke Handling Combine
// 	handlingCombine := models.HandlingCombine{
// 		HandlingId: int(handling.ID),
// 		CreatedBy:  int(ctx.Locals("userID").(float64)),
// 	}

// 	if err := c.DB.Create(&handlingCombine).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Insert to handling combine details
// 	for _, item := range payload.Combine {
// 		handlingCombineDetail := models.HandlingCombineDetail{
// 			HandlingCombineId: int(handlingCombine.ID),
// 			HandlingId:        item.Value,
// 			CreatedBy:         int(ctx.Locals("userID").(float64)),
// 		}

// 		if err := c.DB.Create(&handlingCombineDetail).Error; err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 	}

// 	// sqlInsert := `INSERT INTO handlings (name, type, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`

// 	// if err := c.DB.Exec(sqlInsert, result, "combine", 1, time.Now(), time.Now()).Error; err != nil {
// 	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	// }

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handling created successfully"})
// }

// func (c *HandlingController) CreateItemHandling(ctx *fiber.Ctx) error {
// 	var input payloadItemHandling

// 	if err := ctx.BodyParser(&input); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// insert to handling items and handling item details
// 	handlingItem := models.HandlingItem{
// 		ItemCode:  input.ItemCode,
// 		Area:      "DALAM KOTA",
// 		CreatedBy: int(ctx.Locals("userID").(float64)),
// 	}

// 	if err := c.DB.Create(&handlingItem).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	for _, detail := range input.Handlings {
// 		handlingItemDetail := models.HandlingItemDetail{
// 			HandlingItemId: int(handlingItem.ID),
// 			ItemCode:       input.ItemCode,
// 			Handling:       detail,
// 			CreatedBy:      int(ctx.Locals("userID").(float64)),
// 		}

// 		if err := c.DB.Create(&handlingItemDetail).Error; err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item handling created successfully"})
// }

// func (c *HandlingController) GetAllItemHandling(ctx *fiber.Ctx) error {

// 	var products []models.HandlingItem
// 	if err := c.DB.Preload("Details").Find(&products).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item Handlings found", "data": products})
// }
// func (c *HandlingController) GetItemHandlingByID(ctx *fiber.Ctx) error {

// 	var product models.HandlingItem
// 	// by handling id dengan preload
// 	if err := c.DB.Preload("Details").Where("id = ?", ctx.Params("id")).First(&product).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Detail Item Handlings found", "data": product})
// }

// func (c *HandlingController) UpdateItemHandlingByID(ctx *fiber.Ctx) error {
// 	var input payloadItemHandling
// 	id := ctx.Params("id")

// 	// Parse body
// 	if err := ctx.BodyParser(&input); err != nil {
// 		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Cari data induk
// 	var product models.HandlingItem
// 	if err := c.DB.First(&product, id).Error; err != nil {
// 		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Item not found"})
// 	}

// 	// Update induk
// 	product.ItemCode = input.ItemCode
// 	product.Area = "DALAM KOTA"
// 	product.UpdatedBy = int(ctx.Locals("userID").(float64))

// 	if err := c.DB.Save(&product).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Hapus semua detail lama
// 	if err := c.DB.Unscoped().Where("handling_item_id = ?", product.ID).Delete(&models.HandlingItemDetail{}).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Insert detail baru
// 	for _, h := range input.Handlings {
// 		detail := models.HandlingItemDetail{
// 			HandlingItemId: int(product.ID),
// 			ItemCode:       input.ItemCode,
// 			Handling:       h,
// 			CreatedBy:      int(ctx.Locals("userID").(float64)),
// 		}
// 		if err := c.DB.Create(&detail).Error; err != nil {
// 			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 		}
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
// 		"success": true,
// 		"message": "Item handling updated successfully",
// 		"data":    product,
// 	})
// }

// func (c *HandlingController) DeleteItemHandling(ctx *fiber.Ctx) error {
// 	id := ctx.Params("id")

// 	// Hapus detail terlebih dahulu
// 	if err := c.DB.Unscoped().Where("handling_item_id = ?", id).Delete(&models.HandlingItemDetail{}).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	// Hapus induk
// 	if err := c.DB.Unscoped().Delete(&models.HandlingItem{}, id).Error; err != nil {
// 		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
// 	}

// 	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Item handling deleted successfully"})
// }
