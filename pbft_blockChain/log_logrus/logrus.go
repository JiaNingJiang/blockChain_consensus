package loglogrus

import (
	"bytes"
	"fmt"
	"io"
	"os"
	"path"
	"time"

	"github.com/sirupsen/logrus"
)

// 颜色
const (
	red    = 31
	yellow = 33
	blue   = 36
	gray   = 37
)

var (
	defaultLogLevel logrus.Level = logrus.DebugLevel // 默认等级
)

func init() {
	Log = NewLog()
}

var Log *logrus.Logger

type LogFormatter struct{} //需要实现Formatter(entry *logrus.Entry) ([]byte, error)接口

func (t *LogFormatter) Format(entry *logrus.Entry) ([]byte, error) {
	var levelColor int
	switch entry.Level { //根据不同的level去展示颜色
	case logrus.DebugLevel, logrus.TraceLevel:
		levelColor = gray
	case logrus.WarnLevel:
		levelColor = yellow
	case logrus.ErrorLevel, logrus.FatalLevel, logrus.PanicLevel:
		levelColor = red
	default:
		levelColor = blue
	}
	var b *bytes.Buffer
	if entry.Buffer != nil {
		b = entry.Buffer //获取日志实例中的缓冲
	} else {
		b = &bytes.Buffer{}
	}
	//自定义日期格式
	timestamp := entry.Time.Format("2006-01-02 15:04:05") //获取格式化的日志消息时间
	if entry.HasCaller() {                                //检测日志实例是否开启  SetReportCaller(true)
		//自定义文件路径
		funcVal := entry.Caller.Function                                                 //生成日志消息的函数名
		fileVal := fmt.Sprintf("%s:%d", path.Base(entry.Caller.File), entry.Caller.Line) //生成日志的文件和日志行号
		//自定义输出格式
		//fmt.Fprintf(b, "[%s] \x1b[%dm[%s]\x1b[0m %s %s %s\n", timestamp, levelColor, entry.Level, fileVal, funcVal, entry.Message) //entry.Message是日志消息内容
		_ = levelColor
		fmt.Fprintf(b, "[%s] [%s] %s %s %s\n", timestamp, entry.Level, fileVal, funcVal, entry.Message) //entry.Message是日志消息内容
	} else {
		//fmt.Fprintf(b, "[%s] \x1b[%dm[%s]\x1b[0m %s\n", timestamp, levelColor, entry.Level, entry.Message)
		fmt.Fprintf(b, "[%s] [%s] %s\n", timestamp, entry.Level, entry.Message)
	}
	return b.Bytes(), nil //返回格式化的日志缓冲
}

func NewLog() *logrus.Logger {

	mLog := logrus.New() //新建一个实例
	fileDate := time.Now().Format("2006_01_02_15_04")
	rootPath, _ := os.Getwd()
	logPath := rootPath + string(os.PathSeparator)

	commonLogPath := logPath + "log" + string(os.PathSeparator)
	commonLogFile := commonLogPath + fmt.Sprintf("log%s", fileDate)
	file, err := os.OpenFile(commonLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err != nil {
		mLog.Infof("Create LogFile is failed, err:%v", err)
	}

	fileWriter := logFileWriter{file, commonLogPath, commonLogFile, "log", fileDate} //具有自定义的Writer方法

	writers := []io.Writer{
		&fileWriter,
		os.Stdout,
	}
	//  同时写文件和屏幕
	fileAndStdoutWriter := io.MultiWriter(writers...) //io.MultiWriter可以包含多个Writer对象
	mLog.SetOutput(fileAndStdoutWriter)
	mLog.SetReportCaller(true)         //开启返回函数名和行号
	mLog.SetFormatter(&LogFormatter{}) //设置自己定义的Formatter
	mLog.SetLevel(defaultLogLevel)     //设置最低的Level

	// 错误日志单独存储的Hook
	errLogPath := logPath + "errLog" + string(os.PathSeparator)
	errLogFile := errLogPath + fmt.Sprintf("err.log%s", fileDate)
	errFile, _ := os.OpenFile(errLogFile, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	errFileWriter := logFileWriter{errFile, errLogPath, errLogFile, "err.log", fileDate} //具有自定义的Writer方法
	errhook := &WarnHook{Writer: &errFileWriter}
	mLog.AddHook(errhook)

	return mLog

}
