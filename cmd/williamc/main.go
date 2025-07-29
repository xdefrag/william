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
	fmt.Printf("Success: %t\n", resp.Success)
	if resp.Message != nil {
		fmt.Printf("Message: %s\n", *resp.Message)
	}
	if resp.EventId != nil {
		fmt.Printf("Event ID: %s\n", *resp.EventId)
	}

	return nil
}
