package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/google/uuid"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
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

// notFoundStatus translates the repository's sentinel into a gRPC NotFound.
//
// The status code is what the gateway turns into HTTP 404. Wrapping it in a
// plain error instead - as this file used to do with
// fmt.Errorf("module with ID %s not found") - reaches the browser as a 500,
// which is both wrong and indistinguishable from a real outage.
func notFoundStatus(moduleID string, err error) error {
	if errors.Is(err, domain.ErrNotFound) {
		return status.Errorf(codes.NotFound, "module %s was not found", moduleID)
	}
	return err
}

func (s *ModuleService) GetModule(ctx context.Context, moduleID string) (*domain.Module, error) {
	module, err := s.repo.GetModule(ctx, moduleID)
	if err != nil {
		return nil, notFoundStatus(moduleID, err)
	}
	return module, nil
}

type UpdateModuleInput struct {
	ID    string
	Title string
	Text  string
}

func (s *ModuleService) UpdateModule(ctx context.Context, update *UpdateModuleInput) error {

	existingModule, err := s.repo.GetModule(ctx, update.ID)
	if err != nil {
		return notFoundStatus(update.ID, fmt.Errorf("failed to fetch module for validation: %w", err))
	}

	// Update the fields of the existing module with the new values
	existingModule.Title = update.Title
	existingModule.Text = update.Text

	existingModule.UpdatedAt = time.Now()

	return s.repo.UpdateModule(ctx, existingModule)
}

// DeleteModule removes a module.
//
// This previously fetched the module, checked it existed, and then returned nil
// without deleting anything - so DELETE answered 200 while leaving the module
// readable. The fetch is still done, so a delete against an unknown id reports
// 404 rather than silently succeeding.
func (s *ModuleService) DeleteModule(ctx context.Context, moduleID string) error {
	module, err := s.repo.GetModule(ctx, moduleID)
	if err != nil {
		return notFoundStatus(moduleID, fmt.Errorf("failed to fetch module before deletion: %w", err))
	}
	_ = module

	return s.repo.DeleteModule(ctx, moduleID)
}
func (s *ModuleService) ListModulesByCourseID(ctx context.Context, courseID string) ([]*domain.Module, error) {
	return s.repo.ListModules(ctx, courseID)
}
