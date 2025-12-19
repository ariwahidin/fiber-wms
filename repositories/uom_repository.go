package repositories

import (
	"errors"
	"fiber-app/models"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type UomRepository struct {
	DB *gorm.DB
}

func NewUomRepository(DB *gorm.DB) *UomRepository {
	return &UomRepository{DB: DB}
}

type UomConversionResult struct {
	Ean          string  `json:"ean"`
	ItemCode     string  `json:"item_code"`
	FromUom      string  `json:"from_uom"`
	FromQty      float64 `json:"from_qty"`
	ToUom        string  `json:"to_uom"`
	Rate         float64 `json:"rate"`
	QtyConverted float64 `json:"qty_converted"`
}

type ResultUomByEan struct {
	ItemCode string  `json:"item_code"`
	BaseEan  string  `json:"base_ean"`
	BaseUom  string  `json:"base_uom"`
	Ean      string  `json:"ean"`
	Uom      string  `json:"uom"`
	Rate     float64 `json:"rate"`
}

// func (r *UomRepository) ConversionQty(item_code string, from_qty int, from_uom string) (UomConversionResult, error) {

// 	var product models.Product
// 	err := r.DB.Table("products").Where("item_code = ?", item_code).First(&product).Error
// 	if err != nil {
// 		return UomConversionResult{}, err
// 	}

// 	var UomConversion models.UomConversion
// 	errUom := r.DB.Table("uom_conversions").
// 		Where("item_code = ? AND from_uom = ? AND to_uom = ?", item_code, from_uom, product.Uom).
// 		First(&UomConversion).Error

// 	if errUom != nil {
// 		if errors.Is(errUom, gorm.ErrRecordNotFound) {
// 			return UomConversionResult{}, errors.New("Failed to convert UOM for item: " + item_code +
// 				". Conversion from " + from_uom + " to " + product.Uom + " not found")
// 		}
// 		return UomConversionResult{}, errUom
// 	}

// 	if !UomConversion.IsLocked {
// 		errLock := r.LockUOMIfUsed(item_code, from_uom)
// 		if errLock != nil {
// 			return UomConversionResult{}, errLock
// 		}
// 	}

// 	conversionQty := float64(from_qty) * UomConversion.ConversionRate
// 	return UomConversionResult{
// 		Ean:          UomConversion.Ean,
// 		Rate:         UomConversion.ConversionRate,
// 		ItemCode:     item_code,
// 		FromUom:      from_uom,
// 		ToUom:        product.Uom,
// 		FromQty:      from_qty,
// 		QtyConverted: conversionQty,
// 	}, nil
// }

func (r *UomRepository) ConversionQty(itemCode string, fromQty float64, fromUom string) (UomConversionResult, error) {
	var product models.Product
	if err := r.DB.Table("products").Where("item_code = ?", itemCode).First(&product).Error; err != nil {
		return UomConversionResult{}, err
	}

	currentUom := fromUom
	totalRate := 1.0
	ean := ""
	visited := make(map[string]bool)

	for {
		if visited[currentUom] {
			return UomConversionResult{}, fmt.Errorf("detected circular conversion for %s (%s)", itemCode, currentUom)
		}
		visited[currentUom] = true

		var conv models.UomConversion
		err := r.DB.Table("uom_conversions").
			Where("item_code = ? AND from_uom = ?", itemCode, currentUom).
			First(&conv).Error
		if err != nil {
			return UomConversionResult{}, fmt.Errorf("no conversion found from %s for %s", currentUom, itemCode)
		}

		totalRate *= conv.ConversionRate
		ean = conv.Ean

		if conv.ToUom == product.Uom {
			break
		}

		currentUom = conv.ToUom
	}

	conversionQty := float64(fromQty) * totalRate
	return UomConversionResult{
		Ean:          ean,
		Rate:         totalRate,
		ItemCode:     itemCode,
		FromUom:      fromUom,
		ToUom:        product.Uom,
		FromQty:      fromQty,
		QtyConverted: conversionQty,
	}, nil
}

func (r *UomRepository) GetUomConversionByEan(Ean string) (ResultUomByEan, error) {

	var uomConversion models.UomConversion
	if err := r.DB.Debug().Where("ean = ?", Ean).First(&uomConversion).Error; err != nil {
		return ResultUomByEan{}, err
	}

	var product models.Product
	if errP := r.DB.Where("item_code = ?", uomConversion.ItemCode).First(&product).Error; errP != nil {
		return ResultUomByEan{}, errP
	}

	baseUom := product.Uom
	baseEan := product.Barcode
	currentEan := uomConversion.Ean
	currentUom := uomConversion.FromUom
	currentRate := 1.0
	visited := make(map[string]bool)

	fmt.Println("Uom Conversion : ", uomConversion)
	fmt.Println("From Uom : ", uomConversion.FromUom)
	fmt.Println("Current Uom :", currentUom)

	for {
		if visited[currentUom] {
			return ResultUomByEan{}, fmt.Errorf("detected circular conversion for %s (%s)", currentEan, currentUom)
		}
		visited[currentUom] = true

		fmt.Println("Current UOM IN LOOP : ", currentUom)
		var conv models.UomConversion
		err := r.DB.Where("item_code = ? AND from_uom = ?", uomConversion.ItemCode, currentUom).
			First(&conv).Error
		if err != nil {
			return ResultUomByEan{}, fmt.Errorf("no conversion found from %s for %s", currentUom, currentEan)
		}

		currentRate *= conv.ConversionRate
		currentEan = conv.Ean

		if conv.ToUom == baseUom {
			break
		}

		currentUom = conv.ToUom
	}
	fmt.Println("Rate : ", currentRate)
	return ResultUomByEan{
		ItemCode: uomConversion.ItemCode,
		BaseEan:  baseEan,
		BaseUom:  baseUom,
		Ean:      Ean,
		Uom:      uomConversion.FromUom,
		Rate:     currentRate,
	}, nil
}

func (r *UomRepository) LockUOMIfUsed(item_code, uom string) error {
	return r.DB.Table("uom_conversions").
		Where("item_code = ? AND from_uom = ? AND (is_locked = 0 OR is_locked IS NULL)", item_code, uom).
		Updates(map[string]interface{}{
			"is_locked":  true,
			"updated_at": time.Now(),
		}).Error
}

func (r *UomRepository) CheckUomConversionExists(item_code string, from_uom string) (bool, error) {
	var uomConversion models.UomConversion
	if err := r.DB.Where("item_code = ? AND from_uom = ?", item_code, from_uom).First(&uomConversion).Error; err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return false, errors.New("UOM conversion not found for item: " + item_code +
				" from UoM: " + from_uom)
		}
		return false, err
	}
	return true, nil
}
