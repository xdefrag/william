package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/joho/godotenv"
	"github.com/urfave/cli/v2"
	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials/insecure"
	"google.golang.org/grpc/metadata"
	"google.golang.org/protobuf/types/known/timestamppb"

	"github.com/xdefrag/william/internal/auth"
	"github.com/xdefrag/william/pkg/adminpb"
)

func main() {
	// Load .env file if it exists
	_ = godotenv.Load()

	app := &cli.App{
		Name:  "williamc",
		Usage: "William gRPC Admin Client",
		Flags: []cli.Flag{
			&cli.StringFlag{
				Name:    "server",
				Aliases: []string{"s"},
				Value:   "localhost:8080",
				Usage:   "gRPC server address",
				EnvVars: []string{"WILLIAM_SERVER"},
			},
			&cli.Int64Flag{
				Name:     "telegram-user-id",
				Aliases:  []string{"u"},
				Usage:    "Telegram user ID for JWT token",
				Required: true,
			},
			&cli.StringFlag{
				Name:    "jwt-secret",
				Usage:   "JWT secret for token generation",
				EnvVars: []string{"JWT_SECRET"},
			},
			&cli.DurationFlag{
				Name:  "token-duration",
				Value: 24 * time.Hour,
				Usage: "JWT token validity duration",
			},
		},
		Commands: []*cli.Command{
			{
				Name:  "generate-token",
				Usage: "Generate JWT token",
				Flags: []cli.Flag{
					&cli.DurationFlag{
						Name:  "duration",
						Value: 24 * time.Hour,
						Usage: "Token validity duration",
					},
				},
				Action: generateTokenAction,
			},
			{
				Name:  "get-chat-summary",
				Usage: "Get chat summaries",
				Flags: []cli.Flag{
					&cli.Int64SliceFlag{
						Name:     "chat-ids",
						Aliases:  []string{"c"},
						Usage:    "Chat IDs to get summaries for",
						Required: true,
					},
				},
				Action: getChatSummaryAction,
			},
			{
				Name:  "get-user-summary",
				Usage: "Get user summaries",
				Flags: []cli.Flag{
					&cli.Int64Flag{
						Name:     "chat-id",
						Usage:    "Chat ID",
						Required: true,
					},
					&cli.Int64SliceFlag{
						Name:     "user-ids",
						Usage:    "User IDs to get summaries for",
						Required: true,
					},
				},
				Action: getUserSummaryAction,
			},
			{
				Name:  "trigger-summarization",
				Usage: "Trigger manual summarization",
				Flags: []cli.Flag{
					&cli.Int64Flag{
						Name:     "chat-id",
						Usage:    "Chat ID to summarize",
						Required: true,
					},
				},
				Action: triggerSummarizationAction,
			},
			{
				Name:  "get-my-chats",
				Usage: "Get chats accessible by current user",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "token",
						Usage:    "JWT token for authentication",
						Required: true,
					},
				},
				Action: getMyChatsAction,
			},
			{
				Name:  "get-user-roles",
				Usage: "Get all user roles for a chat",
				Flags: []cli.Flag{
					&cli.Int64Flag{
						Name:     "chat-id",
						Usage:    "Chat ID to get roles for",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "token",
						Usage:    "JWT token for authentication",
						Required: true,
					},
				},
				Action: getUserRolesAction,
			},
			{
				Name:  "set-user-role",
				Usage: "Set a user role in a chat",
				Flags: []cli.Flag{
					&cli.Int64Flag{
						Name:     "user-id",
						Usage:    "Telegram user ID",
						Required: true,
					},
					&cli.Int64Flag{
						Name:     "chat-id",
						Usage:    "Telegram chat ID",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "role",
						Usage:    "Role to assign",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "expires-at",
						Usage: "Role expiration time (RFC3339 format, e.g. 2024-12-31T23:59:59Z)",
					},
					&cli.StringFlag{
						Name:     "token",
						Usage:    "JWT token for authentication",
						Required: true,
					},
				},
				Action: setUserRoleAction,
			},
			{
				Name:  "remove-user-role",
				Usage: "Remove a user role from a chat",
				Flags: []cli.Flag{
					&cli.Int64Flag{
						Name:     "user-id",
						Usage:    "Telegram user ID",
						Required: true,
					},
					&cli.Int64Flag{
						Name:     "chat-id",
						Usage:    "Telegram chat ID",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "token",
						Usage:    "JWT token for authentication",
						Required: true,
					},
				},
				Action: removeUserRoleAction,
			},
			{
				Name:  "get-allowed-chats",
				Usage: "Get all allowed chats",
				Flags: []cli.Flag{
					&cli.StringFlag{
						Name:     "token",
						Usage:    "JWT token for authentication",
						Required: true,
					},
				},
				Action: getAllowedChatsAction,
			},
			{
				Name:  "add-allowed-chat",
				Usage: "Add a chat to the allowed list",
				Flags: []cli.Flag{
					&cli.Int64Flag{
						Name:     "chat-id",
						Usage:    "Telegram chat ID",
						Required: true,
					},
					&cli.StringFlag{
						Name:  "name",
						Usage: "Optional chat name/description",
					},
					&cli.StringFlag{
						Name:     "token",
						Usage:    "JWT token for authentication",
						Required: true,
					},
				},
				Action: addAllowedChatAction,
			},
			{
				Name:  "remove-allowed-chat",
				Usage: "Remove a chat from the allowed list",
				Flags: []cli.Flag{
					&cli.Int64Flag{
						Name:     "chat-id",
						Usage:    "Telegram chat ID",
						Required: true,
					},
					&cli.StringFlag{
						Name:     "token",
						Usage:    "JWT token for authentication",
						Required: true,
					},
				},
				Action: removeAllowedChatAction,
			},
		},
	}

	if err := app.Run(os.Args); err != nil {
		log.Fatal(err)
	}
}

