package sharkhttp

import (
	"context"
	"fmt"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

func New(ctx context.Context, evn string, logger *zap.Logger, port int) *gin.Engine {
	gin.DisableConsoleColor()
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(recoveryMiddleware(logger))
	router.Use(corsMiddleare())
	router.Use(errorMiddleware())
	if evn == "dev" {
		router.GET("swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	}
	go router.Run(":" + fmt.Sprint(port))
	return router
}
