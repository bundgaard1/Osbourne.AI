package server

import (
	"context"

	"osbourne.local/profile-service/gen/profile"
	"osbourne.local/profile-service/internal/domain"
	"osbourne.local/profile-service/internal/service"
)

type ProfileServer struct {
	profile.UnimplementedProfileServiceServer
	profileSvc *service.ProfileService
}

func NewProfileServer(profileSvc *service.ProfileService) *ProfileServer {
	return &ProfileServer{
		profileSvc: profileSvc,
	}
}

func (s *ProfileServer) GetUserProfile(ctx context.Context, req *profile.ProfileRequest) (*profile.ProfileResponse, error) {
	p, err := s.profileSvc.GetProfile(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}
	profileProto := toProtoProfile(p)

	return profileProto, nil
}

func (s *ProfileServer) CreateProfile(ctx context.Context, req *profile.CreateProfileRequest) (*profile.CreateProfileResponse, error) {
	p := &domain.UserProfile{
		ID:    req.GetUserId(),
		Name:  req.GetFullName(),
		Email: req.GetEmail(),
		Role:  domain.UserRole(req.GetRole()),
	}

	if err := s.profileSvc.CreateProfile(ctx, p); err != nil {
		return nil, err
	}

	return &profile.CreateProfileResponse{Success: true}, nil
}

func toProtoProfile(p *domain.UserProfile) *profile.ProfileResponse {
	return &profile.ProfileResponse{
		Id:   p.ID,
		Name: p.Name,
		Role: string(p.Role),
	}
}
