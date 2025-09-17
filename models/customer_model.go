package models

import "gorm.io/gorm"

type Customer struct {
	gorm.Model
	CustomerCode string `json:"customer_code" gorm:"unique"`
	CustomerName string `json:"customer_name"`
	CustAddr1    string `json:"cust_addr1"`
	CustAddr2    string `json:"cust_addr2"`
	CustCity     string `json:"cust_city"`
	CustArea     string `json:"cust_area"`
	CustCountry  string `json:"cust_country"`
	CustPhone    string `json:"cust_phone"`
	CustEmail    string `json:"cust_email"`
	OwnerCode    string `json:"owner_code"`
	IsActive     bool   `json:"is_active" gorm:"default:true"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}
