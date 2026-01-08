package controllers

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"strconv"
	"strings"

	"github.com/go-playground/validator"
	"github.com/gofiber/fiber/v2"
	"golang.org/x/crypto/bcrypt"
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

	// hashedPassword := userInput.Password

	hashedPassword, err := bcrypt.GenerateFromPassword(
		[]byte(userInput.Password),
		bcrypt.DefaultCost,
	)
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"error": "failed to hash password",
		})
	}

	user := models.User{
		Username:  userInput.Username,
		Name:      userInput.Name,
		Email:     userInput.Email,
		Password:  string(hashedPassword),
		BaseRoute: userInput.BaseRoute,
		CreatedBy: int(ctx.Locals("userID").(float64)),
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

		hashedPassword, err := bcrypt.GenerateFromPassword(
			[]byte(userInput.Password),
			bcrypt.DefaultCost,
		)
		if err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"error": "failed to hash password",
			})
		}

		user.Password = string(hashedPassword)
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
		ID        uint   `json:"id"`
		Username  string `json:"username"`
		Name      string `json:"name"`
		Email     string `json:"email"`
		CreatedAt string `json:"created_at"`
		UpdatedAt string `json:"updated_at"`
	}

	var user models.User
	if err := c.DB.First(&user, userID).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	userProfile.ID = user.ID
	userProfile.Username = user.Username
	userProfile.Name = user.Name
	userProfile.Email = user.Email
	userProfile.CreatedAt = user.CreatedAt.Format("2006-01-02")
	userProfile.UpdatedAt = user.UpdatedAt.Format("2006-01-02")
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

//================================================================
// BEGIN USER OWNER
//================================================================

type CreateUserOwnerRequest struct {
	UserID   uint   `json:"user_id" validate:"required"`
	OwnerIDs []uint `json:"owner_ids" validate:"required,min=1"`
}

type UpdateUserOwnerRequest struct {
	OwnerIDs []uint `json:"owner_ids" validate:"required,min=1"`
}

// CreateUserOwner - Assign multiple owners to a user
func (c *UserController) CreateUserOwner(ctx *fiber.Ctx) error {
	var req CreateUserOwnerRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Check if user exists
	var user models.User
	if err := c.DB.First(&user, req.UserID).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User not found",
		})
	}

	// Check if owners exist
	var owners []models.Owner
	if err := c.DB.Where("id IN ?", req.OwnerIDs).Find(&owners).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Error fetching owners",
			"error":   err.Error(),
		})
	}

	if len(owners) != len(req.OwnerIDs) {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Some owners not found",
		})
	}

	// Create UserOwner entries
	tx := c.DB.Begin()
	for _, owner := range owners {
		userOwner := models.UserOwner{
			UserID:    req.UserID,
			Username:  user.Username,
			OwnerID:   owner.ID,
			OwnerCode: owner.Code,
		}

		if err := tx.Create(&userOwner).Error; err != nil {
			tx.Rollback()
			// Check if already exists
			if err.Error() == "UNIQUE constraint failed" {
				continue
			}
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Error creating user-owner relationship",
				"error":   err.Error(),
			})
		}
	}
	tx.Commit()

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{
		"message": "User-Owner relationships created successfully",
		"data": fiber.Map{
			"user_id":   req.UserID,
			"owner_ids": req.OwnerIDs,
		},
	})
}

// GetUserOwners - Get all owners for a specific user
func (c *UserController) GetUserOwners(ctx *fiber.Ctx) error {
	userID, err := strconv.ParseUint(ctx.Params("userId"), 10, 32)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid user ID",
		})
	}

	var userOwners []models.UserOwner
	if err := c.DB.Preload("Owner").Where("user_id = ?", userID).Find(&userOwners).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Error fetching user owners",
			"error":   err.Error(),
		})
	}

	// Get owner details
	var ownerIDs []uint
	for _, uo := range userOwners {
		ownerIDs = append(ownerIDs, uo.OwnerID)
	}

	var owners []models.Owner
	if len(ownerIDs) > 0 {
		c.DB.Where("id IN ?", ownerIDs).Find(&owners)
	}

	return ctx.JSON(fiber.Map{
		"message": "User owners fetched successfully",
		"data": fiber.Map{
			"user_id": userID,
			"owners":  owners,
		},
	})
}

