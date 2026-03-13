// @title           Webinar Backend API
// @version         1.0
// @description     REST API for the Webinar platform.
//
// @basePath        /
//
// @securityDefinitions.apikey  KratosSession
// @in                          cookie
// @name                        ory_kratos_session
// @description                 Ory Kratos session cookie obtained via the self-service login flow.
package main

import (
	"context"
	"log"
	"net/http"

	"github.com/gin-gonic/gin"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/joho/godotenv"
	swaggerfiles "github.com/swaggo/files"
	ginswagger "github.com/swaggo/gin-swagger"

	_ "github.com/LuizHVicari/webinar-backend/docs"
	"github.com/LuizHVicari/webinar-backend/internal/organization"
	"github.com/LuizHVicari/webinar-backend/internal/user"
	"github.com/LuizHVicari/webinar-backend/pkg/config"
	"github.com/LuizHVicari/webinar-backend/pkg/keto"
	"github.com/LuizHVicari/webinar-backend/pkg/middleware"
	db "github.com/LuizHVicari/webinar-backend/sqlc/generated"
)

// @Summary      Health check
// @Tags         health
// @Produce      json
// @Success      200  {object}  map[string]string
// @Router       /health [get]
func healthHandler(c *gin.Context) {
	c.JSON(http.StatusOK, gin.H{"status": "ok"})
}

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

	router.GET("/health", healthHandler)
	router.GET("/swagger/*any", ginswagger.WrapHandler(swaggerfiles.Handler))

	auth := router.Group("/", middleware.Auth(cfg.KratosPublicURL, userSvc))
	orgHandler.RegisterRoutes(auth)
	userHandler.RegisterRoutes(auth)

	log.Fatal(router.Run(":" + cfg.Port))
}
