package services

import (
	"fiber-app/models"
	"fiber-app/repositories"
)

type UserService struct {
	repo *repositories.UserRepository
}

func NewUserService(repo *repositories.UserRepository) *UserService {
	return &UserService{repo: repo}
}

// Create user
func (s *UserService) CreateUser(user *models.User) error {
	return s.repo.Create(user)
}

// Get user by ID
func (s *UserService) GetUserByID(id uint) (*models.User, error) {
	return s.repo.GetByID(id)
}

// Get all users
func (s *UserService) GetAllUsers() ([]models.User, error) {
	return s.repo.GetAll()
}

// Update user
func (s *UserService) UpdateUser(user *models.User) error {
	return s.repo.Update(user)
}

// Delete user
func (s *UserService) DeleteUser(id uint) error {
	return s.repo.Delete(id)
}

func (s *UserService) GetUserByEmail(email string) (*models.User, error) {
	var user models.User
	err := s.repo.DB.Where("email = ?", email).First(&user).Error
	return &user, err
}
