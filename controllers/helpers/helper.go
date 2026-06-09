package helpers

import "strings"

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

// helper function, taruh di file utils/helpers
func chunkInts(slice []int, size int) [][]int {
	var chunks [][]int
	for size < len(slice) {
		slice, chunks = slice[size:], append(chunks, slice[:size])
	}
	return append(chunks, slice)
}
