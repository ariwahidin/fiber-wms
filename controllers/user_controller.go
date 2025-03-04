package controllers

import (
	"errors"
	"fiber-app/models"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type UserController struct {
	DB *gorm.DB
}

func NewUserController(DB *gorm.DB) *UserController {
	return &UserController{DB: DB}
}

// Create user
func (c *UserController) CreateUser(ctx *fiber.Ctx) error {

	var userInput struct {
		Username string `json:"username" validate:"required,min=3"`
		Name     string `json:"name" validate:"required,min=3"`
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=6"`
	}

	// Parse Body
	if err := ctx.BodyParser(&userInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi input menggunakan validator
	validate := validator.New()
	if err := validate.Struct(userInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Membuat user dengan memasukkan data ke struct models.User
	user := models.User{
		Username:  userInput.Username,
		Name:      userInput.Name,
		Email:     userInput.Email,
		Password:  userInput.Password,
		CreatedBy: int(ctx.Locals("userID").(float64)),
	}

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	result := c.DB.Select("username", "name", "email", "password", "created_by").Create(&user)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}

	// Respons sukses
	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "User created successfully"})

}

// Get user by ID
func (c *UserController) GetUserByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var result models.User
	if err := c.DB.First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	result.Password = ""

	return ctx.Status(fiber.StatusOK).JSON(result)
}

// Get all users
func (c *UserController) GetAllUsers(ctx *fiber.Ctx) error {
	var users []models.User
	if err := c.DB.Find(&users).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	for i := range users {
		users[i].Password = ""
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"data":    users,
		"total":   len(users),
		"success": true,
	})
}

// Update user
func (c *UserController) UpdateUser(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var userInput struct {
		Username string `json:"username" validate:"required,min=3"`
		Name     string `json:"name" validate:"required,min=3"`
		Email    string `json:"email" validate:"required,email"`
		Password string `json:"password" validate:"required,min=6"`
	}

	// Parse Body
	if err := ctx.BodyParser(&userInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi input menggunakan validator
	validate := validator.New()
	if err := validate.Struct(userInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Membuat user dengan memasukkan data ke struct models.User
	user := models.User{
		Username:  userInput.Username,
		Name:      userInput.Name,
		Email:     userInput.Email,
		Password:  userInput.Password,
		UpdatedBy: int(ctx.Locals("userID").(float64)),
	}

	user.UpdatedAt = ctx.Context().Time()

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	result := c.DB.Select("username", "name", "email", "password", "updated_by", "updated_at").Where("id = ?", id).Updates(&user)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}

	// Respons sukses
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"message": "User updated successfully"})

}

// Delete user
func (c *UserController) DeleteUser(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	// Periksa apakah user dengan ID tersebut ada
	var user models.User
	if err := c.DB.First(&user, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Add UserID to DeletedBy field
	user.DeletedBy = int(ctx.Locals("userID").(float64))

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	result := c.DB.Select("deleted_by").Where("id = ?", id).Updates(&user)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}

	// Hapus user
	result = c.DB.Delete(&user)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}

	// Respons sukses
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"message": "User deleted successfully"})
}

// Get Profile
func (c *UserController) GetProfile(ctx *fiber.Ctx) error {
	userID := int(ctx.Locals("userID").(float64))

	var userProfile struct {
		Username string `json:"username"`
		Name     string `json:"name"`
		Email    string `json:"email"`
	}

	var user models.User
	if err := c.DB.First(&user, userID).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	userProfile.Username = user.Username
	userProfile.Name = user.Name
	userProfile.Email = user.Email
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"data": userProfile, "success": true})
}
