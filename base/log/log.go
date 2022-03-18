package log

import (
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type (
	Level   string
	OutType int
)

const (
	LevelDebug Level = "debug"
	LevelInfo  Level = "info"
	LevelWarn  Level = "warn"
	LevelError Level = "error"

	infoFileOutName  = "service.log"
	errorFileOutName = "error.log"
	trackFileOutName = "track.log"

	//ConsoleOut 控制台输出
	ConsoleOut OutType = 1
	//InfoFileOut 一般日志
	InfoFileOut OutType = 2
	//ErrorFileOut 错误日志
	ErrorFileOut OutType = 4
	//TrackFileOut json日志
	TrackFileOut OutType = 8

	// NormalOut 一般输出
	NormalOut = InfoFileOut | ErrorFileOut
	// NormalOutWithTrack 有一般输出，也有json的track
	NormalOutWithTrack = NormalOut | TrackFileOut
)

var (
	//Builder 初始化Logger的builder，由于配置项太多
	Builder = &builder{logger: &loggerProxy{}}

	levelMapping = map[Level]zapcore.Level{
		LevelDebug: zap.DebugLevel,
		LevelInfo:  zap.InfoLevel,
		LevelWarn:  zap.WarnLevel,
		LevelError: zap.ErrorLevel,
	}
	proxy *loggerProxy
	once  sync.Once
)

type loggerProxy struct {
	path         string
	level        Level
	out          OutType
	maxSize      int
	maxAge       int
	maxBackUps   int
	enableRotate bool

	zapLevel zap.AtomicLevel
	logger   atomic.Value
	dLogger  *zap.SugaredLogger
	nLogger  *zap.SugaredLogger
	tracker  *zap.Logger
}

func (lp *loggerProxy) EnableDebug(debug bool) {
	if debug {
		lp.zapLevel.SetLevel(zapcore.DebugLevel)
		lp.logger.Store(lp.dLogger)
	} else {
		lp.zapLevel.SetLevel(zapcore.InfoLevel)
		lp.logger.Store(lp.nLogger)
	}
}

type builder struct {
	logger *loggerProxy
}

//Path 日志文件路径
func (b *builder) Path(path string) *builder {
	b.logger.path = path
	return b
}

func (b *builder) Level(level Level) *builder {
	b.logger.level = level
	return b
}

func (b *builder) OutType(out OutType) *builder {
	b.logger.out = out
	return b
}

func (b *builder) MaxSize(size int) *builder {
	b.logger.maxSize = size
	return b
}

func (b *builder) MaxAge(age int) *builder {
	b.logger.maxAge = age
	return b
}

func (b *builder) MaxBackUps(count int) *builder {
	b.logger.maxBackUps = count
	return b
}

func (b *builder) EnableRotate(enable bool) *builder {
	b.logger.enableRotate = enable
	return b
}

func (b *builder) Build() {
	once.Do(func() {
		p := b.logger
		if p.out == 0 {
			p.out = NormalOut
		}
		if p.out&NormalOutWithTrack > 0 {
			//需要文件
			if p.path == "" {
				p.path = "./logs"
			}
		}
		if p.path != "" {
			if !exists(p.path) && os.Mkdir(p.path, 0755) != nil {
				panic("fail to create log directory")
			}
		}
		if p.maxBackUps == 0 {
			p.maxBackUps = 30
		}
		if p.level == "" {
			p.level = LevelDebug
		}
		p.zapLevel = zap.NewAtomicLevelAt(levelMapping[p.level])
		//json的track日志
		if p.out&TrackFileOut > 0 {
			encoderCfg := zap.NewProductionEncoderConfig()
			//meta数据
			encoderCfg.TimeKey = "@ts"
			encoderCfg.LevelKey = "@level"
			encoderCfg.MessageKey = "@msg"
			encoderCfg.EncodeTime = timeEncoder
			p.tracker = zap.New(
				zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg),
					zapcore.AddSync(b.getWriter(trackFileOutName)),
					zap.DebugLevel))
		}
		//高优先级
		hp := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.WarnLevel
		})
		//所有
		all := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			if p.zapLevel.Enabled(zap.DebugLevel) {
				return true
			}
			return lvl > zap.DebugLevel
		})
		cores := make([]zapcore.Core, 0, 1)
		consoleConfig := zap.NewDevelopmentEncoderConfig()
		encoder := zapcore.NewConsoleEncoder(consoleConfig)
		if p.out&ConsoleOut > 0 {
			cores = append(cores, zapcore.NewCore(
				encoder, zapcore.AddSync(os.Stdout), all))
		}
		if p.out&InfoFileOut > 0 {
			writer := b.getWriter(infoFileOutName)
			cores = append(cores, zapcore.NewCore(
				encoder, zapcore.AddSync(writer), all))
		}
		if p.out&ErrorFileOut > 0 {
			writer := b.getWriter(errorFileOutName)
			cores = append(cores, zapcore.NewCore(
				encoder, zapcore.AddSync(writer), hp))
		}
		core := zapcore.NewTee(cores...)
		lg := zap.New(core)
		p.logger = atomic.Value{}
		p.nLogger = lg.Sugar()
		//debug模式下打印堆栈
		p.dLogger = lg.WithOptions(zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
		p.EnableDebug(p.level == LevelDebug)
		proxy = p
	})
}

