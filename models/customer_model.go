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
	OwnerCode    string `json:"owner_code"`
	CreatedBy    int
	UpdatedBy    int
	DeletedBy    int
}
