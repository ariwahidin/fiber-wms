package repositories

import (
	"errors"
	"fiber-app/models"
	"fmt"

	"gorm.io/gorm"
)

type UomRepository struct {
	DB *gorm.DB
}

func NewUomRepository(DB *gorm.DB) *UomRepository {
	return &UomRepository{DB: DB}
}

type UomConversionResult struct {
	ItemCode     string `json:"item_code"`
	FromUom      string `json:"from_uom"`
	FromQty      int    `json:"from_qty"`
	ToUom        string `json:"to_uom"`
	QtyConverted int    `json:"qty_converted"`
}

func (r *UomRepository) ConversionQty(item_code string, from_qty int, from_uom string) (UomConversionResult, error) {

	var product models.Product
	err := r.DB.Table("products").Where("item_code = ?", item_code).First(&product).Error
	if err != nil {
		return UomConversionResult{}, err
	}

	var UomConversion models.UomConversion
	err = r.DB.Table("uom_conversions").
		Where("item_code = ? AND from_uom = ? AND to_uom = ?", item_code, from_uom, product.Uom).
		First(&UomConversion).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			fmt.Println("ERROR TIDAK KETEMU")
			return UomConversionResult{}, errors.New("Failed to convert UOM for item: " + item_code +
				". Conversion from " + from_uom + " to " + product.Uom + " not found")
		}
		return UomConversionResult{}, err
	}

	conversionQty := from_qty * UomConversion.ConversionRate
	return UomConversionResult{
		ItemCode:     item_code,
		FromUom:      from_uom,
		ToUom:        product.Uom,
		FromQty:      from_qty,
		QtyConverted: conversionQty,
	}, nil
}
