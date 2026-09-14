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

func toProtoProfile(p *domain.UserProfile) *profile.ProfileResponse {
	return &profile.ProfileResponse{
		Id:   p.ID,
		Name: p.Name,
		Role: string(p.Role),
	}
}