// generateTokenForUser creates a JWT token using the provided context
func generateTokenForUser(c *cli.Context) (string, error) {
	telegramUserID := c.Int64("telegram-user-id")
	jwtSecret := c.String("jwt-secret")
	duration := c.Duration("token-duration")

	if jwtSecret == "" {
		return "", fmt.Errorf("JWT_SECRET is required (set via env var or --jwt-secret flag)")
	}

	jwtManager := auth.NewJWTManager(jwtSecret)
	token, err := jwtManager.GenerateToken(telegramUserID, duration)
	if err != nil {
		return "", fmt.Errorf("failed to generate token: %w", err)
	}

	return token, nil
}

func generateTokenAction(c *cli.Context) error {
	telegramUserID := c.Int64("telegram-user-id")
	jwtSecret := c.String("jwt-secret")
	duration := c.Duration("duration")

	if jwtSecret == "" {
		return fmt.Errorf("JWT_SECRET is required (set via env var or --jwt-secret flag)")
	}

	jwtManager := auth.NewJWTManager(jwtSecret)
	token, err := jwtManager.GenerateToken(telegramUserID, duration)
	if err != nil {
		return fmt.Errorf("failed to generate token: %w", err)
	}

	fmt.Printf("Generated JWT token for user %d (valid for %v):\n%s\n", telegramUserID, duration, token)
	return nil
}

