package service

import (
	"context"
	"fmt"
	"time"

	"github.com/google/uuid"
	"osbourne.local/course-content-service/internal/domain"
)

type ModuleService struct {
	repo domain.ContentRepository
	// fileStore domain.ObjectStore
}

func NewModuleService(repo domain.ContentRepository) *ModuleService {
	return &ModuleService{
		repo: repo,
		// fileStore: fileStore,
	}
}

func (s *ModuleService) CreateModule(ctx context.Context, module *domain.Module) error {
	if module.ID == "" {
		module.ID = uuid.NewString()
	}

	// if module.Attachments == nil {
	// 	module.Attachments = make([]domain.Attachment, 0)
	// }

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

// Files upload and attachment management methods are commented out for now, as they may require
//  further design decisions regarding file storage and metadata handling.

// func (s *ModuleService) UploadAttachment(
// 	ctx context.Context,
// 	moduleID string,
// 	fileName string,
// 	fileType string,
// 	size int64,
// 	fileData io.Reader,
// ) (*domain.Attachment, error) {

// 	// 1. Validate that the module exists
// 	module, err := s.repo.GetModule(ctx, moduleID)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to fetch module: %w", err)
// 	}
// 	if module == nil {
// 		return nil, fmt.Errorf("module with ID %s not found", moduleID)
// 	}

// 	// 2. Generate a unique ID for the attachment and determine the relative path for storage
// 	attachmentID := uuid.NewString()
// 	relativePath := fmt.Sprintf("modules/%s/%s_%s", moduleID, attachmentID, fileName)

// 	// 3. Save the physical file to the object store
// 	savedPath, err := s.fileStore.Save(ctx, relativePath, fileData)
// 	if err != nil {
// 		return nil, fmt.Errorf("failed to save physical file: %w", err)
// 	}

// 	// 4. Create the attachment metadata
// 	attachment := &domain.Attachment{
// 		ID:        attachmentID,
// 		Name:      fileName,
// 		Type:      fileType,
// 		Path:      savedPath, // Can be converted to a full URL in the API layer or here
// 		Size:      size,
// 		CreatedAt: time.Now(),
// 	}

// 	// 5. Add the attachment metadata to the module in the database
// 	err = s.repo.AddAttachmentToModule(ctx, moduleID, attachment)
// 	if err != nil {
// 		// ROLLBACK: If the database fails, we delete the saved file again
// 		_ = s.fileStore.Delete(ctx, savedPath)
// 		return nil, fmt.Errorf("failed to attach file metadata to module in DB: %w", err)
// 	}

// 	return attachment, nil
// }

// func (s *ModuleService) RemoveAttachment(ctx context.Context, moduleID string, attachmentID string) error {
// 	// 1. Fetch the module to get the attachment path
// 	module, err := s.repo.GetModule(ctx, moduleID)
// 	if err != nil {
// 		return fmt.Errorf("failed to fetch module: %w", err)
// 	}
// 	if module == nil {
// 		return fmt.Errorf("module with ID %s not found", moduleID)
// 	}

// 	var attachmentToRemove *domain.Attachment
// 	for _, att := range module.Attachments {
// 		if att.ID == attachmentID {
// 			attachmentToRemove = &att
// 			break
// 		}
// 	}

// 	if attachmentToRemove == nil {
// 		return fmt.Errorf("attachment with ID %s not found in module %s", attachmentID, moduleID)
// 	}

// 	// 2. Remove the attachment metadata from the database
// 	err = s.repo.RemoveAttachmentFromModule(ctx, moduleID, attachmentID)
// 	if err != nil {
// 		return fmt.Errorf("failed to remove attachment metadata from DB: %w", err)
// 	}

// 	// 3. Delete the physical file from the object store
// 	err = s.fileStore.Delete(ctx, attachmentToRemove.Path)
// 	if err != nil {
// 		return fmt.Errorf("failed to delete physical file %s: %w", attachmentToRemove.Path, err)
// 	}

// 	return nil
// }
