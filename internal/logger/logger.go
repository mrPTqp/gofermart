package logger

import "go.uber.org/zap"

func NewSugarLogger() *zap.SugaredLogger{
	var sugar *zap.SugaredLogger
	logger, err := zap.NewDevelopment()
	if err != nil {
		panic(err)
	}
	defer logger.Sync()
	sugar = logger.Sugar()

	return sugar
}