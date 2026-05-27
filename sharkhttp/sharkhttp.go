package sharkhttp

import (
	"fmt"

	"github.com/gin-gonic/gin"
	swaggerfiles "github.com/swaggo/files"
	ginSwagger "github.com/swaggo/gin-swagger"
	"go.uber.org/zap"
)

func New(logger *zap.Logger, port int) *gin.Engine {
	gin.DisableConsoleColor()
	gin.SetMode(gin.ReleaseMode)
	router := gin.New()
	router.Use(recoveryMiddleware(logger))
	router.Use(corsMiddleare())
	router.Use(errorMiddleware())
	go router.Run(":" + fmt.Sprint(port))
	return router
}

func StartSwagger(router *gin.Engine, logger *zap.Logger, port int) {
	router.GET("swagger/*any", ginSwagger.WrapHandler(swaggerfiles.Handler))
	logger.Debug("swagger url: http://127.0.0.1" + ":" + fmt.Sprint(port) + "/swagger/index.html")
}
