package log

import (
	"github.com/samber/lo"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"gopkg.in/natefinch/lumberjack.v2"
	"io"
	"os"
	"path/filepath"
	"reflect"
	"runtime"
	"strings"
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

	infoFileOutName  = "service"
	errorFileOutName = "error"
	trackFileOutName = "track"
	panicFileOutName = "panic"

	// ConsoleOut 控制台输出
	ConsoleOut OutType = 1
	// InfoFileOut 一般日志
	InfoFileOut OutType = 2
	// ErrorFileOut 错误日志
	ErrorFileOut OutType = 4
	// TrackFileOut json日志
	TrackFileOut OutType = 8

	// NormalOut 一般输出
	NormalOut = InfoFileOut | ErrorFileOut
	// NormalOutWithTrack 有一般输出，也有json的track
	NormalOutWithTrack = NormalOut | TrackFileOut
)

var (
	// Builder 初始化Logger的builder，由于配置项太多
	Builder      = &builder{logger: &loggerProxy{}}
	levelMapping = map[Level]zapcore.Level{
		LevelDebug: zap.DebugLevel,
		LevelInfo:  zap.InfoLevel,
		LevelWarn:  zap.WarnLevel,
		LevelError: zap.ErrorLevel,
	}
	aliasMap = map[string]OutType{
		"console": ConsoleOut,
		"file":    NormalOut,
		"track":   TrackFileOut,
	}
	proxy *loggerProxy
	once  sync.Once
	// cbs的读写锁
	debugLock sync.RWMutex
	// debug开关切换的callback
	cbs = make(map[reflect.Value]func(bool))
)

func OutTypeAlias(name string) OutType {
	//为了方便记忆，可以使用文本配置，用|分割
	names := strings.Split(strings.ToLower(name), "|")
	var r OutType
	for _, s := range names {
		r |= aliasMap[strings.TrimSpace(s)]
	}
	return lo.Ternary(r == 0, ConsoleOut, r)
}

type config struct {
	name         string
	path         string
	level        Level
	out          OutType
	maxSize      int //单位Mb，默认100
	maxAge       int //单位天，默认无限
	maxBackUps   int //最大保留旧日志个数，默认无限
	enableRotate bool
}

func (c *config) Name() string {
	return c.name
}

func (c *config) Path() string {
	return c.path
}

func (c *config) Level() Level {
	return c.level
}

func (c *config) Out() OutType {
	return c.out
}

func (c *config) MaxSize() int {
	return c.maxSize
}

func (c *config) MaxAge() int {
	return c.maxAge
}

func (c *config) MaxBackUps() int {
	return c.maxBackUps
}

func (c *config) EnableRotate() bool {
	return c.enableRotate
}

type loggerProxy struct {
	config
	zapLevel zap.AtomicLevel
	logger   atomic.Value
	dLogger  *zap.SugaredLogger
	nLogger  *zap.SugaredLogger
	tracker  *zap.Logger
}

// RegisterDebugSwitchCallback 注册debug开关切换的回调
// 注意某些组件并不能在运行时修改debug模式
func RegisterDebugSwitchCallback(cb func(debug bool)) {
	debugLock.Lock()
	cbs[reflect.ValueOf(cb)] = cb
	debugLock.Unlock()
	cb(IsDebugEnabled())
}

// UnRegisterDebugSwitchCallback 移除debug开关的回调
// 会将cb重置为debug为false的状态
func UnRegisterDebugSwitchCallback(cb func(debug bool)) {
	cb(false)
	debugLock.Lock()
	delete(cbs, reflect.ValueOf(cb))
	debugLock.Unlock()
}

