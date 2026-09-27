package server

import (
	"context"

	"google.golang.org/protobuf/types/known/timestamppb"

	"osbourne.local/common"
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
	// REST callers cannot supply a user_id (GET /api/profile has nowhere to put
	// one), so the JWT decides. Trusted internal gRPC callers still pass it
	// explicitly, which is what keeps the frontend's SSR handlers working.
	userID, err := common.UserIDFromContextOrRequest(ctx, req.GetUserId())
	if err != nil {
		return nil, err
	}

	p, err := s.profileSvc.GetProfile(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toProtoProfile(p), nil
}

// UpdateUserProfile edits the caller's own profile.
//
// Unlike GetUserProfile this deliberately ignores any user_id and takes the
// subject from the verified token only. The field is accepted for gRPC
// symmetry, so honouring it would make PUT /api/profile an IDOR: any
// authenticated caller could rewrite somebody else's profile by putting their
// id in the body.
func (s *ProfileServer) UpdateUserProfile(ctx context.Context, req *profile.UpdateUserProfileRequest) (*profile.UpdateUserProfileResponse, error) {
	claims, ok := common.ClaimsFromContext(ctx)
	if !ok || claims.UserID == "" {
		return nil, common.RequiresAuthentication()
	}

	in := service.UpdateProfileInput{
		Name:         req.GetName(),
		Phone:        req.GetPhone(),
		Bio:          req.GetBio(),
		StudyProgram: req.GetStudyProgram(),
	}
	if ts := req.GetBirthday(); ts != nil {
		t := ts.AsTime()
		in.Birthday = &t
	}

	updated, err := s.profileSvc.UpdateProfile(ctx, claims.UserID, in)
	if err != nil {
		return nil, err
	}

	return &profile.UpdateUserProfileResponse{Profile: toProtoProfile(updated)}, nil
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
