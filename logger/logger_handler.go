package logger

import (
	"bytes"
	"context"
	"fmt"
	"log"
	"log/slog"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"sync"
)

var (
	JSON_FORMAT = 0
	TEXT_FORMAT = 1
)

var (
	reset       = []byte("\033[0m") // 复位
	red         = []byte("\033[31m")
	green       = []byte("\033[32m")
	yellow      = []byte("\033[33m")
	blue        = []byte("\033[34m")
	magenta     = []byte("\033[35m")
	cyan        = []byte("\033[36m")
	gray        = []byte("\033[37m")
	white       = []byte("\033[97m")
	BlueBold    = []byte("\033[34;1m")
	MagentaBold = []byte("\033[35;1m")
	RedBold     = []byte("\033[31;1m")
	YellowBold  = []byte("\033[33;1m")
)

func appendInt(buf []byte, i int) []byte {
	var b [20]byte
	pos := len(b)
	for i >= 10 {
		pos--
		b[pos] = byte(i%10) + '0'
		i /= 10
	}
	pos--
	b[pos] = byte(i) + '0'
	return append(buf, b[pos:]...)
}

type MyModifyHandler struct {
	Mu            *sync.RWMutex
	Stdout        *os.File
	LogFile       *os.File
	LogFileErr    *os.File
	warnErrBase   slog.Handler
	debugInfoBase slog.Handler
	base          slog.Handler // 底层：官方 JSONHandler
	stdoutBase    slog.Handler
}

type logFileKind int

const (
	logFileKindMain logFileKind = iota
	logFileKindErr
	logFileKindStdout
)

type lockedLogWriter struct {
	logObj *FILE
	file   *os.File
	kind   logFileKind
	mu     *sync.RWMutex
}

func (w *lockedLogWriter) Write(p []byte) (int, error) {
	file := w.file
	if w.mu != nil {
		w.mu.Lock()
		defer w.mu.Unlock()
	}
	if w.logObj != nil {
		switch w.kind {
		case logFileKindErr:
			file = w.logObj.logfile_err
		case logFileKindMain:
			file = w.logObj.logfile
		}
	}
	if file == nil {
		return len(p), nil
	}
	return file.Write(p)
}

func NewMyModifyHandler(stdout, logFile, logFileErr *os.File) *MyModifyHandler {
	return newMyModifyHandler(stdout, nil, logFile, logFileErr)
}

func jsonReplaceAttr(groups []string, attr slog.Attr) slog.Attr {
	attr = DesensitizeReplaceAttr(groups, attr)
	if attr.Key == slog.TimeKey && attr.Value.Kind() == slog.KindTime {
		attr.Key = "log_time"
	}
	return attr
}

func newMyModifyHandler(stdout *os.File, logObj *FILE, logFile, logFileErr *os.File) *MyModifyHandler {
	level := slog.LevelDebug
	if Gfilelog != nil && Gfilelog.Logconf != nil {
		level = Gfilelog.getSlogLevel()
	}
	return newMyModifyHandlerWithLevel(stdout, logObj, logFile, logFileErr, level)
}

func newMyModifyHandlerWithLevel(stdout *os.File, logObj *FILE, logFile, logFileErr *os.File, level slog.Level) *MyModifyHandler {
	mu := new(sync.RWMutex)
	if logObj != nil {
		mu = logObj.mu
	}
	debugInfoWriter := &lockedLogWriter{logObj: logObj, file: logFile, kind: logFileKindMain, mu: mu}
	warnErrWriter := &lockedLogWriter{logObj: logObj, file: logFileErr, kind: logFileKindErr, mu: mu}
	stdoutWriter := &lockedLogWriter{file: stdout, kind: logFileKindStdout, mu: mu}
	handler := &MyModifyHandler{
		Mu:            mu,
		Stdout:        stdout,
		LogFile:       logFile,
		LogFileErr:    logFileErr,
		warnErrBase:   slog.NewJSONHandler(warnErrWriter, &slog.HandlerOptions{Level: level, ReplaceAttr: jsonReplaceAttr}),
		debugInfoBase: slog.NewJSONHandler(debugInfoWriter, &slog.HandlerOptions{Level: level, ReplaceAttr: jsonReplaceAttr}),
		stdoutBase:    slog.NewJSONHandler(stdoutWriter, &slog.HandlerOptions{Level: level, ReplaceAttr: jsonReplaceAttr}),
	}
	handler.base = handler.debugInfoBase
	return handler
}

