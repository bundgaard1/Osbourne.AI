package repository

import (
	"context"
	"errors"
	"fmt"

	c "github.com/ostafen/clover/v2"
	"github.com/ostafen/clover/v2/document"
	"github.com/ostafen/clover/v2/query"
	"osbourne.local/course-content-service/internal/domain"
)

type CloverModuleRepository struct {
	collectionName string
	db             *c.DB
}

func NewCloverModuleRepository(db *c.DB, cn string) (*CloverModuleRepository, error) {
	// CreateCollection doubles as the existence check; ErrCollectionExist
	// means the collection already exists, which is fine.
	if err := db.CreateCollection(cn); err != nil && !errors.Is(err, c.ErrCollectionExist) {
		return nil, fmt.Errorf("failed to create collection: %w", err)
	}
	return &CloverModuleRepository{db: db, collectionName: cn}, nil
}

func (r *CloverModuleRepository) ListModules(ctx context.Context, courseID string) ([]*domain.Module, error) {
	results, err := r.db.FindAll(query.NewQuery(r.collectionName).Where(query.Field("courseId").Eq(courseID)))
	if err != nil {
		return nil, err
	}

	modules := make([]*domain.Module, len(results))
	for i, result := range results {
		modules[i], _ = toDomainModule(result)
	}
	return modules, nil
}

func (r *CloverModuleRepository) CreateModule(ctx context.Context, module *domain.Module) error {
	doc := document.NewDocumentOf(module)
	// fmt.Printf("Creating module with ID: %s \n", module.ID)
	if doc == nil {
		return fmt.Errorf("Could not create document from module")
	}

	// This generate a "_id", but we need not worry about it, its for clover internal use, we will use our own "id" field for the module.
	_, err := r.db.InsertOne(r.collectionName, doc)
	return err
}

func (r *CloverModuleRepository) GetModule(ctx context.Context, moduleID string) (*domain.Module, error) {
	assertion := query.Field("id").Eq(moduleID)
	doc, err := r.db.FindFirst(query.NewQuery(r.collectionName).Where(assertion))
	if err != nil {
		return nil, err
	}
	if doc == nil {
		return nil, nil
	}
	// fmt.Println(doc.Fields(true))

	return toDomainModule(doc)
}

func (r *CloverModuleRepository) UpdateModule(ctx context.Context, module *domain.Module) error {
	doc := document.NewDocumentOf(module)
	if doc == nil {
		return fmt.Errorf("failed to convert module to document")
	}

	assertion := query.Field("id").Eq(module.ID)
	err := r.db.Update(
		query.NewQuery(r.collectionName).Where(assertion),
		doc.AsMap(),
	)
	if err != nil {
		return fmt.Errorf("failed to update module: %w", err)
	}

	return nil
}

func (r *CloverModuleRepository) DeleteModule(ctx context.Context, moduleID string) error {
	assertion := query.Field("id").Eq(moduleID)

	err := r.db.Delete(query.NewQuery(r.collectionName).Where(assertion))
	if err != nil {
		return fmt.Errorf("failed to delete module: %w", err)
	}

	return err
}

func (r *CloverModuleRepository) GetAllModules(ctx context.Context) ([]*domain.Module, int32, error) {
	result, err := r.db.FindAll(query.NewQuery(r.collectionName))
	if err != nil {
		return nil, 0, err
	}

	modules := make([]*domain.Module, len(result))
	for i, doc := range result {
		modules[i], _ = toDomainModule(doc)
	}
	return modules, int32(len(modules)), nil
}

func toDomainModule(doc *document.Document) (*domain.Module, error) {
	// fmt.Printf("Unmarshalling document: %v\n", doc)
	var module domain.Module
	if err := doc.Unmarshal(&module); err != nil {
		return nil, fmt.Errorf("failed to unmarshal module: %w", err)
	}
	// fmt.Printf("Marshaled module: %+v\n\n", module)
	return &module, nil
}
