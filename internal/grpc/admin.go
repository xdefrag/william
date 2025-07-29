package grpc

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"log/slog"
	"time"

	"github.com/ThreeDotsLabs/watermill/message"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/xdefrag/william/internal/bot"
	"github.com/xdefrag/william/internal/config"
	"github.com/xdefrag/william/internal/repo"
	"github.com/xdefrag/william/pkg/adminpb"
	"github.com/xdefrag/william/pkg/models"
)

// Role constants
const (
	RoleAdmin     = "admin"
	RoleModerator = "moderator"
	RoleViewer    = "viewer"
)

// AdminService implements the AdminServiceServer interface
type AdminService struct {
	adminpb.UnimplementedAdminServiceServer
	config    *config.Config
	repo      *repo.Repository
	publisher message.Publisher
	logger    *slog.Logger
}

// NewAdminService creates a new AdminService instance
func NewAdminService(cfg *config.Config, repository *repo.Repository, publisher message.Publisher, logger *slog.Logger) *AdminService {
	return &AdminService{
		config:    cfg,
		repo:      repository,
		publisher: publisher,
		logger:    logger,
	}
}

// GetChatSummary retrieves summaries for one or multiple chats
func (s *AdminService) GetChatSummary(ctx context.Context, req *adminpb.GetChatSummaryRequest) (*adminpb.GetChatSummaryResponse, error) {
	if len(req.ChatIds) == 0 {
		return nil, status.Error(codes.InvalidArgument, "at least one chat_id is required")
	}

	// Check view permissions for all requested chats
	for _, chatID := range req.ChatIds {
		if err := s.checkChatPermission(ctx, chatID, false); err != nil {
			return nil, err
		}
	}

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

	return &adminpb.GetChatSummaryResponse{Summaries: summaries}, nil
}

// GetUserSummary retrieves summaries for one or multiple users in a chat
func (s *AdminService) GetUserSummary(ctx context.Context, req *adminpb.GetUserSummaryRequest) (*adminpb.GetUserSummaryResponse, error) {
	if req.ChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id is required")
	}

	// Check view permissions for the chat
	if err := s.checkChatPermission(ctx, req.ChatId, false); err != nil {
		return nil, err
	}

	var summaries []*adminpb.UserSummary

	// If no specific user IDs provided, get all user summaries for the chat
	if len(req.UserIds) == 0 {
		allSummaries, err := s.repo.GetAllUserSummariesByChatID(ctx, req.ChatId)
		if err != nil {
			s.logger.Error("Failed to get all user summaries",
				slog.Int64("chat_id", req.ChatId),
				slog.String("error", err.Error()),
			)
			return nil, status.Error(codes.Internal, "failed to retrieve user summaries")
		}

		for _, summary := range allSummaries {
			summaries = append(summaries, s.userSummaryToProto(summary))
		}
	} else {
		// Get summaries for specific user IDs
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
	}

	return &adminpb.GetUserSummaryResponse{Summaries: summaries}, nil
}

// TriggerSummarization manually triggers summarization for a chat
func (s *AdminService) TriggerSummarization(ctx context.Context, req *adminpb.TriggerSummarizationRequest) (*adminpb.TriggerSummarizationResponse, error) {
	if req.ChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id is required")
	}

	// Check mutation permissions for the chat (admin or moderator)
	if err := s.checkChatPermission(ctx, req.ChatId, true); err != nil {
		return nil, err
	}

	// Generate unique event ID for tracking
	eventID := make([]byte, 16) // 128 bits
	if _, err := rand.Read(eventID); err != nil {
		s.logger.Error("Failed to generate event ID",
			slog.Int64("chat_id", req.ChatId),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "failed to generate event ID")
	}

	eventIDStr := hex.EncodeToString(eventID)
	s.logger.Info("Manual summarization triggered",
		slog.Int64("chat_id", req.ChatId),
		slog.String("event_id", eventIDStr),
	)

	// Create and publish summarize event
	event := bot.SummarizeEvent{
		ChatID:    req.ChatId,
		Timestamp: time.Now(),
	}

	eventData, err := event.Marshal()
	if err != nil {
		s.logger.Error("Failed to marshal summarize event",
			slog.Int64("chat_id", req.ChatId),
			slog.String("event_id", eventIDStr),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "failed to create summarization event")
	}

	// Publish the event with the event ID as message ID
	if err := s.publisher.Publish("summarize", message.NewMessage(eventIDStr, eventData)); err != nil {
		s.logger.Error("Failed to publish summarize event",
			slog.Int64("chat_id", req.ChatId),
			slog.String("event_id", eventIDStr),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "failed to trigger summarization")
	}

	s.logger.Info("Summarization event published successfully",
		slog.Int64("chat_id", req.ChatId),
		slog.String("event_id", eventIDStr),
	)

	return &adminpb.TriggerSummarizationResponse{
		EventId: eventIDStr,
	}, nil
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

	if summary.Username != nil {
		proto.Username = summary.Username
	}
	if summary.FirstName != nil {
		proto.FirstName = summary.FirstName
	}
	if summary.LastName != nil {
		proto.LastName = summary.LastName
	}
	if summary.Traits != nil {
		proto.Traits = summary.Traits
	}

	return proto
}

