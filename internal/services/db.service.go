package services

import (
	"errors"
	"fmt"

	"ignis/internal/database"

	"gorm.io/gorm"
)

// DBService handles all database operations using GORM
type DBService struct {
	db database.Service
}

// NewDBService creates a new instance of DBService
func NewDBService(db database.Service) *DBService {
	return &DBService{
		db: db,
	}
}

// GetDB returns the GORM database instance
func (s *DBService) GetDB() *gorm.DB {
	return s.db.GetDB()
}

// AutoMigrate runs auto migration for given models
func (s *DBService) AutoMigrate(models ...interface{}) error {
	return s.db.GetDB().AutoMigrate(models...)
}

// Create creates a new record in the database
func (s *DBService) Create(model interface{}) error {
	result := s.db.GetDB().Create(model)
	if result.Error != nil {
		return fmt.Errorf("failed to create record: %w", result.Error)
	}
	return nil
}

// GetByID retrieves a record by its ID
func (s *DBService) GetByID(model interface{}, id interface{}) error {
	result := s.db.GetDB().First(model, id)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("record not found")
		}
		return fmt.Errorf("failed to get record: %w", result.Error)
	}
	return nil
}

// GetAll retrieves all records of a model
func (s *DBService) GetAll(models interface{}) error {
	result := s.db.GetDB().Find(models)
	if result.Error != nil {
		return fmt.Errorf("failed to get records: %w", result.Error)
	}
	return nil
}

// Update updates a record in the database
func (s *DBService) Update(model interface{}) error {
	result := s.db.GetDB().Save(model)
	if result.Error != nil {
		return fmt.Errorf("failed to update record: %w", result.Error)
	}
	return nil
}

// Delete deletes a record from the database
func (s *DBService) Delete(model interface{}, id interface{}) error {
	result := s.db.GetDB().Delete(model, id)
	if result.Error != nil {
		return fmt.Errorf("failed to delete record: %w", result.Error)
	}
	return nil
}

// FindWhere finds records based on conditions
func (s *DBService) FindWhere(models interface{}, query interface{}, args ...interface{}) error {
	result := s.db.GetDB().Where(query, args...).Find(models)
	if result.Error != nil {
		return fmt.Errorf("failed to find records: %w", result.Error)
	}
	return nil
}

// FindOne finds a single record based on conditions
func (s *DBService) FindOne(model interface{}, query interface{}, args ...interface{}) error {
	result := s.db.GetDB().Where(query, args...).First(model)
	if result.Error != nil {
		if errors.Is(result.Error, gorm.ErrRecordNotFound) {
			return fmt.Errorf("record not found")
		}
		return fmt.Errorf("failed to find record: %w", result.Error)
	}
	return nil
}

// Transaction executes a function within a database transaction
func (s *DBService) Transaction(fn func(*gorm.DB) error) error {
	return s.db.GetDB().Transaction(fn)
}

// Count counts records based on conditions
func (s *DBService) Count(model interface{}, query interface{}, args ...interface{}) (int64, error) {
	var count int64
	result := s.db.GetDB().Model(model).Where(query, args...).Count(&count)
	if result.Error != nil {
		return 0, fmt.Errorf("failed to count records: %w", result.Error)
	}
	return count, nil
}
