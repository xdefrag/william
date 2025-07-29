package grpc

import (
	"context"
	"fmt"
	"log/slog"

	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/xdefrag/william/internal/repo"
	"github.com/xdefrag/william/pkg/adminpb"
	"github.com/xdefrag/william/pkg/models"
)

// AdminService implements the AdminServiceServer interface
type AdminService struct {
	adminpb.UnimplementedAdminServiceServer
	repo   *repo.Repository
	logger *slog.Logger
}

// NewAdminService creates a new AdminService instance
func NewAdminService(repository *repo.Repository, logger *slog.Logger) *AdminService {
	return &AdminService{
		repo:   repository,
		logger: logger,
	}
}

// GetChatSummary retrieves the latest summary for a specific chat
func (s *AdminService) GetChatSummary(ctx context.Context, req *adminpb.GetChatSummaryRequest) (*adminpb.GetChatSummaryResponse, error) {
	if req.ChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id is required")
	}

	summary, err := s.repo.GetLatestChatSummary(ctx, req.ChatId)
	if err != nil {
		s.logger.Error("Failed to get chat summary",
			slog.Int64("chat_id", req.ChatId),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "Failed to retrieve chat summary")
	}

	if summary == nil {
		return &adminpb.GetChatSummaryResponse{Summary: nil}, nil
	}

	protoSummary := s.chatSummaryToProto(summary)
	return &adminpb.GetChatSummaryResponse{Summary: protoSummary}, nil
}

// ListChatSummaries retrieves summaries for multiple chats
func (s *AdminService) ListChatSummaries(ctx context.Context, req *adminpb.ListChatSummariesRequest) (*adminpb.ListChatSummariesResponse, error) {
	if len(req.ChatIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one chat_id is required")
	}

	// Get summaries for all requested chat IDs
	var summaries []*adminpb.ChatSummary
	for _, chatID := range req.ChatIds {
		summary, err := s.repo.GetLatestChatSummary(ctx, chatID)
		if err != nil {
			s.logger.Error("Failed to get chat summary",
				slog.Int64("chat_id", chatID),
				slog.String("error", err.Error()),
			)
			continue // Skip failed summaries, don't fail the entire request
		}

		if summary != nil {
			summaries = append(summaries, s.chatSummaryToProto(summary))
		}
	}

	return &adminpb.ListChatSummariesResponse{Summaries: summaries}, nil
}

// GetUserSummary retrieves the latest summary for a specific user in a chat
func (s *AdminService) GetUserSummary(ctx context.Context, req *adminpb.GetUserSummaryRequest) (*adminpb.GetUserSummaryResponse, error) {
	if req.ChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id is required")
	}
	if req.UserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "user_id is required")
	}

	summary, err := s.repo.GetLatestUserSummary(ctx, req.ChatId, req.UserId)
	if err != nil {
		s.logger.Error("Failed to get user summary",
			slog.Int64("chat_id", req.ChatId),
			slog.Int64("user_id", req.UserId),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "Failed to retrieve user summary")
	}

	if summary == nil {
		return &adminpb.GetUserSummaryResponse{Summary: nil}, nil
	}

	protoSummary := s.userSummaryToProto(summary)
	return &adminpb.GetUserSummaryResponse{Summary: protoSummary}, nil
}

// ListUserSummaries retrieves summaries for multiple users in a chat
func (s *AdminService) ListUserSummaries(ctx context.Context, req *adminpb.ListUserSummariesRequest) (*adminpb.ListUserSummariesResponse, error) {
	if req.ChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id is required")
	}
	if len(req.UserIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one user_id is required")
	}

	var summaries []*adminpb.UserSummary
	for _, userID := range req.UserIds {
		summary, err := s.repo.GetLatestUserSummary(ctx, req.ChatId, userID)
		if err != nil {
			s.logger.Error("Failed to get user summary",
				slog.Int64("chat_id", req.ChatId),
				slog.Int64("user_id", userID),
				slog.String("error", err.Error()),
			)
			continue // Skip failed summaries
		}

		if summary != nil {
			summaries = append(summaries, s.userSummaryToProto(summary))
		}
	}

	return &adminpb.ListUserSummariesResponse{Summaries: summaries}, nil
}

// Helper methods to convert models to protobuf messages

func (s *AdminService) chatSummaryToProto(summary *models.ChatSummary) *adminpb.ChatSummary {
	topics := make(map[string]string)
	for k, v := range summary.TopicsJSON {
		if str, ok := v.(string); ok {
			topics[k] = str
		} else {
			topics[k] = fmt.Sprintf("%v", v)
		}
	}

	proto := &adminpb.ChatSummary{
		Id:        summary.ID,
		ChatId:    summary.ChatID,
		Summary:   summary.Summary,
		Topics:    topics,
		CreatedAt: timestamppb.New(summary.CreatedAt),
		UpdatedAt: timestamppb.New(summary.UpdatedAt),
	}

	if summary.NextEvents != nil {
		proto.NextEvents = summary.NextEvents
	}

	return proto
}

func (s *AdminService) userSummaryToProto(summary *models.UserSummary) *adminpb.UserSummary {
	likes := make(map[string]string)
	for k, v := range summary.LikesJSON {
		if str, ok := v.(string); ok {
			likes[k] = str
		} else {
			likes[k] = fmt.Sprintf("%v", v)
		}
	}

	dislikes := make(map[string]string)
	for k, v := range summary.DislikesJSON {
		if str, ok := v.(string); ok {
			dislikes[k] = str
		} else {
			dislikes[k] = fmt.Sprintf("%v", v)
		}
	}

	competencies := make(map[string]string)
	for k, v := range summary.CompetenciesJSON {
		if str, ok := v.(string); ok {
			competencies[k] = str
		} else {
			competencies[k] = fmt.Sprintf("%v", v)
		}
	}

	proto := &adminpb.UserSummary{
		Id:           summary.ID,
		ChatId:       summary.ChatID,
		UserId:       summary.UserID,
		Likes:        likes,
		Dislikes:     dislikes,
		Competencies: competencies,
		CreatedAt:    timestamppb.New(summary.CreatedAt),
		UpdatedAt:    timestamppb.New(summary.UpdatedAt),
	}

	if summary.Traits != nil {
		proto.Traits = summary.Traits
	}

	return proto
}
