package sharklog

import (
	"context"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/lornshark/shark/sharksnowflake"

	"github.com/bytedance/sonic"
	"github.com/segmentio/kafka-go"
	"github.com/spf13/cast"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

func New(ctx context.Context, name string, id string, writer *kafka.Writer) *SharkLog {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString(t.Format("2006-01-02 15:04:05.000"))
	}
	encoderConfig.CallerKey = "caller"
	encoderConfig.EncodeCaller = zapcore.ShortCallerEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	consoleEncoder := zapcore.NewConsoleEncoder(encoderConfig)
	consoleCore := zapcore.NewCore(consoleEncoder, zapcore.AddSync(os.Stdout), zapcore.DebugLevel)
	hostName, _ := os.Hostname()
	snowFlake := sharksnowflake.NewSnowflake()
	lwriter := &logWriter{
		ctx:       ctx,
		name:      name,
		id:        id,
		writer:    writer,
		hostName:  hostName,
		snowFlake: snowFlake,
	}
	redisCore := zapcore.NewCore(zapcore.NewJSONEncoder(encoderConfig), zapcore.AddSync(lwriter), zapcore.DebugLevel)
	core := zapcore.NewTee(consoleCore, redisCore)
	logger := zap.New(core, zap.AddCaller())
	return &SharkLog{
		Zap:    logger,
		writer: lwriter,
	}
}

type SharkLog struct {
	Zap    *zap.Logger
	writer *logWriter
}

type logWriter struct {
	ctx       context.Context
	writer    *kafka.Writer
	snowFlake *sharksnowflake.Snowflake
	hostName  string
	name      string
	id        string
}

func (w *logWriter) Write(p []byte) (n int, err error) {
	if w.writer != nil {
		data := map[string]any{
			"log_id":      cast.ToString(w.snowFlake.Generate()),
			"server_name": fmt.Sprintf("%v-%v", w.name, w.id),
			"server_host": w.hostName,
			"msg":         string(p),
		}
		b, _ := sonic.Marshal(data)
		err := w.writer.WriteMessages(context.Background(), kafka.Message{Value: b})
		if err != nil {
			fmt.Println("日志写入Kafka失败", err, "日志内容", string(p))
		}
		select {
		case <-w.ctx.Done():
			s := string(p)
			if strings.Contains(s, "****************server exit****************") {
				w.writer.Close()
			}
		default:
		}
	}
	return len(p), nil
}
