package main

import (
	"fmt"
	_ "github.com/codyguo/godaemon"
	"github.com/fsnotify/fsnotify"
	"github.com/robfig/cron/v3"
	"github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"io"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"
)

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
	_, _ = os.Create(dstFileName)
	file1, err1 := os.Open(dstFileName)
	if err1 != nil {
		fmt.Println(err1)
	}
	// 创建目标文件
	file2, err2 := os.OpenFile(srcFileName, os.O_RDWR|os.O_CREATE, os.ModePerm)
	if err2 != nil {
		fmt.Println(err2)
	}
	//使用结束关闭文件
	defer file1.Close()
	defer file2.Close()
	n, e := io.Copy(file2, file1)
	if e != nil {
		fmt.Println(e)
	} else {
		fmt.Println("拷贝成功。。。，拷贝字节数：", n)
	}

	return n, nil
}

// 返回一个支持至 秒 级别的 cron
func newWithSeconds() *cron.Cron {
	secondParser := cron.NewParser(cron.Second | cron.Minute |
		cron.Hour | cron.Dom | cron.Month | cron.DowOptional | cron.Descriptor)
	return cron.New(cron.WithParser(secondParser), cron.WithChain())
}

// 日志归档
func logC() {
	logrus.Info("开启日志归档!")
	c := newWithSeconds()
	spec := "1 1 1 * * ?"
	c.AddFunc(spec, func() {
		date := time.Now().AddDate(0, 0, -1).Format("20060102")
		err := os.Rename(viper.GetString("nginx.logPath")+"/access.log", viper.GetString("auxiliary.logPath")+date+"_access.log")
		if err != nil {
			fmt.Println(err)
			logrus.Error("归档Nginx access日志错误!")
			return
		}
		logrus.Debug("归档access日志成功!")
		err = os.Rename(viper.GetString("nginx.logPath")+"/error.log", viper.GetString("auxiliary.logPath")+date+"_error.log")
		if err != nil {
			fmt.Println(err)
			logrus.Error("归档Nginx error日志错误!")
			return
		}
		logrus.Debug("归档error日志成功!")
		// 使用ioutil一次性读取文件
		data, err := os.ReadFile("/usr/local/nginx/logs/nginx.pid")
		if err != nil {
			fmt.Println("read file err:", err.Error())
			logrus.Error("读取nginx pid 错误")
			return
		}

		// 打印文件内容
		err = exec.Command("bash", "-c", "kill -USR1 "+strings.Replace(string(data), "\n", "", 1)).Run()
		if err != nil {
			logrus.Error("调用nginx打印日志命令错误!")
			fmt.Println(err)
			return
		}
		logrus.Debug("重置Nginx日志成功!")
	})
	c.Start()
}

func jk() {
	//问题：当使用vi或vim编辑被监视的文件（如config.conf）时，我希望它会触发Write Event。但是，实际上，它会触发重命名，从而导致原始文件无效。
	// vim实际上创建了一个临时文件，删除现有文件，并在保存时用临时文件替换它。
	logrus.Info("开启文件监控!")
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		logrus.Debug("创建观察者失败: ", err)
	}
	defer func(watcher *fsnotify.Watcher) {
		err := watcher.Close()
		if err != nil {
			logrus.Error("观察者退出失败:", err)
			return
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
				logrus.Debug("发现文件变动:", event.Name, "变动类型", event.Op)
				_, err := CopyFile(viper.GetString("auxiliary.confPath")+time.Now().Format("20060102150405")+path.Ext(event.Name), event.Name)
				if err != nil {
					fmt.Println("copy文件错误")
					fmt.Println(err)
					logrus.Error("备份文件错误!", err)
					return
				}
				logrus.Debug("备份文件成功!")
			case err, ok := <-watcher.Errors:
				if !ok {
					logrus.Error("文件监听错误:", err)
					return
				}
				logrus.Error("error: ", err)
			}
		}
	}()
	err = watcher.Add(viper.GetString("nginx.confPath"))
	if err != nil {
		fmt.Println("add failed:", err)
	}
	logrus.Debug("添加监控成功:", viper.GetString("nginx.confPath"))
	<-done
	logrus.Info("文件监控退出!")
}
func jk2() {

}
func initFile() {
	filePath, err := PathExists(viper.GetString("auxiliary.confPath"))
	if err != nil {
		return
	}
	if !filePath {
		err := os.MkdirAll(viper.GetString("auxiliary.confPath"), 0755)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("创建配置文件备份文件夹成功!")
		logrus.Debug("创建配置文件备份文件夹成功!")
	}
	filePath, err = PathExists(viper.GetString("auxiliary.logPath"))
	if err != nil {
		return
	}
	if !filePath {
		err := os.MkdirAll(viper.GetString("auxiliary.logPath"), 0755)
		if err != nil {
			fmt.Println(err)
			return
		}
		fmt.Println("创建日志归档文件夹成功!")
		logrus.Debug("创建日志归档文件夹成功!")
	}
}
func main() {
	logrus.SetLevel(logrus.DebugLevel)
	// 设置日志输出到什么地方去
	// 将日志输出到标准输出，就是直接在控制台打印出来。
	// 先打开一个日志文件
	file, err := os.OpenFile("logrus.log", os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		// 设置将日志输出到文件
		logrus.SetOutput(file)
	} else {
		logrus.Info("打开日志文件失败，默认输出到stderr")
	}
	//logrus.SetOutput(os.Stdout)
	// 设置为true则显示日志在代码什么位置打印的
	//log.SetReportCaller(true)

	// 设置日志以json格式输出， 如果不设置默认以text格式输出
	logrus.SetFormatter(&logrus.JSONFormatter{})
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")
	err = viper.ReadInConfig()
	if err != nil {
		fmt.Println("读取配置文件错误!")
		logrus.Error("读取配置文件错误!")
		return
	}
	//initFile()
	logrus.Info("初始化完成!")
	go jk()
	//go logC()
	logrus.Info("开启监控成功!")
	logrus.Error("开启监控成功!")
	select {}
}