// 判断所给路径文件/文件夹是否存在=>避免循环依赖fs
func exists(path string) bool {
	_, err := os.Stat(path) //os.Stat获取文件信息
	if err != nil {
		if os.IsExist(err) {
			return true
		}
		return false
	}
	return true
}

func timeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02T15:04:05.000000"))
}

func (b *builder) getWriter(name string) io.Writer {
	fullName := filepath.Join(b.logger.path, name)
	if !b.logger.enableRotate {
		f, err := os.OpenFile(fullName, os.O_CREATE|os.O_RDWR|os.O_APPEND, 0644)
		if err != nil {
			panic("fail to open log file")
		}
		return f
	}
	return &lumberjack.Logger{
		Filename:   fullName,
		MaxSize:    b.logger.maxSize,
		MaxAge:     b.logger.maxAge,
		MaxBackups: b.logger.maxBackUps,
	}
}

//Debug 会调试模式下打印caller，其他忽略，减少开销
func Debug(format string, a ...interface{}) {
	proxy.logger.Load().(*zap.SugaredLogger).Debugf(format, a...)
}

func Info(format string, a ...interface{}) {
	proxy.logger.Load().(*zap.SugaredLogger).Infof(format, a...)
}

func Warn(format string, a ...interface{}) {
	proxy.logger.Load().(*zap.SugaredLogger).Warnf(format, a...)
}

func Error(format string, a ...interface{}) {
	proxy.logger.Load().(*zap.SugaredLogger).Errorf(format, a...)
}

func Fatal(format string, a ...interface{}) {
	proxy.logger.Load().(*zap.SugaredLogger).Fatalf(format, a...)
}

//JsonInfo 会输出json格式的日志，json日志在单独的文件里
func JsonInfo(msg string, fields ...zap.Field) {
	proxy.tracker.Info(msg, fields...)
}

func JsonWarn(msg string, fields ...zap.Field) {
	proxy.tracker.Warn(msg, fields...)
}

func JsonError(msg string, fields ...zap.Field) {
	proxy.tracker.Error(msg, fields...)
}

func init() {
	lg, _ := zap.NewDevelopment()
	//默认情况下初始化一个仅输出到控制台的日志方便测试
	proxy = &loggerProxy{}
	proxy.nLogger = lg.Sugar()
	proxy.dLogger = lg.WithOptions(zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
	proxy.logger = atomic.Value{}
	proxy.EnableDebug(true)
}
