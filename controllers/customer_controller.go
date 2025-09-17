package controllers

import (
	"errors"
	"fiber-app/models"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type CustomerController struct {
	DB *gorm.DB
}

var customerInput struct {
	ID           uint   `json:"id"`
	CustomerCode string `json:"customer_code" validate:"required,min=3"`
	CustomerName string `json:"customer_name" validate:"required,min=3"`
	CustAddr1    string `json:"cust_addr1"`
	CustAddr2    string `json:"cust_addr2"`
	CustCity     string `json:"cust_city"`
	CustArea     string `json:"cust_area"`
	CustPhone    string `json:"cust_phone"`
	CustCountry  string `json:"cust_country"`
	CustEmail    string `json:"cust_email"`
	OwnerCode    string `json:"owner_code"`
}

func NewCustomerController(db *gorm.DB) *CustomerController {
	return &CustomerController{DB: db}
}

func (c *CustomerController) GetAllCustomers(ctx *fiber.Ctx) error {
	var customers []models.Customer
	if err := c.DB.Find(&customers).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Customers found", "data": customers})
}

func (c *CustomerController) GetCustomerByID(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var result models.Customer
	if err := c.DB.First(&result, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Customer not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Customer found", "data": result})
}

func (c *CustomerController) CreateCustomer(ctx *fiber.Ctx) error {
	if err := ctx.BodyParser(&customerInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	customer := models.Customer{
		CustomerCode: customerInput.CustomerCode,
		CustomerName: customerInput.CustomerName,
		CustAddr1:    customerInput.CustAddr1,
		CustAddr2:    customerInput.CustAddr2,
		CustCity:     customerInput.CustCity,
		CustArea:     customerInput.CustArea,
		CustPhone:    customerInput.CustPhone,
		CustCountry:  customerInput.CustCountry,
		CustEmail:    customerInput.CustEmail,
		CreatedBy:    int(ctx.Locals("userID").(float64)),
	}

	if err := c.DB.Create(&customer).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Customer created successfully", "data": customer})
}

func (c *CustomerController) UpdateCustomer(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	if err := ctx.BodyParser(&customerInput); err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
	}

	if err := c.DB.Debug().
		Model(&models.Customer{}).
		Where("id = ?", id).
		Updates(map[string]interface{}{
			// "customer_code": customerInput.CustomerCode,
			"customer_name": customerInput.CustomerName,
			"cust_addr1":    customerInput.CustAddr1,
			"cust_city":     customerInput.CustCity,
			"cust_area":     customerInput.CustArea,
			"cust_phone":    customerInput.CustPhone,
			"cust_country":  customerInput.CustCountry,
			"cust_email":    customerInput.CustEmail,
			"updated_by":    int(ctx.Locals("userID").(float64)),
		}).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// customer := models.Customer{
	// 	CustomerCode: customerInput.CustomerCode,
	// 	CustomerName: customerInput.CustomerName,
	// 	CustAddr1:    customerInput.CustAddr1,
	// 	CustAddr2:    customerInput.CustAddr2,
	// 	CustCity:     customerInput.CustCity,
	// 	CustArea:     customerInput.CustArea,
	// 	CustPhone:    customerInput.CustPhone,
	// 	CustEmail:    customerInput.CustEmail,
	// 	CustCountry:  customerInput.CustCountry,
	// 	UpdatedBy:    int(ctx.Locals("userID").(float64)),
	// }

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	// if err := c.DB.Select("customer_code", "customer_name", "updated_by").Where("id = ?", id).Updates(&customer).Error; err != nil {
	// 	return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	// }

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Customer updated successfully", "data": customerInput})
}

func (c *CustomerController) DeleteCustomer(ctx *fiber.Ctx) error {
	id, err := ctx.ParamsInt("id")
	if err != nil {
		return ctx.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "Invalid ID"})
	}

	var customer models.Customer
	if err := c.DB.First(&customer, id).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ctx.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "Customer not found"})
		}
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Add CustomerID to DeletedBy field
	customer.DeletedBy = int(ctx.Locals("userID").(float64))

	// Hanya menyimpan field yang dipilih dengan menggunakan Select
	if err := c.DB.Select("deleted_by").Where("id = ?", id).Updates(&customer).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	// Hapus customer
	if err := c.DB.Delete(&customer).Error; err != nil {
		return ctx.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": err.Error()})
	}

	return ctx.Status(fiber.StatusOK).JSON(fiber.Map{"success": true, "message": "Customer deleted successfully", "data": customer})
}
