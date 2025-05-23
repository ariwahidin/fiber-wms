package routes

import (
	"fiber-app/controllers"
	"fiber-app/middleware"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func SetupInboundRoutes(app *fiber.App, inboundController *controllers.InboundController, db *gorm.DB) {

	inboundMidleware := middleware.NewAuthMiddleware(db)

	api := app.Group("/api/v1/inbound", middleware.AuthMiddleware, inboundMidleware.CheckPermission("create_inbound"))

	api.Post("/upload", inboundController.UploadInboundFromExcel)

	api.Get("/create", inboundController.PreapareInbound)

	api.Get("/:id", inboundController.GetInboundByID)
	api.Get("/putaway/sheet/:id", inboundController.GetPutawaySheet)

	api.Post("/", inboundController.SaveHeaderInbound)
	api.Get("/", inboundController.GetAllListInbound)
	api.Put("/:id", inboundController.UpdateInboundByID)
	api.Put("/detail/:id", inboundController.UpdateDetailByID)
	api.Post("/detail/", inboundController.CreateOrUpdateItemInbound)
	api.Get("/detail/draft", inboundController.GetInboundDetailDraftByUserID)
	api.Delete("/detail/:id", inboundController.DeleteInboundDetail)
	api.Post("/complete/:id", inboundController.ProcessingInboundComplete)
}
