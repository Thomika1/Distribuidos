package main

import (
	"log"
	"os"

	"github.com/Thomika1/Distribuidos/pkg/handler"
	"github.com/Thomika1/Distribuidos/pkg/middleware"
	"github.com/Thomika1/Distribuidos/pkg/models"
	"github.com/gin-gonic/gin"
	"gorm.io/driver/postgres"
	"gorm.io/gorm"
)

func main() {
	dsn := getEnv("DB_DSN", "host=localhost port=5432 user=postgres password=postgres dbname=chargebacks sslmode=disable")

	db, err := gorm.Open(postgres.Open(dsn), &gorm.Config{})
	if err != nil {
		log.Fatalf("failed to connect to database: %v", err)
	}

	if err := models.AutoMigrate(db); err != nil {
		log.Fatalf("failed to auto-migrate: %v", err)
	}

	r := gin.Default()

	h := handler.NewChargebackHandler(db)

	// Endpoints sem middleware de lock (para comparação)
	public := r.Group("/api/v1")
	{
		public.GET("/health", h.Health)
		public.POST("/chargebacks/nolock", h.Create)
	}

	// Endpoints com middleware 2PL (lock de duas fases)
	locked := r.Group("/api/v1")
	locked.Use(middleware.ConcurrencyMiddleware())
	{
		locked.POST("/chargebacks", h.Create)
		locked.GET("/chargebacks", h.List)
		locked.GET("/chargebacks/:id", h.Get)
		locked.POST("/chargebacks/validate", h.Validate)
	}

	port := getEnv("PORT", "8080")
	log.Printf("server starting on :%s", port)
	if err := r.Run(":" + port); err != nil {
		log.Fatalf("failed to start server: %v", err)
	}
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}