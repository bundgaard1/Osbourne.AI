package server

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

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
	return toProtoProfile(p), nil
}

func toProtoProfile(p *domain.UserProfile) *profile.ProfileResponse {
	resp := &profile.ProfileResponse{
		Id:           p.ID,
		Name:         p.Name,
		Phone:        p.Phone,
		Bio:          p.Bio,
		StudyProgram: p.StudyProgram,
	}
	if p.Birthday != nil {
		resp.Birthday = timestamppb.New(*p.Birthday)
	}
	return resp
}