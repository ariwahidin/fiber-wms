package rpt_builder_service

import (
	"fiber-app/models/rpt_builder"
	"sort"

	"gorm.io/gorm"
)

// GetOrInitUserColumnPrefs mengambil preferensi kolom user untuk satu sheet.
// Kalau belum ada → auto-insert dari default columns, lalu return hasilnya.
func GetOrInitUserColumnPrefs(
	db *gorm.DB,
	userID uint,
	templateID uint,
	sheetID uint,
	defaultColumns []rpt_builder.Rpt2Column,
) ([]rpt_builder.Rpt2UserColumnPref, error) {

	var prefs []rpt_builder.Rpt2UserColumnPref
	err := db.Where("user_id = ? AND sheet_id = ?", userID, sheetID).
		Order("column_order ASC").
		Find(&prefs).Error
	if err != nil {
		return nil, err
	}

	// Sudah ada → langsung return
	if len(prefs) > 0 {
		return prefs, nil
	}

	// Belum ada → insert dari default
	for _, col := range defaultColumns {
		pref := rpt_builder.Rpt2UserColumnPref{
			TemplateID:  templateID,
			SheetID:     sheetID,
			UserID:      userID,
			ColumnKey:   col.ColumnKey,
			IsVisible:   col.IsDefaultVisible,
			ColumnOrder: col.ColumnOrder,
		}
		prefs = append(prefs, pref)
	}

	if err := db.Create(&prefs).Error; err != nil {
		return nil, err
	}

	return prefs, nil
}

// PrefsToColumnConfigs mengkonversi []Rpt2UserColumnPref + map default column config
// menjadi []ColumnConfig yang siap dipakai excel generator.
func PrefsToColumnConfigs(
	prefs []rpt_builder.Rpt2UserColumnPref,
	defaultCols []rpt_builder.Rpt2Column,
) []ColumnConfig {

	// Build lookup default cols by key
	defaultMap := make(map[string]rpt_builder.Rpt2Column)
	for _, c := range defaultCols {
		defaultMap[c.ColumnKey] = c
	}

	var configs []ColumnConfig
	for _, pref := range prefs {
		def, ok := defaultMap[pref.ColumnKey]
		if !ok {
			continue
		}
		configs = append(configs, ColumnConfig{
			ColumnKey:   pref.ColumnKey,
			DisplayName: def.DisplayName,
			ExcelWidth:  def.ExcelWidth,
			ExcelFormat: def.ExcelFormat,
			IsVisible:   pref.IsVisible,
			ColumnOrder: pref.ColumnOrder,
		})
	}

	sort.Slice(configs, func(i, j int) bool {
		return configs[i].ColumnOrder < configs[j].ColumnOrder
	})

	return configs
}
