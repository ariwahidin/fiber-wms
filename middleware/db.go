package middleware

import (
	"fiber-app/controllers/configurations"
	"reflect"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

func InjectDBMiddleware(controller interface{}) fiber.Handler {
	return func(c *fiber.Ctx) error {
		dbName, ok := c.Locals("unit").(string)
		if !ok || dbName == "" {
			return fiber.NewError(fiber.StatusInternalServerError, "database name not found in context")
		}

		db, err := configurations.GetDBConnection(dbName)
		if err != nil {
			return fiber.NewError(fiber.StatusInternalServerError, "error connecting to database")
		}

		// Inject ke field DB di controller
		val := reflect.ValueOf(controller)
		if val.Kind() != reflect.Ptr || val.IsNil() {
			return fiber.NewError(fiber.StatusInternalServerError, "controller must be a non-nil pointer")
		}

		elem := val.Elem()
		dbField := elem.FieldByName("DB")
		if !dbField.IsValid() || !dbField.CanSet() {
			return fiber.NewError(fiber.StatusInternalServerError, "DB field not found or cannot be set in controller")
		}

		if dbField.Type() != reflect.TypeOf((*gorm.DB)(nil)) {
			return fiber.NewError(fiber.StatusInternalServerError, "DB field has wrong type")
		}

		dbField.Set(reflect.ValueOf(db))

		return c.Next()
	}
}
