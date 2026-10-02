package models

import "gorm.io/gorm"

type Customer struct {
	gorm.Model
	CustomerCode string `json:"customer_code" gorm:"unique"`
	CustomerName string `json:"customer_name" required:"required"`
	CustAddr1    string `json:"cust_addr1" required:"required"`
	CustAddr2    string `json:"cust_addr2"`
	CustCity     string `json:"cust_city" required:"required"`
	CustArea     string `json:"cust_area"`
	CustCountry  string `json:"cust_country" required:"required"`
	CustPhone    string `json:"cust_phone" required:"required"`
	CustEmail    string `json:"cust_email"`
	OwnerCode    string `json:"owner_code"`
	IsActive     bool   `json:"is_active" gorm:"default:true"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}
