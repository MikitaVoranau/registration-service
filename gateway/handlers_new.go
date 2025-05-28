package main

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
	"google.golang.org/grpc"
)

type LoginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type RegisterRequest struct {
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

	// Call auth service login
	// Note: You'll need to implement the actual gRPC call here based on your auth service proto
	// This is a placeholder for the actual implementation
	token := "dummy-token" // Replace with actual token from auth service

	c.JSON(http.StatusOK, gin.H{"token": token})
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

	// Call auth service register
	// Note: You'll need to implement the actual gRPC call here based on your auth service proto
	// This is a placeholder for the actual implementation

	c.JSON(http.StatusOK, gin.H{"message": "Registration successful"})
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

		// Verify token with auth service
		// Note: You'll need to implement the actual token verification here
		// This is a placeholder for the actual implementation
		if token == "" {
			c.JSON(http.StatusUnauthorized, gin.H{"error": "Invalid token"})
			c.Abort()
			return
		}

		// Set user context
		c.Set("user_id", "dummy-user-id") // Replace with actual user ID from token
		c.Next()
	}
}

func handleFileUpload(c *gin.Context) {
	_, _ = c.Get("user_id")

	file, err := c.FormFile("file")
	if err != nil {
		c.JSON(http.StatusBadRequest, gin.H{"error": "No file provided"})
		return
	}

	// Connect to file service
	conn, err := grpc.Dial(os.Getenv("FILE_SERVICE_ADDR"), grpc.WithInsecure())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to file service"})
		return
	}
	defer conn.Close()

	// Open the file
	src, err := file.Open()
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to open file"})
		return
	}
	defer src.Close()

	// Call file service upload
	// Note: You'll need to implement the actual gRPC call here based on your file service proto
	// This is a placeholder for the actual implementation

	c.JSON(http.StatusOK, gin.H{"message": "File uploaded successfully"})
}

func handleListFiles(c *gin.Context) {
	_, _ = c.Get("user_id")

	// Connect to file service
	conn, err := grpc.Dial(os.Getenv("FILE_SERVICE_ADDR"), grpc.WithInsecure())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to file service"})
		return
	}
	defer conn.Close()

	// Call file service list files
	// Note: You'll need to implement the actual gRPC call here based on your file service proto
	// This is a placeholder for the actual implementation
	files := []gin.H{
		{
			"id":       "1",
			"name":     "example.txt",
			"size":     1024,
			"uploaded": "2024-03-20T12:00:00Z",
		},
	}

	c.JSON(http.StatusOK, files)
}

func handleDownloadFile(c *gin.Context) {
	_, _ = c.Get("user_id")
	_ = c.Param("id")

	// Connect to file service
	conn, err := grpc.Dial(os.Getenv("FILE_SERVICE_ADDR"), grpc.WithInsecure())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to file service"})
		return
	}
	defer conn.Close()

	// Call file service download
	// Note: You'll need to implement the actual gRPC call here based on your file service proto
	// This is a placeholder for the actual implementation

	// Set response headers for file download
	c.Header("Content-Disposition", "attachment; filename=example.txt")
	c.Header("Content-Type", "application/octet-stream")

	// Write file content to response
	c.String(http.StatusOK, "Example file content")
}

func handleDeleteFile(c *gin.Context) {
	_, _ = c.Get("user_id")
	_ = c.Param("id")

	// Connect to file service
	conn, err := grpc.Dial(os.Getenv("FILE_SERVICE_ADDR"), grpc.WithInsecure())
	if err != nil {
		c.JSON(http.StatusInternalServerError, gin.H{"error": "Failed to connect to file service"})
		return
	}
	defer conn.Close()

	// Call file service delete
	// Note: You'll need to implement the actual gRPC call here based on your file service proto
	// This is a placeholder for the actual implementation

	c.JSON(http.StatusOK, gin.H{"message": "File deleted successfully"})
}