// ChangeLogLevel 切换debug状态
func (lp *loggerProxy) ChangeLogLevel(level Level, force bool) {
	if !force && levelMapping[level] == lp.zapLevel.Level() {
		return
	}
	debug := level == LevelDebug
	if debug {
		lp.zapLevel.SetLevel(zapcore.DebugLevel)
		lp.logger.Store(lp.dLogger)
	} else {
		lp.zapLevel.SetLevel(levelMapping[level])
		lp.logger.Store(lp.nLogger)
	}
	debugLock.RLock()
	for _, cb := range cbs {
		cb(debug)
	}
	debugLock.RUnlock()
}

func ChangeLogLevel(level Level) {
	proxy.ChangeLogLevel(level, false)
}

// IsDebugEnabled 是否打开了debug
func IsDebugEnabled() bool {
	return proxy.zapLevel.Enabled(zapcore.DebugLevel)
}

type builder struct {
	logger *loggerProxy
}

func (b *builder) Name(name string) *builder {
	b.logger.name = name
	return b
}

// Path 日志文件路径
func (b *builder) Path(path string) *builder {
	b.logger.path = path
	return b
}

func (b *builder) Level(level Level) *builder {
	b.logger.level = level
	return b
}

func (b *builder) OutType(out OutType) *builder {
	if out <= 0 {
		out = NormalOutWithTrack
	}
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

func getTrackEncodeConf() zapcore.EncoderConfig {
	encoderCfg := zap.NewProductionEncoderConfig()
	// meta数据，按ECS规定的格式来
	encoderCfg.TimeKey = "@timestamp"
	encoderCfg.LevelKey = "log.level"
	encoderCfg.MessageKey = "message"
	encoderCfg.EncodeTime = timeEncoder
	return encoderCfg
}

func (b *builder) Build() {
	once.Do(func() {
		p := b.logger
		if p.out == 0 {
			p.out = NormalOut
		}
		if p.out&NormalOutWithTrack > 0 {
			// 需要文件
			if p.path == "" {
				p.path = "./log"
			}
		}
		if p.path != "" {
			if !exists(p.path) && os.MkdirAll(p.path, 0755) != nil {
				panic("fail to create log directory")
			}
		}
		if p.out&NormalOutWithTrack > 0 {
			// 将panic日志重定向到文件，不然的话都会打到stderr里
			filename := lo.Ternary(b.logger.Name() == "", panicFileOutName,
				b.logger.Name()+"-"+panicFileOutName) + ".log"
			if err := redirectStderr(filepath.Join(p.path, filename)); err != nil {
				panic("fail to redirect panic log to file:" + err.Error())
			}
		}
		if p.level == "" {
			p.level = LevelDebug
		}
		p.zapLevel = zap.NewAtomicLevelAt(levelMapping[p.level])
		encoderCfg := getTrackEncodeConf()
		// json的track日志
		if p.out&TrackFileOut > 0 {
			filename := lo.Ternary(b.logger.Name() == "", trackFileOutName,
				b.logger.Name()+"-"+trackFileOutName) + ".log"
			p.tracker = zap.New(
				zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg),
					zapcore.AddSync(b.getWriter(filename)),
					zap.DebugLevel))
		} else {
			//为了防止没有创建track文件，但是使用了track函数做调试造成panic
			p.tracker = zap.New(
				zapcore.NewCore(zapcore.NewJSONEncoder(encoderCfg),
					zapcore.AddSync(os.Stdout),
					zap.DebugLevel))
		}
		// 高优先级
		hp := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
			return lvl >= zapcore.WarnLevel
		})
		// 所有
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
			filename := lo.Ternary(b.logger.Name() == "", infoFileOutName, b.logger.Name()) + ".log"
			writer := b.getWriter(filename)
			cores = append(cores, zapcore.NewCore(
				encoder, zapcore.AddSync(writer), all))
		}
		if p.out&ErrorFileOut > 0 {
			filename := lo.Ternary(b.logger.Name() == "", errorFileOutName,
				b.logger.Name()+"-"+errorFileOutName) + ".log"
			writer := b.getWriter(filename)
			cores = append(cores, zapcore.NewCore(
				encoder, zapcore.AddSync(writer), hp))
		}
		core := zapcore.NewTee(cores...)
		lg := zap.New(core)
		p.logger = atomic.Value{}
		p.nLogger = lg.Sugar()
		// debug模式下打印堆栈
		p.dLogger = lg.WithOptions(zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
		p.ChangeLogLevel(p.level, true)
		proxy = p
	})
}

