package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"

	"github.com/LuizHVicari/webinar-backend/internal/organization"
	"github.com/LuizHVicari/webinar-backend/internal/user"
	"github.com/LuizHVicari/webinar-backend/pkg/config"
	"github.com/LuizHVicari/webinar-backend/pkg/keto"
	"github.com/LuizHVicari/webinar-backend/pkg/middleware"
	db "github.com/LuizHVicari/webinar-backend/sqlc/generated"
)

func main() {
	_ = godotenv.Load()

	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		log.Fatalf("db: %v", err)
	}
	defer pool.Close()

	queries := db.New(pool)
	ketoClient := keto.New(cfg.KetoReadURL, cfg.KetoWriteURL)

	orgRepo := organization.NewOrganizationRepository(queries)
	orgSvc := organization.NewOrganizationService(orgRepo)

	inviteRepo := organization.NewInviteRepository(queries)
	inviteSvc := organization.NewInviteService(inviteRepo, ketoClient)

	userRepo := user.NewUserRepository(queries)
	userSvc := user.NewUserService(userRepo, orgSvc, inviteSvc, ketoClient)

	orgHandler := organization.NewHandler(inviteSvc)
	userHandler := user.NewHandler(userSvc)

	router := gin.Default()

	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"status": "ok"})
	})

	auth := router.Group("/", middleware.Auth(cfg.KratosPublicURL, userSvc))
	orgHandler.RegisterRoutes(auth)
	userHandler.RegisterRoutes(auth)

	log.Fatal(router.Run(":" + cfg.Port))
}