// GetMyChats retrieves chats accessible by the current user
func (s *AdminService) GetMyChats(ctx context.Context, req *adminpb.GetMyChatsRequest) (*adminpb.GetMyChatsResponse, error) {
	userID, ok := ctx.Value(TelegramUserIDKey).(int64)
	if !ok {
		return nil, status.Error(codes.Unauthenticated, "user ID not found in context")
	}

	// Get chat IDs where the user has any role
	chatIDs, err := s.repo.GetUserChats(ctx, userID)
	if err != nil {
		s.logger.Error("Failed to get user chats",
			slog.Int64("user_id", userID),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "failed to retrieve user chats")
	}

	s.logger.Info("Retrieved user chats",
		slog.Int64("user_id", userID),
		slog.Int("chat_count", len(chatIDs)),
	)

	return &adminpb.GetMyChatsResponse{ChatIds: chatIDs}, nil
}

// Role management handlers

// isAdmin checks if the user has admin privileges
func (s *AdminService) isAdmin(ctx context.Context) error {
	userID, ok := ctx.Value(TelegramUserIDKey).(int64)
	if !ok {
		return status.Error(codes.Unauthenticated, "user ID not found in context")
	}

	if s.config.AdminUserID == 0 {
		return status.Error(codes.PermissionDenied, "admin user not configured")
	}

	if userID != s.config.AdminUserID {
		s.logger.Warn("Non-admin user attempted admin operation",
			slog.Int64("user_id", userID),
			slog.Int64("admin_user_id", s.config.AdminUserID),
		)
		return status.Error(codes.PermissionDenied, "admin privileges required")
	}

	return nil
}

// checkChatPermission checks if user has permission for chat operations
// isMutation: true for operations that modify data (admin/moderator), false for read-only (admin/moderator/viewer)
func (s *AdminService) checkChatPermission(ctx context.Context, chatID int64, isMutation bool) error {
	userID, ok := ctx.Value(TelegramUserIDKey).(int64)
	if !ok {
		return status.Error(codes.Unauthenticated, "user ID not found in context")
	}

	// Global admin has access to everything
	if s.config.AdminUserID != 0 && userID == s.config.AdminUserID {
		return nil
	}

	// Check user role in chat
	userRole, err := s.repo.GetUserRole(ctx, userID, chatID)
	if err != nil {
		s.logger.Warn("User has no role in chat",
			slog.Int64("user_id", userID),
			slog.Int64("chat_id", chatID),
		)
		return status.Error(codes.PermissionDenied, "no access to this chat")
	}

	// Check if role has expired
	if userRole.ExpiresAt != nil && time.Now().After(*userRole.ExpiresAt) {
		s.logger.Warn("User role has expired",
			slog.Int64("user_id", userID),
			slog.Int64("chat_id", chatID),
			slog.String("role", userRole.Role),
			slog.Time("expired_at", *userRole.ExpiresAt),
		)
		return status.Error(codes.PermissionDenied, "role has expired")
	}

	// Check permissions based on operation type
	if isMutation {
		// Mutation operations: only admin and moderator
		if userRole.Role != RoleAdmin && userRole.Role != RoleModerator {
			s.logger.Warn("Insufficient permissions for mutation operation",
				slog.Int64("user_id", userID),
				slog.Int64("chat_id", chatID),
				slog.String("role", userRole.Role),
			)
			return status.Error(codes.PermissionDenied, "insufficient permissions for this operation")
		}
	} else {
		// Read operations: admin, moderator, and viewer
		if userRole.Role != RoleAdmin && userRole.Role != RoleModerator && userRole.Role != RoleViewer {
			s.logger.Warn("Insufficient permissions for read operation",
				slog.Int64("user_id", userID),
				slog.Int64("chat_id", chatID),
				slog.String("role", userRole.Role),
			)
			return status.Error(codes.PermissionDenied, "insufficient permissions for this operation")
		}
	}

	return nil
}