// 核心：在这里修改日志内容
func (h *MyModifyHandler) Handle(ctx context.Context, r slog.Record) error {
	// ✅ 获取 短文件名 + 行号
	_, file, line, _ := runtime.Caller(4)
	shortFile := filepath.Base(file)
	source := fmt.Sprintf("%s:%d", shortFile, line)
	// ✅ 往日志里追加/修改字段
	r.AddAttrs(
		slog.String("source", source),
	)

	for _, k := range Gfilelog.LogTags {
		if m := ctx.Value(k); m != nil {
			if value, ok := m.(string); ok {
				r.AddAttrs(slog.String(k, value))
			}
		} else {
			r.AddAttrs(slog.String(k, "-"))
		}
	}
	var err error
	if Gfilelog.Logconf.Stdout {
		//处理彩色
		if Gfilelog.Logconf.Colorful {
			var buf bytes.Buffer
			base := slog.NewJSONHandler(&buf, &slog.HandlerOptions{Level: slog.LevelDebug, ReplaceAttr: jsonReplaceAttr})
			if err := base.Handle(ctx, r); err != nil {
				return err
			}
			data := buf.Bytes()
			tbuf := make([]byte, 0, 1024)
			switch r.Level {
			case slog.LevelWarn:
				tbuf = append(tbuf, yellow...)
			case slog.LevelDebug:
				tbuf = append(tbuf, green...)
			case slog.LevelInfo:
				tbuf = append(tbuf, blue...)
			case slog.LevelError:
				tbuf = append(tbuf, red...)
			}
			tbuf = append(tbuf, data...)
			if Gfilelog.Logconf.Colorful {
				tbuf = append(tbuf, reset...)
			}
			h.Mu.Lock()
			_, err = h.Stdout.Write(tbuf)
			h.Mu.Unlock()
		} else {
			err = h.stdoutBase.Handle(ctx, r)
		}
	} else {
		if r.Level < slog.LevelWarn {
			err = h.debugInfoBase.Handle(ctx, r)
		} else {
			err = h.debugInfoBase.Handle(ctx, r)
			err = h.warnErrBase.Handle(ctx, r)
		}
	}
	return err
}

// 以下三个是固定写法，不用管
func (h *MyModifyHandler) Enabled(ctx context.Context, l slog.Level) bool {
	return h.base.Enabled(ctx, l)
}

func (h *MyModifyHandler) WithAttrs(attrs []slog.Attr) slog.Handler {
	return &MyModifyHandler{base: h.base.WithAttrs(attrs)}
}

func (h *MyModifyHandler) WithGroup(name string) slog.Handler {
	return &MyModifyHandler{base: h.base.WithGroup(name)}
}

// 自定义 Handler
type CustomHandler struct {
	Mu         *sync.RWMutex
	Stdout     *os.File
	LogObj     *FILE
	LogFile    *os.File
	LogFileErr *os.File
	Flags      int
	RegExp     *regexp.Regexp
	Format     int
}

type CustomHandlerNum struct {
	CustomHandler
}

func NewCustomLogger(stdout, logFile, logFileErr *os.File, flags int) *slog.Logger {
	return newCustomLogger(stdout, nil, logFile, logFileErr, nil, flags)
}

func newCustomLogger(stdout *os.File, logObj *FILE, logFile, logFileErr *os.File, mu *sync.RWMutex, flags int) *slog.Logger {
	if mu == nil && logObj != nil {
		mu = logObj.mu
	}
	if mu == nil {
		mu = new(sync.RWMutex)
	}
	return slog.New(&CustomHandler{
		Stdout:     stdout,
		LogObj:     logObj,
		LogFile:    logFile,
		LogFileErr: logFileErr,
		Mu:         mu,
		Flags:      flags,
		RegExp:     regexp.MustCompile(`^[\w\-\.]+\.go:\d+`),
	})
}

func (h *CustomHandler) IsFileLinePrefix(s string) bool {
	return h.RegExp.MatchString(s)
}

func (h *CustomHandler) Enabled(_ context.Context, lvl slog.Level) bool {
	if Gfilelog == nil || Gfilelog.Logconf == nil {
		return true
	}
	return lvl >= Gfilelog.getSlogLevel()
}

func (h *CustomHandler) WithAttrs([]slog.Attr) slog.Handler {
	return h
}

func (h *CustomHandler) WithGroup(string) slog.Handler {
	return h
}

