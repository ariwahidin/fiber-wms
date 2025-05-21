package controllers

import (
	"strconv"

	"fiber-app/models" // ganti dengan path models-mu

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type MenuController struct {
	DB *gorm.DB
}

func NewMenuController(DB *gorm.DB) *MenuController {
	return &MenuController{DB: DB}
}

func (mc *MenuController) GetAllMenus(ctx *fiber.Ctx) error {
	var menus []models.Menu
	err := mc.DB.Preload("Children").Preload("Permissions").Find(&menus).Error
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"data": menus, "success": true})

}

// GetMenus ambil menu root beserta children dan permissions
func (mc *MenuController) GetMenus(ctx *fiber.Ctx) error {
	var menus []models.Menu
	err := mc.DB.
		Debug().
		Preload("Children").
		Preload("Permissions").
		Where("parent_id IS NULL").
		Order("menu_order asc").
		Find(&menus).Error
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}
	return ctx.JSON(fiber.Map{"data": menus, "success": true})
}

func (mc *MenuController) GetMenuUser(ctx *fiber.Ctx) error {
	var menus []models.Menu
	err := mc.DB.
		Preload("Children").
		Where("parent_id IS NULL").
		Order("menu_order asc").
		Find(&menus).Error
	if err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Map ke bentuk frontend
	var result []map[string]interface{}
	for _, menu := range menus {
		children := []map[string]interface{}{}
		for _, child := range menu.Children {
			children = append(children, map[string]interface{}{
				"title": child.Name,
				"url":   child.Path,
			})
		}

		result = append(result, map[string]interface{}{
			"title": menu.Name,
			"url":   menu.Path,
			"icon":  menu.Icon, // pastikan icon-nya string, misalnya "InboxIcon"
			// "isActive": menu.IsActive, // boolean
			"isActive": true,
			"items":    children, // anak-anak menu
		})
	}

	return ctx.JSON(fiber.Map{
		"success": true,
		"data":    result,
	})
}

// GetMenuByID ambil menu berdasarkan ID, termasuk children dan permissions
func (mc *MenuController) GetMenuByID(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	menuID, err := strconv.Atoi(id)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var menu models.Menu
	err = mc.DB.Preload("Children").Preload("Permissions").First(&menu, menuID).Error
	if err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Menu not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(menu)
}

// CreateMenu input data baru
func (mc *MenuController) CreateMenu(ctx *fiber.Ctx) error {
	type MenuInput struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Icon        string `json:"icon"`
		Order       int    `json:"order"`
		ParentID    *uint  `json:"parent_id"`
		Permissions []uint `json:"permissions"`
	}

	var input MenuInput
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	// Ambil permissions
	var permissions []models.Permission
	if len(input.Permissions) > 0 {
		if err := mc.DB.Where("id IN ?", input.Permissions).Find(&permissions).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}

	menu := models.Menu{
		Name:        input.Name,
		Path:        input.Path,
		Icon:        input.Icon,
		MenuOrder:   input.Order,
		ParentID:    input.ParentID,
		Permissions: permissions,
	}

	if err := mc.DB.Create(&menu).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusCreated).JSON(fiber.Map{"message": "Menu created successfully", "data": menu, "success": true})
}

// UpdateMenu update data menu berdasarkan ID
func (mc *MenuController) UpdateMenu(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	menuID, err := strconv.Atoi(id)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var menu models.Menu
	if err := mc.DB.Preload("Permissions").First(&menu, menuID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Menu not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	type MenuInput struct {
		Name        string `json:"name"`
		Path        string `json:"path"`
		Icon        string `json:"icon"`
		Order       int    `json:"order"`
		ParentID    *uint  `json:"parent_id"`
		Permissions []uint `json:"permissions"`
	}

	var input MenuInput
	if err := ctx.BodyParser(&input); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid input"})
	}

	menu.Name = input.Name
	menu.Path = input.Path
	menu.Icon = input.Icon
	menu.MenuOrder = input.Order
	menu.ParentID = input.ParentID

	// Update permissions
	var permissions []models.Permission
	if len(input.Permissions) > 0 {
		if err := mc.DB.Where("id IN ?", input.Permissions).Find(&permissions).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
		}
	}
	menu.Permissions = permissions

	if err := mc.DB.Save(&menu).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"message": "Menu updated successfully", "data": menu, "success": true})
}

// DeleteMenu hapus menu berdasarkan ID
func (mc *MenuController) DeleteMenu(ctx *fiber.Ctx) error {
	id := ctx.Params("id")
	menuID, err := strconv.Atoi(id)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var menu models.Menu
	if err := mc.DB.First(&menu, menuID).Error; err != nil {
		if err == gorm.ErrRecordNotFound {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Menu not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	if err := mc.DB.Delete(&menu).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.JSON(fiber.Map{"message": "Menu deleted successfully", "success": true})
}

func (c *MenuController) GetMenuPermission(ctx *fiber.Ctx) error {
	idParam := ctx.Params("id")
	permissionID, err := strconv.Atoi(idParam)
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid permission ID",
		})
	}

	var permission models.Permission
	if err := c.DB.Preload("Menus").First(&permission, permissionID).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Permission not found",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"data":    permission.Menus,
	})
}

func (pc *MenuController) UpdatePermissionMenus(ctx *fiber.Ctx) error {
	permissionID, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid permission ID",
		})
	}

	// Payload struct untuk parsing body JSON
	var body struct {
		MenuIDs []uint `json:"menu_ids"`
	}
	if err := ctx.BodyParser(&body); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{
			"success": false,
			"message": "Invalid request body",
		})
	}

	// Load permission dari DB
	var permission models.Permission
	if err := pc.DB.First(&permission, permissionID).Error; err != nil {
		return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{
			"success": false,
			"message": "Permission not found",
		})
	}

	// Load menus yang dipilih dari DB (validasi apakah menu_ids valid)
	var menus []models.Menu
	if len(body.MenuIDs) > 0 {
		if err := pc.DB.Where("id IN ?", body.MenuIDs).Find(&menus).Error; err != nil {
			return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"success": false,
				"message": "Failed to fetch menus",
			})
		}
	}

	// Update relasi many2many permission_menus: GORM akan hapus relasi lama & set relasi baru
	if err := pc.DB.Model(&permission).Association("Menus").Replace(menus); err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
			"success": false,
			"message": "Failed to update permission menus",
		})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{
		"success": true,
		"message": "Permission menus updated successfully",
		"data":    menus, // optional: bisa kirim kembali data menu yang sudah di-set
	})
}
