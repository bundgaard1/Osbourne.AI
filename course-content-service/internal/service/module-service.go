package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"osbourne.local/course-content-service/internal/domain"
)

type ModuleService struct {
	repo domain.ModuleRepository
}

func NewModuleService(repo domain.ModuleRepository) *ModuleService {
	return &ModuleService{
		repo: repo,
	}
}

func (s *ModuleService) CreateModule(ctx context.Context, module *domain.Module) error {
	if module.ID == "" {
		module.ID = uuid.NewString()
	}

	module.UpdatedAt = time.Now()

	return s.repo.CreateModule(ctx, module)
}

func (s *ModuleService) GetModule(ctx context.Context, moduleID string) (*domain.Module, error) {
	return s.repo.GetModule(ctx, moduleID)
}

type UpdateModuleInput struct {
	ID    string
	Title string
	Text  string
}

func (s *ModuleService) UpdateModule(ctx context.Context, update *UpdateModuleInput) error {

	existingModule, err := s.repo.GetModule(ctx, update.ID)
	if err != nil {
		return fmt.Errorf("failed to fetch module for validation: %w", err)
	}
	if existingModule == nil {
		return fmt.Errorf("module with ID %s not found", update.ID)
	}

	// Update the fields of the existing module with the new values
	existingModule.Title = update.Title
	existingModule.Text = update.Text

	existingModule.UpdatedAt = time.Now()

	return s.repo.UpdateModule(ctx, existingModule)
}

func (s *ModuleService) DeleteModule(ctx context.Context, moduleID string) error {
	module, err := s.repo.GetModule(ctx, moduleID)
	if err != nil {
		return fmt.Errorf("failed to fetch module before deletion: %w", err)
	}
	if module == nil {
		return fmt.Errorf("module with ID %s not found", moduleID)
	}

	return nil
}
func (s *ModuleService) ListModulesByCourseID(ctx context.Context, courseID string) ([]*domain.Module, error) {
	return s.repo.ListModules(ctx, courseID)
}
