package main

import (
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
	"time"

	authpb "registration-service/api/authproto/proto-generate"
	filepb "registration-service/api/fileproto/proto-generate"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
	"google.golang.org/grpc/metadata"
)

type LoginRequest struct {
	Username string `json:"username"`
	Password string `json:"password"`
}

type RegisterRequest struct {
	Username string `json:"username"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

func handleLogin(c *gin.Context) {
	var req LoginRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Connect to auth service
	conn, err := grpc.Dial(os.Getenv("AUTH_SERVICE_ADDR"), grpc.WithInsecure())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to auth service"})
		return
	}
	defer conn.Close()

	client := authpb.NewAuthServiceClient(conn)
	resp, err := client.Login(context.Background(), &authpb.LoginRequest{
		Username: req.Username,
		Password: req.Password,
	})

	if err != nil {
		c.JSON(http.StatusUnauthorized, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"token":        resp.Token,
		"refreshToken": resp.RefreshToken,
	})
}

func handleRegister(c *gin.Context) {
	var req RegisterRequest
	if err := c.ShouldBindJSON(&req); err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	// Connect to auth service
	conn, err := grpc.Dial(os.Getenv("AUTH_SERVICE_ADDR"), grpc.WithInsecure())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to auth service"})
		return
	}
	defer conn.Close()

	client := authpb.NewAuthServiceClient(conn)
	resp, err := client.Register(context.Background(), &authpb.RegisterRequest{
		Username: req.Username,
		Email:    req.Email,
		Password: req.Password,
	})

	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
		return
	}

	c.JSON(http.StatusOK, gin.H{"message": resp.Message})
}

func authMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		auth := c.GetHeader("Authorization")
		if auth == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "No authorization header"})
			c.Abort()
			return
		}

		// Extract token
		parts := strings.Split(auth, " ")
		if len(parts) != 2 || parts[0] != "Bearer" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid authorization header"})
			c.Abort()
			return
		}

		token := parts[1]

		// Connect to auth service
		conn, err := grpc.Dial(os.Getenv("AUTH_SERVICE_ADDR"), grpc.WithInsecure())
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to auth service"})
			c.Abort()
			return
		}
		defer conn.Close()

		// Verify token
		client := authpb.NewAuthServiceClient(conn)
		resp, err := client.GetUIDByToken(context.Background(), &authpb.GetUIDByTokenRequest{
			Token: token,
		})

		if err != nil || !resp.IsValid {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Set user context
		c.Set("userID", resp.Uid)
		c.Next()
	}
}

func handleFileUpload(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	uploadedFile, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": fmt.Sprintf("No file provided: %v", err)})
		return
	}

	// Connect to file service
	conn, err := grpc.Dial(os.Getenv("FILE_SERVICE_ADDR"), grpc.WithInsecure())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to connect to file service: %v", err)})
		return
	}
	defer conn.Close()

	// Create context with userID in metadata
	md := metadata.New(map[string]string{
		"user_id": fmt.Sprint(userID),
	})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	client := filepb.NewFileServiceClient(conn)
	stream, err := client.UploadFile(ctx)
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to start file upload: %v", err)})
		return
	}

	// Send metadata
	err = stream.Send(&filepb.UploadFileRequest{
		Data: &filepb.UploadFileRequest_Metadata{
			Metadata: &filepb.FileMetadata{
				Name:        uploadedFile.Filename,
				ContentType: uploadedFile.Header.Get("Content-Type"),
			},
		},
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to send file metadata: %v", err)})
		return
	}

	// Open and read file
	src, err := uploadedFile.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to open file: %v", err)})
		return
	}
	defer src.Close()

	// Send file data in chunks
	buffer := make([]byte, 32*1024) // 32KB chunks
	for {
		n, err := src.Read(buffer)
		if err == io.EOF {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to read file: %v", err)})
			return
		}

		err = stream.Send(&filepb.UploadFileRequest{
			Data: &filepb.UploadFileRequest_Chunk{
				Chunk: buffer[:n],
			},
		})
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to upload file chunk: %v", err)})
			return
		}
	}

	// Close stream and get response
	resp, err := stream.CloseAndRecv()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": fmt.Sprintf("Failed to complete file upload: %v", err)})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"fileId":  resp.FileId,
		"message": resp.Message,
	})
}

func handleListFiles(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	// Connect to file service
	conn, err := grpc.Dial(os.Getenv("FILE_SERVICE_ADDR"), grpc.WithInsecure())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to file service"})
		return
	}
	defer conn.Close()

	// Create context with userID in metadata
	md := metadata.New(map[string]string{
		"user_id": fmt.Sprint(userID),
	})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	client := filepb.NewFileServiceClient(conn)
	resp, err := client.ListFiles(ctx, &filepb.ListFilesRequest{
		IncludeShared: true,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to list files"})
		return
	}

	files := make([]gin.H, 0, len(resp.Files))
	for _, file := range resp.Files {
		files = append(files, gin.H{
			"id":       file.FileId,
			"name":     file.Name,
			"size":     file.Size,
			"version":  file.Version,
			"uploaded": time.Unix(file.CreatedAt, 0).Format(time.RFC3339),
			"updated":  time.Unix(file.UpdatedAt, 0).Format(time.RFC3339),
			"is_owner": file.IsOwner,
		})
	}

	c.JSON(http.StatusOK, files)
}

func handleDownloadFile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File ID is required"})
		return
	}

	// Connect to file service
	conn, err := grpc.Dial(os.Getenv("FILE_SERVICE_ADDR"), grpc.WithInsecure())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to file service"})
		return
	}
	defer conn.Close()

	// Create context with userID in metadata
	md := metadata.New(map[string]string{
		"user_id": fmt.Sprint(userID),
	})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	client := filepb.NewFileServiceClient(conn)

	// Get file info first
	fileInfo, err := client.GetFileInfo(ctx, &filepb.GetFileInfoRequest{
		FileId: fileID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to get file info"})
		return
	}

	// Start download stream
	stream, err := client.DownloadFile(ctx, &filepb.DownloadFileRequest{
		FileId: fileID,
	})
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to start file download"})
		return
	}

	// Set response headers
	c.Header("Content-Disposition", fmt.Sprintf("attachment; filename=%s", fileInfo.File.Name))
	c.Header("Content-Type", fileInfo.File.ContentType)

	// Stream file data to response
	for {
		resp, err := stream.Recv()
		if err == io.EOF {
			break
		}
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to download file"})
			return
		}

		_, err = c.Writer.Write(resp.Chunk)
		if err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to write file data"})
			return
		}
	}
}

func handleDeleteFile(c *gin.Context) {
	userID, exists := c.Get("userID")
	if !exists {
		c.JSON(http.StatusUnauthorized, gin.H{"error": "User not authenticated"})
		return
	}

	fileID := c.Param("id")
	if fileID == "" {
		c.JSON(http.StatusBadRequest, gin.H{"error": "File ID is required"})
		return
	}

	// Connect to file service
	conn, err := grpc.Dial(os.Getenv("FILE_SERVICE_ADDR"), grpc.WithInsecure())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to file service"})
		return
	}
	defer conn.Close()

	// Create context with userID in metadata
	md := metadata.New(map[string]string{
		"user_id": fmt.Sprint(userID),
	})
	ctx := metadata.NewOutgoingContext(context.Background(), md)

	client := filepb.NewFileServiceClient(conn)
	resp, err := client.DeleteFile(ctx, &filepb.DeleteFileRequest{
		FileId: fileID,
	})

	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to delete file"})
		return
	}

	c.JSON(http.StatusOK, gin.H{
		"success": resp.Success,
		"message": resp.Message,
	})
}
