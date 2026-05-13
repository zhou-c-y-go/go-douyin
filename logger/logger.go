package logger

import (
	"Go_Project/global"
	"fmt"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"os"
	"time"
)

const (
	logTmFmtWithMS = "2006-01-02 15:04:05.000"
)

var (
	Now = time.Now()
)

func getEncoder() zapcore.Encoder {
	customTimeEncoder := func(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
		enc.AppendString("[" + t.Format(logTmFmtWithMS) + "]")
	}
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = customTimeEncoder
	//encoderConfig.EncodeLevel = zapcore.CapitalColorLevelEncoder
	return zapcore.NewJSONEncoder(encoderConfig)
}
func getInfoWriterSyncer() zapcore.WriteSyncer {
	lumberWriteSyncer := &lumberjack.Logger{
		Filename:   fmt.Sprintf("./logger/info(%s %d).log", Now.Month(), Now.Day()), //分割日志
		MaxSize:    2 << 4,                                                          // megabytes
		MaxBackups: 100,
		MaxAge:     28,    // days
		Compress:   false, //Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
		LocalTime:  true,
	}
	return zapcore.AddSync(lumberWriteSyncer)
}

func getErrorWriterSyncer() zapcore.WriteSyncer {
	lumberWriteSyncer := &lumberjack.Logger{
		Filename:   fmt.Sprintf("./logger/Error(%s %d).log", Now.Month(), Now.Day()),
		MaxSize:    2 << 4, // megabytes
		MaxBackups: 100,
		MaxAge:     28,    // days
		Compress:   false, //Compress确定是否应该使用gzip压缩已旋转的日志文件。默认值是不执行压缩。
		LocalTime:  true,
	}
	return zapcore.AddSync(lumberWriteSyncer)
}

func Init() {
	encoder := getEncoder()
	highPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev >= zap.ErrorLevel
	})
	lowPriority := zap.LevelEnablerFunc(func(lev zapcore.Level) bool {
		return lev < zap.ErrorLevel && lev >= zap.DebugLevel
	})
	infoFileWriterSyncer := getInfoWriterSyncer()
	errorFileWriterSyncer := getErrorWriterSyncer()
	infoFileCode := zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(infoFileWriterSyncer, zapcore.AddSync(os.Stdout)), lowPriority)
	errorFileCode := zapcore.NewCore(encoder, zapcore.NewMultiWriteSyncer(errorFileWriterSyncer, zapcore.AddSync(os.Stdout)), highPriority)

	var coreArr []zapcore.Core
	coreArr = append(coreArr, infoFileCode)
	coreArr = append(coreArr, errorFileCode)
	global.Logger = zap.New(zapcore.NewTee(coreArr...), zap.AddCaller(), zap.AddCallerSkip(1))
	fmt.Println("日志生成成功")
	global.SugaredLogger = global.Logger.Sugar()
}