func (h *CustomHandler) GetAttr(r slog.Record, key string) (slog.Value, bool) {
	var foundVal slog.Value
	var found bool

	r.Attrs(func(attr slog.Attr) bool {
		if attr.Key == key {
			foundVal = attr.Value
			found = true
			return false // 找到就停止遍历，性能更好
		}
		return true
	})

	return foundVal, found
}

func (h *CustomHandler) appendTextAttr(buf []byte, attr slog.Attr) []byte {
	attr = DesensitizeReplaceAttr(nil, attr)
	attr.Value = attr.Value.Resolve()
	buf = append(buf, ' ')
	buf = append(buf, attr.Key...)
	buf = append(buf, '=')
	buf = append(buf, fmt.Sprint(attr.Value.Any())...)
	return buf
}

func (h *CustomHandler) appendLevel(buf []byte, level slog.Level) []byte {
	switch {
	case level >= slog.LevelError:
		buf = append(buf, "[ERROR]"...)
	case level >= slog.LevelWarn:
		buf = append(buf, "[WARN]"...)
	case level >= slog.LevelInfo:
		buf = append(buf, "[INFO]"...)
	default:
		buf = append(buf, "[DEBUG]"...)
	}
	return buf
}

func (h *CustomHandler) appendContextValues(buf []byte, ctx context.Context) []byte {
	for i, k := range Gfilelog.LogTags {
		if i > 0 {
			buf = append(buf, ' ')
		}
		if m := ctx.Value(k); m != nil {
			if value, ok := m.(string); ok {
				buf = append(buf, value...)
				continue
			}
		}
		buf = append(buf, '-')
	}
	return buf
}

func (h *CustomHandler) Handle(ctx context.Context, r slog.Record) error {
	buf := make([]byte, 0, 1024)
	if Gfilelog.Logconf.Colorful {
		switch r.Level {
		case slog.LevelWarn:
			buf = append(buf, yellow...)
		case slog.LevelDebug:
			buf = append(buf, green...)
		case slog.LevelInfo:
			buf = append(buf, blue...)
		case slog.LevelError:
			buf = append(buf, red...)
		}
	}
	// 日期
	if h.Flags&log.Ldate != 0 {
		buf = r.Time.AppendFormat(buf, "2006-01-02")
		buf = append(buf, ' ')
	}
	// 时间+微秒
	if h.Flags&log.Lmicroseconds != 0 {
		buf = r.Time.AppendFormat(buf, "15:04:05.000000")
		buf = append(buf, ' ')
	}

	msgStr := r.Message
	//fmt.Println(msgStr)
	//fmt.Println(h.IsFileLinePrefix(msgStr[0:]))
	// 文件:行号
	if h.Flags&log.Lshortfile != 0 && !h.IsFileLinePrefix(msgStr[0:]) {
		var pcs [1]uintptr
		// skip=4 是 slog + 封装函数标准跳过层数（99% 情况直接用）
		runtime.Callers(5, pcs[:])

		frames := runtime.CallersFrames(pcs[:])
		f, _ := frames.Next()

		buf = append(buf, filepath.Base(f.File)...)
		buf = append(buf, ':')
		buf = appendInt(buf, f.Line)
		buf = append(buf, ' ')
	}

	buf = h.appendContextValues(buf, ctx)
	buf = append(buf, ' ')
	buf = h.appendLevel(buf, r.Level)
	buf = append(buf, ' ')
	buf = append(buf, msgStr...)
	r.Attrs(func(attr slog.Attr) bool {
		buf = h.appendTextAttr(buf, attr)
		return true
	})
	buf = append(buf, '\n')

	if Gfilelog.Logconf.Colorful {
		buf = append(buf, reset...)
	}
	var err error
	//h.Mu.Lock()
	//defer h.Mu.Unlock()
	if Gfilelog.Logconf.Stdout {
		h.Mu.Lock()
		_, err = h.Stdout.Write(buf)
		h.Mu.Unlock()
	} else {
		err = h.writeFiles(r.Level, buf)
	}
	return err
}

func (h *CustomHandler) writeFiles(level slog.Level, buf []byte) error {
	h.Mu.Lock()
	defer h.Mu.Unlock()

	logFile := h.LogFile
	logFileErr := h.LogFileErr
	if h.LogObj != nil {
		logFile = h.LogObj.logfile
		logFileErr = h.LogObj.logfile_err
	}
	if logFile == nil {
		return nil
	}
	if _, err := logFile.Write(buf); err != nil {
		return err
	}
	if level >= slog.LevelWarn && logFileErr != nil {
		_, err := logFileErr.Write(buf)
		return err
	}
	return nil
}
