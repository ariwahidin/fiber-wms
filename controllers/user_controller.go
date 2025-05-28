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

func (c *UserController) CreateUser(ctx *fiber.Ctx) error {
	var userInput struct {
		Username    string `json:"username" validate:"required,min=3"`
		Name        string `json:"name" validate:"required,min=3"`
		Email       string `json:"email" validate:"required,email"`
		Password    string `json:"password" validate:"required,min=6"`
		BaseRoute   string `json:"base_route"`
		Roles       []uint `json:"roles"`
		Permissions []uint `json:"permissions"`
	}

	// Parse Body
	if err := ctx.BodyParser(&userInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Validasi input
	validate := validator.New()
	if err := validate.Struct(userInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Hash password
	// hashedPassword, err := HashPassword(userInput.Password) // pastikan kamu punya fungsi ini
	// if err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash password"})
	// }

	hashedPassword := userInput.Password

	user := models.User{
		Username:  userInput.Username,
		Name:      userInput.Name,
		Email:     userInput.Email,
		Password:  hashedPassword,
		BaseRoute: userInput.BaseRoute,
		CreatedBy: int(ctx.Locals("userID").(float64)), // pastikan userID diset lewat JWT middleware misalnya
	}

	// Simpan user dulu
	if err := c.DB.Create(&user).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Assign Roles
	if len(userInput.Roles) > 0 {
		var roles []models.Role
		if err := c.DB.Where("id IN ?", userInput.Roles).Find(&roles).Error; err == nil {
			c.DB.Model(&user).Association("Roles").Replace(roles)
		}
	}

	// Assign Permissions
	if len(userInput.Permissions) > 0 {
		var perms []models.Permission
		if err := c.DB.Where("id IN ?", userInput.Permissions).Find(&perms).Error; err == nil {
			c.DB.Model(&user).Association("Permissions").Replace(perms)
		}
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"success": true,
		"message": "User created successfully",
	})
}

func (c *UserController) GetUserByID(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	var user models.User

	err := c.DB.
		Preload("Roles").
		Preload("Permissions").
		First(&user, id).Error

	if err != nil {
		return ctx.Status(500).JSON(fiber.Map{
			"error": err.Error(),
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"data":    user,
		"success": true,
	})
}

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

func (c *UserController) UpdateUser(ctx *fiber.Ctx) error {
	id := ctx.Params("id") // Ambil ID user dari route parameter

	var userInput struct {
		Username    string `json:"username" validate:"required,min=3"`
		Name        string `json:"name" validate:"required,min=3"`
		Email       string `json:"email" validate:"required,email"`
		Password    string `json:"password"` // opsional saat update
		BaseRoute   string `json:"base_route"`
		Roles       []uint `json:"roles"`
		Permissions []uint `json:"permissions"`
	}

	if err := ctx.BodyParser(&userInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	validate := validator.New()
	if err := validate.Struct(userInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Cek apakah user dengan ID ini ada
	var user models.User
	if err := c.DB.Preload("Roles").Preload("Permissions").First(&user, id).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "User not found"})
	}

	// Update field
	user.Username = userInput.Username
	user.Name = userInput.Name
	user.Email = userInput.Email
	user.BaseRoute = userInput.BaseRoute
	user.UpdatedBy = int(ctx.Locals("userID").(float64))

	// Jika password tidak kosong, update
	if userInput.Password != "" {
		// hashedPassword, err := HashPassword(userInput.Password)
		// if err != nil {
		//     return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to hash password"})
		// }
		user.Password = userInput.Password
	}

	// Simpan perubahan user
	if err := c.DB.Save(&user).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Update Roles
	if userInput.Roles != nil {
		var roles []models.Role
		if err := c.DB.Where("id IN ?", userInput.Roles).Find(&roles).Error; err == nil {
			c.DB.Model(&user).Association("Roles").Replace(roles)
		}
	}

	// Update Permissions
	if userInput.Permissions != nil {
		var perms []models.Permission
		if err := c.DB.Where("id IN ?", userInput.Permissions).Find(&perms).Error; err == nil {
			c.DB.Model(&user).Association("Permissions").Replace(perms)
		}
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "User updated successfully",
	})
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

func (c *UserController) CreateRole(ctx *fiber.Ctx) error {
	var role models.Role
	if err := ctx.BodyParser(&role); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	role.CreatedBy = int(ctx.Locals("userID").(float64))
	role.CreatedAt = ctx.Context().Time()
	result := c.DB.Create(&role)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Role created successfully"})
}

func (c *UserController) GetRoles(ctx *fiber.Ctx) error {
	// var roles []models.Role
	// if err := c.DB.Find(&roles).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }
	// return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"data": roles, "success": true})

	var roles []models.Role
	if err := c.DB.Preload("Permissions").Find(&roles).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    roles,
	})
}

func (c *UserController) GetRoleByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var role models.Role
	if err := c.DB.First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Role not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"data": role, "success": true})
}

func (c *UserController) UpdateRole(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var role models.Role
	if err := c.DB.First(&role, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Role not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := ctx.BodyParser(&role); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	role.UpdatedBy = int(ctx.Locals("userID").(float64))
	role.UpdatedAt = ctx.Context().Time()
	result := c.DB.Save(&role)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Role updated successfully"})
}

// Permission

func (c *UserController) GetPermissions(ctx *fiber.Ctx) error {
	var permissions []models.Permission
	if err := c.DB.Find(&permissions).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"data": permissions, "success": true})
}

func (c *UserController) GetPermissionByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var permission models.Permission
	if err := c.DB.First(&permission, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Permission not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"data": permission, "success": true})
}

func (c *UserController) CreatePermission(ctx *fiber.Ctx) error {
	var permission models.Permission
	if err := ctx.BodyParser(&permission); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	permission.CreatedBy = int(ctx.Locals("userID").(float64))
	permission.CreatedAt = ctx.Context().Time()
	result := c.DB.Create(&permission)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Permission created successfully"})
}

func (c *UserController) UpdatePermission(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}
	var permission models.Permission
	if err := c.DB.First(&permission, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Permission not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	if err := ctx.BodyParser(&permission); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}
	permission.UpdatedBy = int(ctx.Locals("userID").(float64))
	permission.UpdatedAt = ctx.Context().Time()
	result := c.DB.Save(&permission)
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": result.Error.Error()})
	}
	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Permission updated successfully"})
}

func (c *UserController) UpdatePermissionsForRole(ctx *fiber.Ctx) error {
	// Ambil ID dari parameter URL
	roleID, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid role ID"})
	}

	// Cari role-nya
	var role models.Role
	if err := c.DB.Preload("Permissions").First(&role, roleID).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Role not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Parse request body
	var body struct {
		PermissionIds []uint `json:"permissionIds"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	// Ambil daftar permission dari DB berdasarkan ID yang diberikan
	var permissions []models.Permission
	if len(body.PermissionIds) > 0 {
		if err := c.DB.Where("id IN ?", body.PermissionIds).Find(&permissions).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	// Update relasi role_permission (replace)
	if err := c.DB.Model(&role).Association("Permissions").Replace(&permissions); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "Failed to update permissions"})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Permissions updated successfully for role",
	})
}