func getChatSummaryAction(c *cli.Context) error {
	server := c.String("server")
	chatIDs := c.Int64Slice("chat-ids")

	// Generate token automatically
	token, err := generateTokenForUser(c)
	if err != nil {
		return fmt.Errorf("failed to generate JWT token: %w", err)
	}

	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close connection: %v\n", closeErr)
		}
	}()

	client := adminpb.NewAdminServiceClient(conn)

	// Add JWT token to metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.GetChatSummary(ctx, &adminpb.GetChatSummaryRequest{
		ChatIds: chatIDs,
	})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	fmt.Printf("Found %d chat summaries:\n", len(resp.Summaries))
	for i, summary := range resp.Summaries {
		fmt.Printf("\n--- Chat Summary %d ---\n", i+1)
		fmt.Printf("Chat ID: %d\n", summary.ChatId)
		fmt.Printf("Summary: %s\n", summary.Summary)
		fmt.Printf("Topics: %v\n", summary.Topics)
		if summary.NextEvents != nil {
			fmt.Printf("Next Events: %s\n", *summary.NextEvents)
		}
		fmt.Printf("Created: %s\n", summary.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated: %s\n", summary.UpdatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

func getUserSummaryAction(c *cli.Context) error {
	server := c.String("server")
	chatID := c.Int64("chat-id")
	userIDs := c.Int64Slice("user-ids")

	// Generate token automatically
	token, err := generateTokenForUser(c)
	if err != nil {
		return fmt.Errorf("failed to generate JWT token: %w", err)
	}

	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close connection: %v\n", closeErr)
		}
	}()

	client := adminpb.NewAdminServiceClient(conn)

	// Add JWT token to metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.GetUserSummary(ctx, &adminpb.GetUserSummaryRequest{
		ChatId:  chatID,
		UserIds: userIDs,
	})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	fmt.Printf("Found %d user summaries:\n", len(resp.Summaries))
	for i, summary := range resp.Summaries {
		fmt.Printf("\n--- User Summary %d ---\n", i+1)
		fmt.Printf("User ID: %d\n", summary.UserId)
		fmt.Printf("Chat ID: %d\n", summary.ChatId)
		fmt.Printf("Likes: %v\n", summary.Likes)
		fmt.Printf("Dislikes: %v\n", summary.Dislikes)
		fmt.Printf("Competencies: %v\n", summary.Competencies)
		if summary.Traits != nil {
			fmt.Printf("Traits: %s\n", *summary.Traits)
		}
		fmt.Printf("Created: %s\n", summary.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated: %s\n", summary.UpdatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

func triggerSummarizationAction(c *cli.Context) error {
	server := c.String("server")
	chatID := c.Int64("chat-id")

	// Generate token automatically
	token, err := generateTokenForUser(c)
	if err != nil {
		return fmt.Errorf("failed to generate JWT token: %w", err)
	}

	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close connection: %v\n", closeErr)
		}
	}()

	client := adminpb.NewAdminServiceClient(conn)

	// Add JWT token to metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.TriggerSummarization(ctx, &adminpb.TriggerSummarizationRequest{
		ChatId: chatID,
	})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	fmt.Printf("Summarization triggered for chat %d\n", chatID)
	fmt.Printf("Event ID: %s\n", resp.EventId)

	return nil
}

func getUserRolesAction(c *cli.Context) error {
	server := c.String("server")
	chatID := c.Int64("chat-id")
	token := c.String("token")

	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close connection: %v\n", closeErr)
		}
	}()

	client := adminpb.NewAdminServiceClient(conn)

	// Add JWT token to metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.GetUserRoles(ctx, &adminpb.GetUserRolesRequest{
		TelegramChatId: chatID,
	})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	fmt.Printf("Found %d user roles for chat %d:\n", len(resp.Roles), chatID)
	for i, role := range resp.Roles {
		fmt.Printf("\n--- Role %d ---\n", i+1)
		fmt.Printf("ID: %d\n", role.Id)
		fmt.Printf("User ID: %d\n", role.TelegramUserId)
		fmt.Printf("Chat ID: %d\n", role.TelegramChatId)
		fmt.Printf("Role: %s\n", role.Role)
		if role.ExpiresAt != nil {
			fmt.Printf("Expires At: %s\n", role.ExpiresAt.AsTime().Format("2006-01-02 15:04:05"))
		} else {
			fmt.Printf("Expires At: Never\n")
		}
		fmt.Printf("Created: %s\n", role.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
		fmt.Printf("Updated: %s\n", role.UpdatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

func setUserRoleAction(c *cli.Context) error {
	server := c.String("server")
	userID := c.Int64("user-id")
	chatID := c.Int64("chat-id")
	role := c.String("role")
	expiresAtStr := c.String("expires-at")
	token := c.String("token")

	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close connection: %v\n", closeErr)
		}
	}()

	client := adminpb.NewAdminServiceClient(conn)

	req := &adminpb.SetUserRoleRequest{
		TelegramUserId: userID,
		TelegramChatId: chatID,
		Role:           role,
	}

	// Parse expires_at if provided
	if expiresAtStr != "" {
		expiresAt, err := time.Parse(time.RFC3339, expiresAtStr)
		if err != nil {
			return fmt.Errorf("invalid expires-at format, use RFC3339 (e.g. 2024-12-31T23:59:59Z): %w", err)
		}
		req.ExpiresAt = timestamppb.New(expiresAt)
	}

	// Add JWT token to metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.SetUserRole(ctx, req)
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	fmt.Printf("User role set successfully\n")
	fmt.Printf("Role ID: %d\n", resp.RoleId)

	return nil
}

func removeUserRoleAction(c *cli.Context) error {
	server := c.String("server")
	userID := c.Int64("user-id")
	chatID := c.Int64("chat-id")
	token := c.String("token")

	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close connection: %v\n", closeErr)
		}
	}()

	client := adminpb.NewAdminServiceClient(conn)

	// Add JWT token to metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err = client.RemoveUserRole(ctx, &adminpb.RemoveUserRoleRequest{
		TelegramUserId: userID,
		TelegramChatId: chatID,
	})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	fmt.Printf("User role removed successfully\n")

	return nil
}