// GetAllUsers - Get all users with their owners
func (c *UserController) GetAllUserOwners(ctx *fiber.Ctx) error {
	var users []models.User
	if err := c.DB.Find(&users).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Error fetching users",
			"error":   err.Error(),
		})
	}

	type UserWithOwners struct {
		models.User
		Owners []models.Owner `json:"owners"`
	}

	var result []UserWithOwners
	for _, user := range users {
		var userOwners []models.UserOwner
		c.DB.Where("user_id = ?", user.ID).Find(&userOwners)

		var ownerIDs []uint
		for _, uo := range userOwners {
			ownerIDs = append(ownerIDs, uo.OwnerID)
		}

		var owners []models.Owner
		if len(ownerIDs) > 0 {
			c.DB.Where("id IN ?", ownerIDs).Find(&owners)
		}

		result = append(result, UserWithOwners{
			User:   user,
			Owners: owners,
		})
	}

	return ctx.JSON(fiber.Map{
		"message": "Users with owners fetched successfully",
		"data":    result,
	})
}

// UpdateUserOwners - Update owners for a user (replace all)
func (c *UserController) UpdateUserOwners(ctx *fiber.Ctx) error {
	userID, err := strconv.ParseUint(ctx.Params("userId"), 10, 32)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid user ID",
		})
	}

	var req UpdateUserOwnerRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Check if user exists
	var user models.User
	if err := c.DB.First(&user, userID).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User not found",
		})
	}

	// Check if owners exist
	var owners []models.Owner
	if err := c.DB.Where("id IN ?", req.OwnerIDs).Find(&owners).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Error fetching owners",
			"error":   err.Error(),
		})
	}

	if len(owners) != len(req.OwnerIDs) {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "Some owners not found",
		})
	}

	tx := c.DB.Begin()

	// Delete existing relationships
	if err := tx.Where("user_id = ?", userID).Delete(&models.UserOwner{}).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Error deleting existing relationships",
			"error":   err.Error(),
		})
	}

	// Create new relationships
	for _, owner := range owners {
		userOwner := models.UserOwner{
			UserID:    uint(userID),
			Username:  user.Username,
			OwnerID:   owner.ID,
			OwnerCode: owner.Code,
		}

		if err := tx.Create(&userOwner).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Error creating user-owner relationship",
				"error":   err.Error(),
			})
		}
	}

	tx.Commit()

	return ctx.JSON(fiber.Map{
		"message": "User owners updated successfully",
		"data": fiber.Map{
			"user_id":   userID,
			"owner_ids": req.OwnerIDs,
		},
	})
}

// DeleteUserOwner - Remove specific owner from user
func (c *UserController) DeleteUserOwner(ctx *fiber.Ctx) error {
	userID, err := strconv.ParseUint(ctx.Params("userId"), 10, 32)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid user ID",
		})
	}

	ownerID, err := strconv.ParseUint(ctx.Params("ownerId"), 10, 32)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid owner ID",
		})
	}

	result := c.DB.Where("user_id = ? AND owner_id = ?", userID, ownerID).Delete(&models.UserOwner{})
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Error deleting user-owner relationship",
			"error":   result.Error.Error(),
		})
	}

	if result.RowsAffected == 0 {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"message": "User-Owner relationship not found",
		})
	}

	return ctx.JSON(fiber.Map{
		"message": "User-Owner relationship deleted successfully",
	})
}

// DeleteAllUserOwners - Remove all owners from a user
func (c *UserController) DeleteAllUserOwners(ctx *fiber.Ctx) error {
	userID, err := strconv.ParseUint(ctx.Params("userId"), 10, 32)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"message": "Invalid user ID",
		})
	}

	result := c.DB.Where("user_id = ?", userID).Delete(&models.UserOwner{})
	if result.Error != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Error deleting user owners",
			"error":   result.Error.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"message": "All user owners deleted successfully",
		"data": fiber.Map{
			"deleted_count": result.RowsAffected,
		},
	})
}

// GetAvailableOwners - Get all owners for selection
func (c *UserController) GetAvailableOwners(ctx *fiber.Ctx) error {
	var owners []models.Owner
	if err := c.DB.Find(&owners).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"message": "Error fetching owners",
			"error":   err.Error(),
		})
	}

	return ctx.JSON(fiber.Map{
		"message": "Owners fetched successfully",
		"data":    owners,
	})
}

//================================================================
// END USER OWNER
//================================================================

//================================================================
// BEGIN UPDATE USER PROFILE
//================================================================

type UpdateUserProfileRequest struct {
	ID       uint   `json:"id"`
	Name     string `json:"name" validate:"required,min=2,max=100"`
	Email    string `json:"email" validate:"required,email"`
	Username string `json:"username" validate:"required,min=3,max=50,alphanum"`
	Password string `json:"password" validate:"omitempty,min=8,max=100"`
}

