package main

import (
	"log"
	"os"

	"github.com/gin-contrib/cors"
	"github.com/gin-gonic/gin"
)

func main() {
	r := gin.Default()

	// Enable CORS
	config := cors.DefaultConfig()
	config.AllowAllOrigins = true
	config.AllowHeaders = []string{"Origin", "Content-Length", "Content-Type", "Authorization"}
	r.Use(cors.New(config))

	// Serve static files
	r.Static("/static", "./static")
	r.LoadHTMLGlob("templates/*")

	// Routes
	r.GET("/", func(c *gin.Context) {
		c.HTML(200, "index.html", gin.H{
			"title": "File Storage Service",
		})
	})

	// API routes will be added here
	api := r.Group("/api")
	{
		// Auth routes
		api.POST("/register", handleRegister)
		api.POST("/login", handleLogin)

		// File routes
		authorized := api.Group("/")
		authorized.Use(authMiddleware())
		{
			authorized.POST("/upload", handleFileUpload)
			authorized.GET("/files", handleListFiles)
			authorized.GET("/files/:id", handleDownloadFile)
			authorized.DELETE("/files/:id", handleDeleteFile)
		}
	}

	port := os.Getenv("PORT")
	if port == "" {
		port = "8080"
	}

	log.Printf("Gateway starting on port %s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("Failed to start server: %v", err)
	}
}