func getAllowedChatsAction(c *cli.Context) error {
	server := c.String("server")
	token := c.String("token")

	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close connection: %v\n", closeErr)
		}
	}()

	client := adminpb.NewAdminServiceClient(conn)

	// Add JWT token to metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.GetAllowedChats(ctx, &adminpb.GetAllowedChatsRequest{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	fmt.Printf("Found %d allowed chats:\n", len(resp.Chats))
	for i, chat := range resp.Chats {
		fmt.Printf("\n--- Chat %d ---\n", i+1)
		fmt.Printf("ID: %d\n", chat.Id)
		fmt.Printf("Chat ID: %d\n", chat.ChatId)
		if chat.Name != nil {
			fmt.Printf("Name: %s\n", *chat.Name)
		} else {
			fmt.Printf("Name: (not set)\n")
		}
		fmt.Printf("Created: %s\n", chat.CreatedAt.AsTime().Format("2006-01-02 15:04:05"))
	}

	return nil
}

func addAllowedChatAction(c *cli.Context) error {
	server := c.String("server")
	chatID := c.Int64("chat-id")
	name := c.String("name")
	token := c.String("token")

	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close connection: %v\n", closeErr)
		}
	}()

	client := adminpb.NewAdminServiceClient(conn)

	req := &adminpb.AddAllowedChatRequest{
		ChatId: chatID,
	}

	// Add name if provided
	if name != "" {
		req.Name = &name
	}

	// Add JWT token to metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.AddAllowedChat(ctx, req)
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	fmt.Printf("Allowed chat added successfully\n")
	fmt.Printf("Record ID: %d\n", resp.ChatId)

	return nil
}

func removeAllowedChatAction(c *cli.Context) error {
	server := c.String("server")
	chatID := c.Int64("chat-id")
	token := c.String("token")

	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close connection: %v\n", closeErr)
		}
	}()

	client := adminpb.NewAdminServiceClient(conn)

	// Add JWT token to metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	_, err = client.RemoveAllowedChat(ctx, &adminpb.RemoveAllowedChatRequest{
		ChatId: chatID,
	})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	fmt.Printf("Allowed chat removed successfully\n")

	return nil
}

func getMyChatsAction(c *cli.Context) error {
	server := c.String("server")
	token := c.String("token")

	conn, err := grpc.NewClient(server, grpc.WithTransportCredentials(insecure.NewCredentials()))
	if err != nil {
		return fmt.Errorf("failed to connect to server: %w", err)
	}
	defer func() {
		if closeErr := conn.Close(); closeErr != nil {
			fmt.Printf("Warning: failed to close connection: %v\n", closeErr)
		}
	}()

	client := adminpb.NewAdminServiceClient(conn)

	// Add JWT token to metadata
	ctx := metadata.AppendToOutgoingContext(context.Background(), "authorization", "Bearer "+token)
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	resp, err := client.GetMyChats(ctx, &adminpb.GetMyChatsRequest{})
	if err != nil {
		return fmt.Errorf("gRPC call failed: %w", err)
	}

	fmt.Printf("Found %d accessible chats:\n", len(resp.ChatIds))
	for i, chatID := range resp.ChatIds {
		fmt.Printf("Chat %d: %d\n", i+1, chatID)
	}

	return nil
}