// GetUserRoles retrieves all user roles for a chat
func (s *AdminService) GetUserRoles(ctx context.Context, req *adminpb.GetUserRolesRequest) (*adminpb.GetUserRolesResponse, error) {
	// Check admin privileges
	if err := s.isAdmin(ctx); err != nil {
		return nil, err
	}

	if req.TelegramChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "telegram_chat_id is required")
	}

	roles, err := s.repo.GetUserRolesByChatID(ctx, req.TelegramChatId)
	if err != nil {
		s.logger.Error("Failed to get user roles",
			slog.Int64("chat_id", req.TelegramChatId),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "failed to retrieve user roles")
	}

	var protoRoles []*adminpb.UserRole
	for _, role := range roles {
		protoRoles = append(protoRoles, s.userRoleToProto(role))
	}

	return &adminpb.GetUserRolesResponse{Roles: protoRoles}, nil
}

// SetUserRole assigns a role to a user in a chat
func (s *AdminService) SetUserRole(ctx context.Context, req *adminpb.SetUserRoleRequest) (*adminpb.SetUserRoleResponse, error) {
	// Check admin privileges
	if err := s.isAdmin(ctx); err != nil {
		return nil, err
	}

	if req.TelegramUserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "telegram_user_id is required")
	}
	if req.TelegramChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "telegram_chat_id is required")
	}
	if req.Role == "" {
		return nil, status.Error(codes.InvalidArgument, "role is required")
	}

	var expiresAt *time.Time
	if req.ExpiresAt != nil {
		expiry := req.ExpiresAt.AsTime()
		expiresAt = &expiry
	}

	role, err := s.repo.SetUserRole(ctx, req.TelegramUserId, req.TelegramChatId, req.Role, expiresAt)
	if err != nil {
		s.logger.Error("Failed to set user role",
			slog.Int64("user_id", req.TelegramUserId),
			slog.Int64("chat_id", req.TelegramChatId),
			slog.String("role", req.Role),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "failed to set user role")
	}

	s.logger.Info("User role set successfully",
		slog.Int64("user_id", req.TelegramUserId),
		slog.Int64("chat_id", req.TelegramChatId),
		slog.String("role", req.Role),
		slog.Int64("role_id", role.ID),
	)

	return &adminpb.SetUserRoleResponse{
		RoleId: role.ID,
	}, nil
}

// RemoveUserRole removes a user's role from a chat
func (s *AdminService) RemoveUserRole(ctx context.Context, req *adminpb.RemoveUserRoleRequest) (*adminpb.RemoveUserRoleResponse, error) {
	// Check admin privileges
	if err := s.isAdmin(ctx); err != nil {
		return nil, err
	}

	if req.TelegramUserId == 0 {
		return nil, status.Error(codes.InvalidArgument, "telegram_user_id is required")
	}
	if req.TelegramChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "telegram_chat_id is required")
	}

	err := s.repo.RemoveUserRole(ctx, req.TelegramUserId, req.TelegramChatId)
	if err != nil {
		s.logger.Error("Failed to remove user role",
			slog.Int64("user_id", req.TelegramUserId),
			slog.Int64("chat_id", req.TelegramChatId),
			slog.String("error", err.Error()),
		)

		if err.Error() == "user role not found" {
			return nil, status.Error(codes.NotFound, "user role not found")
		}

		return nil, status.Error(codes.Internal, "failed to remove user role")
	}

	s.logger.Info("User role removed successfully",
		slog.Int64("user_id", req.TelegramUserId),
		slog.Int64("chat_id", req.TelegramChatId),
	)

	return &adminpb.RemoveUserRoleResponse{}, nil
}

