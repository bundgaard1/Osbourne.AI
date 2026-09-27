package server

import (
	"context"

	coursecontent "osbourne.local/course-content-service/gen/course-content"
	"osbourne.local/course-content-service/internal/domain"
	"osbourne.local/course-content-service/internal/service"
)

type ContentServer struct {
	// Embed Unimplemented, not the CourseContentServiceServer interface. The
	// interface embeds a nil value, so any method left unimplemented would
	// resolve through a nil pointer and panic the server instead of returning
	// codes.Unimplemented. Every other service in this workspace already does it
	// this way.
	coursecontent.UnimplementedCourseContentServiceServer
	sourceSvc *service.ModuleService
}

func NewContentServer(sourceSvc *service.ModuleService) *ContentServer {
	return &ContentServer{
		sourceSvc: sourceSvc,
	}
}

// CreateModule implements the CreateModule RPC. It was previously named Create,
// which matched no interface method, so the RPC resolved through the nil
// embedded interface above and panicked when called.
func (s *ContentServer) CreateModule(ctx context.Context, req *coursecontent.CreateModuleRequest) (*coursecontent.CreateModuleResponse, error) {
	module := &domain.Module{
		ID:       "",
		CourseID: req.GetCourseId(),
		Title:    req.GetTitle(),
		Text:     req.GetText(),
	}

	err := s.sourceSvc.CreateModule(ctx, module)
	if err != nil {
		return nil, err
	}

	m, err := s.sourceSvc.GetModule(ctx, module.ID)
	if err != nil {
		return nil, err
	}

	moduleProto := toProtoModule(m)

	return &coursecontent.CreateModuleResponse{
		Module: moduleProto,
	}, nil
}

func (s *ContentServer) GetModule(ctx context.Context, req *coursecontent.GetModuleRequest) (*coursecontent.GetModuleResponse, error) {
	module, err := s.sourceSvc.GetModule(ctx, req.GetModuleId())
	if err != nil {
		return nil, err
	}

	moduleProto := toProtoModule(module)

	return &coursecontent.GetModuleResponse{
		Module: moduleProto,
	}, nil
}

func (s *ContentServer) UpdateModule(ctx context.Context, req *coursecontent.UpdateModuleRequest) (*coursecontent.UpdateModuleResponse, error) {
	update := &service.UpdateModuleInput{
		ID:    req.GetModuleId(),
		Title: req.GetTitle(),
		Text:  req.GetText(),
	}

	err := s.sourceSvc.UpdateModule(ctx, update)
	if err != nil {
		return nil, err
	}

	m, err := s.sourceSvc.GetModule(ctx, update.ID)
	if err != nil {
		return nil, err
	}

	moduleProto := toProtoModule(m)

	return &coursecontent.UpdateModuleResponse{
		Module: moduleProto,
	}, nil
}

func (s *ContentServer) DeleteModule(ctx context.Context, req *coursecontent.DeleteModuleRequest) (*coursecontent.DeleteModuleResponse, error) {
	err := s.sourceSvc.DeleteModule(ctx, req.GetModuleId())
	if err != nil {
		return nil, err
	}

	return &coursecontent.DeleteModuleResponse{}, nil
}

func (s *ContentServer) ListModulesByCourseID(ctx context.Context, req *coursecontent.ListModulesByCourseIDRequest) (*coursecontent.ListModulesByCourseIDResponse, error) {
	modules, err := s.sourceSvc.ListModulesByCourseID(ctx, req.GetCourseId())
	if err != nil {
		return nil, err
	}

	var moduleProtos []*coursecontent.Module
	for _, module := range modules {
		moduleProtos = append(moduleProtos, toProtoModule(module))
	}

	return &coursecontent.ListModulesByCourseIDResponse{
		Modules: moduleProtos,
	}, nil
}

// toProtoModule is nil-safe on purpose. The service now returns a NotFound
// status for a missing module, so this should never see nil - but a nil
// dereference here panics the process and takes every in-flight request with
// it, and a gateway route is directly reachable by anyone who can authenticate.
// The catalogue's equivalent already has this guard.
func toProtoModule(module *domain.Module) *coursecontent.Module {
	if module == nil {
		return nil
	}

	return &coursecontent.Module{
		Id:       module.ID,
		CourseId: module.CourseID,
		Title:    module.Title,
		Text:     module.Text,
	}
}