func (c *UserController) UpdateUserProfile(ctx *fiber.Ctx) error {
	// Get user ID from JWT/session context
	userID := ctx.Locals("userID")
	if userID == nil {
		return ctx.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
			"success": false,
			"message": "Unauthorized: User not authenticated",
		})
	}

	// Parse request body
	var req UpdateUserProfileRequest
	if err := ctx.BodyParser(&req); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
			"error":   err.Error(),
		})
	}

	// Validate request

	validate := validator.New()
	if err1 := validate.Struct(req); err1 != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"error":   err1.Error(),
			"success": false,
			"message": "Validation failed",
		})
	}

	// if err := c.Validate.Struct(req); err != nil {
	// 	validationErrors := err.(validator.ValidationErrors)
	// 	return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
	// 		"success": false,
	// 		"message": "Validation failed",
	// 		"errors":  formatValidationErrors(validationErrors),
	// 	})
	// }

	// Sanitize inputs
	req.Name = strings.TrimSpace(req.Name)
	req.Email = strings.TrimSpace(strings.ToLower(req.Email))
	req.Username = strings.TrimSpace(strings.ToLower(req.Username))

	// Begin transaction
	tx := c.DB.Begin()
	defer func() {
		if r := recover(); r != nil {
			tx.Rollback()
		}
	}()

	// Get existing user
	var user models.User
	if err := tx.Debug().First(&user, userID).Error; err != nil {
		tx.Rollback()
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
				"success": false,
				"message": "User not found",
			})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to fetch user data",
			"error":   err.Error(),
		})
	}

	fmt.Println("user: ", user)

	// Check if username is taken by another user
	if req.Username != user.Username {
		var count int64
		if err := tx.Model(&models.User{}).Where("username = ? AND id != ?", req.Username, userID).Count(&count).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to validate username",
				"error":   err.Error(),
			})
		}
		if count > 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Username already taken",
			})
		}
	}

	// Check if email is taken by another user
	if req.Email != user.Email {
		var count int64
		if err := tx.Model(&models.User{}).Where("email = ? AND id != ?", req.Email, userID).Count(&count).Error; err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to validate email",
				"error":   err.Error(),
			})
		}
		if count > 0 {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Email already registered",
			})
		}
	}

	// Update user fields
	user.Name = req.Name
	user.Email = req.Email
	user.Username = req.Username
	user.UpdatedBy = int(userID.(float64))

	// Hash and update password if provided
	if req.Password != "" {
		hashedPassword, err := bcrypt.GenerateFromPassword([]byte(req.Password), bcrypt.DefaultCost)
		if err != nil {
			tx.Rollback()
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to hash password",
				"error":   err.Error(),
			})
		}
		user.Password = string(hashedPassword)
	}

	// Save changes
	if err := tx.Debug().Save(&user).Error; err != nil {
		tx.Rollback()
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update user profile",
			"error":   err.Error(),
		})
	}

	// Commit transaction
	if err := tx.Commit().Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to commit changes",
			"error":   err.Error(),
		})
	}

	// Prepare response (exclude sensitive data)
	userResponse := fiber.Map{
		"id":         user.ID,
		"username":   user.Username,
		"name":       user.Name,
		"email":      user.Email,
		"created_at": user.CreatedAt,
		"updated_at": user.UpdatedAt,
	}

	// fmt.Println(userResponse)

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Profile updated successfully",
		"data":    userResponse,
	})
}

// Helper function to format validation errors
// func formatValidationErrors(errors validator.ValidationErrors) []fiber.Map {
// 	var formattedErrors []fiber.Map
// 	for _, err := range errors {
// 		formattedErrors = append(formattedErrors, fiber.Map{
// 			"field":   strings.ToLower(err.Field()),
// 			"message": getValidationMessage(err),
// 		})
// 	}
// 	return formattedErrors
// }

// Helper function to get user-friendly validation messages
// func getValidationMessage(err validator.FieldError) string {
// 	field := strings.ToLower(err.Field())
// 	switch err.Tag() {
// 	case "required":
// 		return field + " is required"
// 	case "email":
// 		return "Invalid email format"
// 	case "min":
// 		return field + " must be at least " + err.Param() + " characters"
// 	case "max":
// 		return field + " must not exceed " + err.Param() + " characters"
// 	case "alphanum":
// 		return field + " must contain only letters and numbers"
// 	default:
// 		return field + " is invalid"
// 	}
// }

//================================================================
// END UPDATE USER PROFILE
//================================================================
