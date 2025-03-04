package controllers

import (
	"errors"
	"fiber-app/models"
	"fiber-app/repositories"
	"fmt"
	"log"
	"strings"
	"time"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type HandlingController struct {
	DB *gorm.DB
}

func NewHandlingController(db *gorm.DB) *HandlingController {
	return &HandlingController{DB: db}
}

func (c *HandlingController) Create(ctx *fiber.Ctx) error {

	fmt.Println("Apa ini", string(ctx.Body()))
	fmt.Println("Apa itu", string(ctx.Body()))
	// cara menghentikan kode disini dan melanjutkan ke catch error
	// return nil

	var handlingInput struct {
		Name    string `json:"name" validate:"required,min=3"`
		RateIdr int    `json:"rate_idr" validate:"required"`
	}

	if err := ctx.BodyParser(&handlingInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	handling := models.Handling{
		Name:      handlingInput.Name,
		Type:      "single",
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&handling).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Insert ke Handling rate
	handlingRate := models.HandlingRate{
		HandlingId: int(handling.ID),
		Name:       handlingInput.Name,
		RateIdr:    handlingInput.RateIdr,
		CreatedBy:  int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&handlingRate).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Insert ke Handling Combine
	handlingCombine := models.HandlingCombine{
		HandlingId: int(handling.ID),
		CreatedBy:  int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&handlingCombine).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// insert ke Handling Combine Detail
	handlingCombineDetail := models.HandlingCombineDetail{
		HandlingCombineId: int(handlingCombine.ID),
		HandlingId:        int(handling.ID),
		CreatedBy:         int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&handlingCombineDetail).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handling created successfully", "data": handling})
}

func (c *HandlingController) GetAll(ctx *fiber.Ctx) error {

	type handlingResponse struct {
		ID        uint      `json:"id"`
		Name      string    `json:"name"`
		Type      string    `json:"type"`
		RateIdr   int       `json:"rate_idr"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	var result []handlingResponse

	handlingRepo := repositories.NewHandlingRepository(c.DB)
	handlings, err := handlingRepo.GetHandlingRates()

	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// mapping handlingResponse dari handling
	for _, handling := range handlings {
		result = append(result, handlingResponse{
			ID:        uint(handling.HandlingID),
			Name:      handling.Name,
			Type:      handling.Type,
			RateIdr:   handling.RateIDR,
			UpdatedAt: handling.UpdatedAt,
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handlings found", "data": result})
}

func (c *HandlingController) GetAllOriginHandling(ctx *fiber.Ctx) error {

	type handlingResponse struct {
		ID        uint      `json:"id"`
		Name      string    `json:"name"`
		Type      string    `json:"type"`
		UpdatedAt time.Time `json:"updated_at"`
	}

	var result []models.Handling
	if err := c.DB.Where("type = ?", "single").Find(&result).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	var response []handlingResponse
	// mapping handlingResponse dari handling
	for _, handling := range result {
		response = append(response, handlingResponse{
			ID:        handling.ID,
			Name:      handling.Name,
			Type:      handling.Type,
			UpdatedAt: handling.UpdatedAt,
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handlings found", "data": response})
}

func (c *HandlingController) GetByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var handling models.Handling
	if err := c.DB.First(&handling, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Handling not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handling found", "data": handling})
}

func (c *HandlingController) Update(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")

	// Check if the ID is valid
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var handlingInput struct {
		Name    string `json:"name" validate:"required,min=3"`
		RateIdr int    `json:"rate_idr" validate:"required"`
	}

	var handling models.Handling

	// Check if the handling exists
	if err := c.DB.Where("id = ? AND type = ?", id, "single").First(&handling).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Handling not found for update"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse the request body
	if err := ctx.BodyParser(&handlingInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validate the input
	validate := validator.New()
	if err := validate.Struct(handlingInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	userID := int(ctx.Locals("userID").(float64))
	currentTime := time.Now()

	// Update the handling
	sqlUpdate := `UPDATE handlings SET name = ?, updated_by = ?, updated_at = ? WHERE id = ?`
	if err := c.DB.Exec(sqlUpdate, handlingInput.Name, userID, currentTime, id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Insert ke Handling Rate
	sqlInsert := `INSERT INTO handling_rates (handling_id, name, rate_idr, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?, ?)`
	if err := c.DB.Exec(sqlInsert, id, handlingInput.Name, handlingInput.RateIdr, userID, currentTime, currentTime).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Return the updated handling
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handling updated successfully", "data": handlingInput})
}

func (c *HandlingController) Delete(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var handling models.Handling
	// Check if the handling exists
	if err := c.DB.Where("id = ? AND type = ?", id, "single").First(&handling).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Handling not found for update"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	userID := int(ctx.Locals("userID").(float64))
	curentTime := time.Now()
	sqlDelete := `UPDATE handlings SET deleted_by = ?, deleted_at = ? WHERE id = ?`
	if err := c.DB.Exec(sqlDelete, userID, curentTime, id).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handling deleted successfully", "data": handling})
}

func (c *HandlingController) CreateCombineHandling(ctx *fiber.Ctx) error {

	fmt.Println("Raw Body:", string(ctx.Body()))

	type Item struct {
		Value int    `json:"value"`
		Label string `json:"label"`
	}

	type RequestPayload struct {
		Combine []Item `json:"combine"`
	}

	// Debug: Print raw body
	rawBody := string(ctx.Body())
	fmt.Println("Raw Body:", rawBody)

	var payload RequestPayload

	// Coba parsing JSON ke struct
	if err := ctx.BodyParser(&payload); err != nil {
		log.Println("Error parsing JSON:", err)
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// check payload harus minimum 2 item
	if len(payload.Combine) < 2 {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Combine handling must have at least 2 items"})
	}

	// Debug: Print hasil parsing
	fmt.Printf("Parsed Items: %+v\n", payload.Combine)

	// Slice untuk menyimpan label saja
	var labels []string

	for _, item := range payload.Combine {
		labels = append(labels, item.Label)
	}

	// Gabungkan dengan koma
	result := strings.Join(labels, ", ")

	fmt.Println(result)

	// Insert ke Handling
	handling := models.Handling{
		Name:      result,
		Type:      "combine",
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&handling).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Insert Ke Handling Combine
	handlingCombine := models.HandlingCombine{
		HandlingId: int(handling.ID),
		CreatedBy:  int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&handlingCombine).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Insert to handling combine details
	for _, item := range payload.Combine {
		handlingCombineDetail := models.HandlingCombineDetail{
			HandlingCombineId: int(handlingCombine.ID),
			HandlingId:        item.Value,
			CreatedBy:         int(ctx.Locals("userID").(float64)),
		}

		if err := c.DB.Create(&handlingCombineDetail).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	// sqlInsert := `INSERT INTO handlings (name, type, created_by, created_at, updated_at) VALUES (?, ?, ?, ?, ?)`

	// if err := c.DB.Exec(sqlInsert, result, "combine", 1, time.Now(), time.Now()).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Handling created successfully"})
}
