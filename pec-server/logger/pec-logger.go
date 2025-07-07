package logger

import (
	"os"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

var log *zap.Logger

// Init initializes the structured logger with file output
func Init(logFilePath string) error {
	f, err := os.OpenFile(logFilePath, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0644)
	if err != nil {
		return err
	}

	ws := zapcore.AddSync(f)
	encCfg := zap.NewProductionEncoderConfig()
	encCfg.TimeKey = "timestamp"
	encCfg.EncodeTime = zapcore.ISO8601TimeEncoder

	core := zapcore.NewCore(zapcore.NewJSONEncoder(encCfg), ws, zap.InfoLevel)
	log = zap.New(core)
	return nil
}

func Sync() {
	if log != nil {
		log.Sync()
	}
}

// LogAcceptance logs a ricevuta di accettazione
func LogAcceptance(from, to, messageID, path string) {
	log.Info("Ricevuta di accettazione generata",
		zap.String("event", "acceptance"),
		zap.String("from", from),
		zap.String("to", to),
		zap.String("message_id", messageID),
		zap.String("eml_path", path),
	)
}

// LogDelivery logs a ricevuta di consegna
func LogDelivery(from, to, messageID, status string) {
	log.Info("Ricevuta di consegna emessa",
		zap.String("event", "delivery"),
		zap.String("from", from),
		zap.String("to", to),
		zap.String("message_id", messageID),
		zap.String("status", status),
	)
}

// LogMessageReceived logs a new incoming PEC message
func LogMessageReceived(from string, to []string, path string) {
	log.Info("Messaggio PEC ricevuto",
		zap.String("event", "message_received"),
		zap.String("from", from),
		zap.Strings("to", to),
		zap.String("path", path),
	)
}

// LogError logs an operational error
func LogError(message string, err error, context map[string]string) {
	fields := []zap.Field{
		zap.String("event", "error"),
		zap.String("error", err.Error()),
	}

	for k, v := range context {
		fields = append(fields, zap.String(k, v))
	}

	log.Error(message, fields...)
}