// userRoleToProto converts a UserRole model to protobuf message
func (s *AdminService) userRoleToProto(role *models.UserRole) *adminpb.UserRole {
	proto := &adminpb.UserRole{
		Id:             role.ID,
		TelegramUserId: role.TelegramUserID,
		TelegramChatId: role.TelegramChatID,
		Role:           role.Role,
		CreatedAt:      timestamppb.New(role.CreatedAt),
		UpdatedAt:      timestamppb.New(role.UpdatedAt),
	}

	if role.ExpiresAt != nil {
		proto.ExpiresAt = timestamppb.New(*role.ExpiresAt)
	}

	return proto
}

// Allowed chats management handlers

// GetAllowedChats retrieves all allowed chats
func (s *AdminService) GetAllowedChats(ctx context.Context, req *adminpb.GetAllowedChatsRequest) (*adminpb.GetAllowedChatsResponse, error) {
	// Check admin privileges
	if err := s.isAdmin(ctx); err != nil {
		return nil, err
	}

	chats, err := s.repo.GetAllowedChatsDetailed(ctx)
	if err != nil {
		s.logger.Error("Failed to get allowed chats",
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "failed to retrieve allowed chats")
	}

	var protoChats []*adminpb.AllowedChat
	for _, chat := range chats {
		protoChats = append(protoChats, s.allowedChatToProto(chat))
	}

	return &adminpb.GetAllowedChatsResponse{Chats: protoChats}, nil
}

// AddAllowedChat adds a chat to the allowed list
func (s *AdminService) AddAllowedChat(ctx context.Context, req *adminpb.AddAllowedChatRequest) (*adminpb.AddAllowedChatResponse, error) {
	// Check admin privileges
	if err := s.isAdmin(ctx); err != nil {
		return nil, err
	}

	if req.ChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id is required")
	}

	var name *string
	if req.Name != nil {
		name = req.Name
	}

	chat, err := s.repo.AddAllowedChatDetailed(ctx, req.ChatId, name)
	if err != nil {
		s.logger.Error("Failed to add allowed chat",
			slog.Int64("chat_id", req.ChatId),
			slog.String("error", err.Error()),
		)
		return nil, status.Error(codes.Internal, "failed to add allowed chat")
	}

	s.logger.Info("Allowed chat added successfully",
		slog.Int64("chat_id", req.ChatId),
		slog.Int64("record_id", chat.ID),
		slog.String("name", func() string {
			if name != nil {
				return *name
			}
			return ""
		}()),
	)

	return &adminpb.AddAllowedChatResponse{
		ChatId: chat.ID,
	}, nil
}

// RemoveAllowedChat removes a chat from the allowed list
func (s *AdminService) RemoveAllowedChat(ctx context.Context, req *adminpb.RemoveAllowedChatRequest) (*adminpb.RemoveAllowedChatResponse, error) {
	// Check admin privileges
	if err := s.isAdmin(ctx); err != nil {
		return nil, err
	}

	if req.ChatId == 0 {
		return nil, status.Error(codes.InvalidArgument, "chat_id is required")
	}

	err := s.repo.RemoveAllowedChat(ctx, req.ChatId)
	if err != nil {
		s.logger.Error("Failed to remove allowed chat",
			slog.Int64("chat_id", req.ChatId),
			slog.String("error", err.Error()),
		)

		if err.Error() == "allowed chat not found" {
			return nil, status.Error(codes.NotFound, "allowed chat not found")
		}

		return nil, status.Error(codes.Internal, "failed to remove allowed chat")
	}

	s.logger.Info("Allowed chat removed successfully",
		slog.Int64("chat_id", req.ChatId),
	)

	return &adminpb.RemoveAllowedChatResponse{}, nil
}

// allowedChatToProto converts an AllowedChat model to protobuf message
func (s *AdminService) allowedChatToProto(chat *models.AllowedChat) *adminpb.AllowedChat {
	proto := &adminpb.AllowedChat{
		Id:        chat.ID,
		ChatId:    chat.ChatID,
		CreatedAt: timestamppb.New(chat.CreatedAt),
	}

	if chat.Name != nil {
		proto.Name = chat.Name
	}

	return proto
}