// 判断所给路径文件/文件夹是否存在=>避免循环依赖fs
func exists(path string) bool {
	_, err := os.Stat(path) // os.Stat获取文件信息
	if err != nil {
		return os.IsExist(err)
	}
	return true
}

func timeEncoder(t time.Time, enc zapcore.PrimitiveArrayEncoder) {
	enc.AppendString(t.Format("2006-01-02T15:04:05.000Z"))
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

// Debug 会调试模式下打印caller，其他忽略，减少开销
func Debug(format string, a ...any) {
	proxy.logger.Load().(*zap.SugaredLogger).Debugf(format, a...)
}

func Info(format string, a ...any) {
	proxy.logger.Load().(*zap.SugaredLogger).Infof(format, a...)
}

func Warn(format string, a ...any) {
	proxy.logger.Load().(*zap.SugaredLogger).Warnf(format, a...)
}

func Error(format string, a ...any) {
	proxy.logger.Load().(*zap.SugaredLogger).Errorf(format, a...)
}

func Fatal(format string, a ...any) {
	proxy.logger.Load().(*zap.SugaredLogger).Fatalf(format, a...)
}

// JsonWith 设置默认的field，如模块名称等
func JsonWith(fields ...zap.Field) *zap.Logger {
	return proxy.tracker.With(fields...)
}

// JsonDebug Json格式的debug
func JsonDebug(msg string, fields ...zap.Field) {
	proxy.tracker.Debug(msg, fields...)
}

// JsonInfo 会输出json格式的日志，json日志在单独的文件里
func JsonInfo(msg string, fields ...zap.Field) {
	proxy.tracker.Info(msg, fields...)
}

func JsonWarn(msg string, fields ...zap.Field) {
	proxy.tracker.Warn(msg, fields...)
}

func JsonError(msg string, fields ...zap.Field) {
	proxy.tracker.Error(msg, fields...)
}

// PanicStack 从panic中恢复并打印日志
// 注意recover必须在当前函数调用
func PanicStack(prefix string, r any) {
	buf := make([]byte, 1024)
	l := runtime.Stack(buf, false)
	Error("%s: %v-> %s", prefix, r, buf[:l])
}

func Flush() {
	if proxy.dLogger != nil {
		_ = proxy.dLogger.Sync()
	}
	if proxy.nLogger != nil {
		_ = proxy.nLogger.Sync()
	}
	if proxy.tracker != nil {
		_ = proxy.tracker.Sync()
	}
}

func init() {
	consoleConfig := zap.NewDevelopmentEncoderConfig()
	encoder := zapcore.NewConsoleEncoder(consoleConfig)
	// 默认情况下初始化一个仅输出到控制台的日志方便测试
	proxy = &loggerProxy{}
	proxy.zapLevel = zap.NewAtomicLevelAt(zapcore.DebugLevel)
	all := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		if proxy.zapLevel.Enabled(zap.DebugLevel) {
			return true
		}
		return lvl > zap.DebugLevel
	})
	core := zapcore.NewCore(
		encoder, zapcore.AddSync(os.Stdout), all)
	lg := zap.New(core)
	proxy.nLogger = lg.Sugar()
	proxy.dLogger = lg.WithOptions(zap.AddCaller(), zap.AddCallerSkip(1)).Sugar()
	// json
	proxy.tracker = zap.New(
		zapcore.NewCore(zapcore.NewJSONEncoder(getTrackEncodeConf()),
			zapcore.AddSync(os.Stdout),
			zap.DebugLevel))
	proxy.logger = atomic.Value{}
	proxy.logger.Store(proxy.dLogger)
}
