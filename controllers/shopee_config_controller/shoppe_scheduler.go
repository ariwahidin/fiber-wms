package shopee_config_controller

import (
	scheduler "fiber-app/shceduler"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type ShopeeConfigController struct {
	DB      *gorm.DB
	QueryDB *gorm.DB
}

func NewShopeeConfigController(db *gorm.DB, queryDB *gorm.DB) *ShopeeConfigController {
	return &ShopeeConfigController{DB: db, QueryDB: queryDB}
}

func (c *ShopeeConfigController) StartScheduler(ctx *fiber.Ctx) error {
	if err := scheduler.StartShopeeScheduler(c.DB, c.QueryDB); err != nil {
		return ctx.Status(400).JSON(fiber.Map{"success": false, "message": err.Error()})
	}
	return ctx.JSON(fiber.Map{"success": true, "message": "Scheduler berhasil dijalankan"})
}

func (c *ShopeeConfigController) StopScheduler(ctx *fiber.Ctx) error {
	scheduler.StopShopeeScheduler()
	return ctx.JSON(fiber.Map{"success": true, "message": "Scheduler dihentikan"})
}

func (c *ShopeeConfigController) GetSchedulerStatus(ctx *fiber.Ctx) error {
	return ctx.JSON(fiber.Map{
		"success": true,
		"running": scheduler.IsSchedulerRunning(),
	})
}
