package sharkhttp

import (
	"bytes"
	"errors"
	"io"
	"net/http"
	"runtime/debug"

	"github.com/lornshark/shark/sharkerror"

	"github.com/gin-gonic/gin"
	"github.com/gorilla/websocket"
	"go.uber.org/zap"
)

var wsUpgrader = websocket.Upgrader{
	ReadBufferSize:  1024,
	WriteBufferSize: 1024,
	CheckOrigin: func(r *http.Request) bool {
		return true
	},
}

func corsMiddleare() gin.HandlerFunc {
	return func(ctx *gin.Context) {
		method := ctx.Request.Method
		ctx.Header("Access-Control-Allow-Origin", "*")
		ctx.Header("Access-Control-Allow-Headers", "Content-Type, x-token, Content-Length, X-Requested-With")
		ctx.Header("Access-Control-Allow-Methods", "GET,POST")
		ctx.Header("Access-Control-Expose-Headers", "Content-Length, Access-Control-Allow-Origin, Access-Control-Allow-Headers, Content-Type")
		ctx.Header("Access-Control-Max-Age", "7200")
		if method == "OPTIONS" {
			ctx.AbortWithStatus(http.StatusNoContent)
		}
		ctx.Next()
	}
}

func recoveryMiddleware(logger *zap.Logger) gin.HandlerFunc {
	return func(ctx *gin.Context) {
		reqPath := ctx.Request.URL.Path
		reqData, _ := ctx.GetRawData()
		ctx.Request.Body = io.NopCloser(bytes.NewBuffer(reqData))
		defer func() {
			if r := recover(); r != nil {
				if logger != nil {
					logger.Error("panic", zap.Any("error", r), zap.String("stack", string(debug.Stack())), zap.String("path", reqPath), zap.ByteString("data", reqData))
				}
				ctx.JSON(http.StatusInternalServerError, map[string]any{"data": r})
				ctx.Abort()
			}
		}()
		ctx.Next()
	}
}

func errorMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		c.Next()
		if len(c.Errors) == 0 {
			return
		}
		var err error = c.Errors.Last().Err
		var e *sharkerror.Error
		if errors.As(err, &e) {
			c.JSON(http.StatusOK, e)
			return
		}
		c.JSON(http.StatusOK, &sharkerror.Error{
			Code: 1,
			Msg:  "未知错误",
			Data: err.Error(),
		})
	}
}
