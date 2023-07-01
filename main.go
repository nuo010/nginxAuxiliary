package main

import (
	"bufio"
	"fmt"
	"github.com/fsnotify/fsnotify"
	"github.com/natefinch/lumberjack"
	"github.com/spf13/viper"
	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
	"io"
	"log"
	"os"
	"path"
	"time"
)

var sugarLogger *zap.SugaredLogger

// PathExists 判断一个文件或文件夹是否存在
// 输入文件路径，根据返回的bool值来判断文件或文件夹是否存在
func PathExists(path string) (bool, error) {
	_, err := os.Stat(path)
	if err == nil {
		return true, nil
	}
	if os.IsNotExist(err) {
		return false, nil
	}
	return false, err
}
func CopyFile(dstFileName string, srcFileName string) (written int64, err error) {
	srcfile, err := os.Open(srcFileName)
	if err != nil {
		fmt.Println("open file error")
		return
	}
	defer srcfile.Close()

	//通过srcfile，获取到reader
	reader := bufio.NewReader(srcfile)

	//打开dstFileName，因为这个文件可能不存在，所以不能使用os.open打开
	dstFile, err := os.OpenFile(dstFileName, os.O_CREATE|os.O_WRONLY, os.ModePerm)
	if err != nil {
		fmt.Println("open fil error")
		return
	}
	defer dstFile.Close()
	//通过dstFile，获取writer
	writer := bufio.NewWriter(dstFile)

	return io.Copy(writer, reader)
}
func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("读取配置文件错误!")
	}

	filePath, err := PathExists(viper.GetString("auxiliary.confPath"))
	if err != nil {
		return
	}
	if !filePath {
		err := os.MkdirAll(viper.GetString("auxiliary.confPath"), 0755)
		if err != nil {
			fmt.Println(err)
		}
		fmt.Println("Nested directory created successfully!")
	}
	go jk()
	//InitLogger()

}

func jk() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		log.Fatal("创建观察者失败: ", err)
	}
	defer func(watcher *fsnotify.Watcher) {
		err := watcher.Close()
		if err != nil {
		}
	}(watcher)

	done := make(chan bool)
	go func() {
		defer close(done)
		for {
			select {
			case event, ok := <-watcher.Events:
				if !ok {
					return
				}
				log.Printf("%s %s\n", event.Name, event.Op)
				_, err := CopyFile(viper.GetString("auxiliary.confPath")+time.Now().Format("20060102150405")+path.Ext(event.Name), event.Name)
				if err != nil {
					fmt.Println("copy文件错误")
					fmt.Println(err)
					return
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					return
				}
				log.Println("error: ", err)
			}
		}
	}()
	err = watcher.Add(viper.GetString("nginx.confPath"))
	if err != nil {
		log.Fatal("add failed:", err)
	}
	<-done
}
func InitLogger() {
	writeSyncer := getLogWriter()
	encoder := getEncoder()
	core := zapcore.NewCore(encoder, writeSyncer, zapcore.DebugLevel)
	logger := zap.New(core, zap.AddCaller())
	sugarLogger = logger.Sugar()
	defer sugarLogger.Sync()
}
func getEncoder() zapcore.Encoder {
	encoderConfig := zap.NewProductionEncoderConfig()
	encoderConfig.EncodeTime = zapcore.ISO8601TimeEncoder
	encoderConfig.EncodeLevel = zapcore.CapitalLevelEncoder
	return zapcore.NewConsoleEncoder(encoderConfig)
}

func getLogWriter() zapcore.WriteSyncer {
	lumberJackLogger := &lumberjack.Logger{
		Filename:   "./test.log",
		MaxSize:    50,
		MaxBackups: 5,
		MaxAge:     30,
		Compress:   false,
	}
	return zapcore.AddSync(lumberJackLogger)
}
