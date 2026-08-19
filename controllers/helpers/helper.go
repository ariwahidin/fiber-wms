package helpers

import (
	"strings"

	"gorm.io/gorm"
)

// Helper function to validate email format
func IsValidEmail(email string) bool {
	if email == "" {
		return true // Empty email is valid (optional field)
	}
	// Simple email validation
	parts := strings.Split(email, "@")
	if len(parts) != 2 {
		return false
	}
	if len(parts[0]) == 0 || len(parts[1]) == 0 {
		return false
	}
	if !strings.Contains(parts[1], ".") {
		return false
	}
	return true
}

const (
	ChunkSizeIN     = 1000 // untuk WHERE ... IN (?)
	ChunkSizeInsert = 50   // untuk Create batch
)

// ChunkSlice memecah slice apapun jadi beberapa slice kecil, generic untuk semua tipe.
func ChunkSlice[T any](items []T, size int) [][]T {
	if size <= 0 {
		size = 500
	}
	var chunks [][]T
	for i := 0; i < len(items); i += size {
		end := i + size
		if end > len(items) {
			end = len(items)
		}
		chunks = append(chunks, items[i:end])
	}
	return chunks
}

// FindInChunks jalanin query "WHERE col IN (?)" dengan otomatis chunking,
// dan gabungin semua hasilnya jadi satu slice.
// Contoh pakai: err := helpers.FindInChunks(db, "item_code IN ?", itemCodes, &products)
func FindInChunks[K any, T any](db *gorm.DB, whereClause string, keys []K, dest *[]T) error {
	if len(keys) == 0 {
		return nil
	}
	for _, chunk := range ChunkSlice(keys, ChunkSizeIN) {
		var part []T
		if err := db.Where(whereClause, chunk).Find(&part).Error; err != nil {
			return err
		}
		*dest = append(*dest, part...)
	}
	return nil
}

// CreateInChunks insert slice apapun secara batch dengan ukuran aman dari limit parameter mssql.
func CreateInChunks[T any](db *gorm.DB, items []T) error {
	if len(items) == 0 {
		return nil
	}
	return db.CreateInBatches(&items, ChunkSizeInsert).Error
}
