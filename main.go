package main

import (
	"bufio"
	"crypto/md5"
	"encoding/hex"
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
	"path/filepath"
	"sort"
	"strings"
	"time"
)

var logPath = "logrus.log"
var md5Text string

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

// CopyFile 新的文件, 旧文件
func CopyFile(dstFileName string, srcFileName string) (written int64, err error) {
	_, _ = os.Create(dstFileName)
	file1, _ := os.ReadFile(srcFileName)
	file, err := os.OpenFile(dstFileName, os.O_WRONLY|os.O_CREATE, 0666)
	if err != nil {
		logrus.Error("文件打开失败", err)
		return 0, err
	}
	//及时关闭file句柄
	defer file.Close()
	//写入文件时，使用带缓存的 *Writer
	write := bufio.NewWriter(file)
	_, err = write.WriteString(string(file1))
	if err != nil {
		return 0, err
	}
	//Flush将缓存的文件真正写入到文件中
	err = write.Flush()
	if err != nil {
		return 0, err
	}

	return int64(len(file1)), nil
}

// 返回一个支持至 秒 级别的 cron
func newWithSeconds() *cron.Cron {
	secondParser := cron.NewParser(cron.Second | cron.Minute |
		cron.Hour | cron.Dom | cron.Month | cron.DowOptional | cron.Descriptor)
	return cron.New(cron.WithParser(secondParser), cron.WithChain())
}

// 日志归档
func startCorn() {
	c := newWithSeconds()
	_, err := c.AddFunc("1 1 1 * * ?", func() {
		logrus.Debug("↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓↓")
		rmDir(viper.GetString("auxiliary.logPath") + time.Now().AddDate(0, 0, -viper.GetInt("back.logDay")).Format("20060102"))
		date := time.Now().AddDate(0, 0, -1).Format("20060102")
		logFilePath := viper.GetString("auxiliary.logPath") + date
		err := os.Mkdir(logFilePath, os.ModePerm)
		if err != nil {
			logrus.Error("创建Nginx归档目录失败!")
			return
		} else {
			logrus.Debug("创建Nginx归档目录成功!,", logFilePath)
		}
		accFilePath := viper.GetString("nginx.logPath") + "/access.log"
		accFileInfo, _ := os.Stat(accFilePath)
		err = os.Rename(accFilePath, logFilePath+"/access.log")
		if err != nil {
			fmt.Println(err)
			logrus.Error("归档Nginx access日志错误!")
			return
		}
		logrus.Debug("归档access日志成功!")
		logrus.Debug("归档大小:", accFileInfo.Size()/1048576, "MB")
		errPath := viper.GetString("nginx.logPath") + "/error.log"
		errFileInfo, _ := os.Stat(errPath)
		err = os.Rename(errPath, logFilePath+"/error.log")
		if err != nil {
			fmt.Println(err)
			logrus.Error("归档Nginx error日志错误!")
			return
		}
		logrus.Debug("归档error日志成功!")
		logrus.Debug("归档大小:", errFileInfo.Size()/1048576, "MB")
		// 使用ioutil一次性读取文件
		data, err := os.ReadFile(viper.GetString("nginx.pidPath"))
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
	if err != nil {
		logrus.Error("开启日志归档任务错误!")
		return
	}
	//c.AddFunc("1 1 1 1 * ?", func() {
	//	err := os.Truncate(logPath, 0)
	//	if err != nil {
	//		logrus.Error("清理日志错误!")
	//		return
	//	}
	//	logrus.Error("清理软件运行日志成功!")
	//})
	_, err = c.AddFunc("1/5 * * * * *", func() {
		fileMD5, err := FileMD5(viper.GetString("nginx.confPath"))
		if err == nil {
			if md5Text != fileMD5 {
				logrus.Debug("#################################")
				// 清理备份
				rmConfBack(viper.GetString("auxiliary.confPath"))
				backPath := viper.GetString("auxiliary.confPath") + time.Now().Format("20060102") + "/"
				backFlag, _ := PathExists(backPath)
				if !backFlag {
					err := os.Mkdir(backPath, os.ModePerm)
					if err != nil {
						logrus.Error("创建Conf归档目录失败!", err)
						return
					} else {
						logrus.Debug("创建Conf归档目录成功!,", backPath)
					}
				}
				_, err = CopyFile(backPath+time.Now().Format("150405")+path.Ext(viper.GetString("nginx.confPath")), viper.GetString("nginx.confPath"))
				if err != nil {
					logrus.Debug("备份文件错误!", err)
				} else {
					logrus.Debug("备份文件成功!")
					md5Text = fileMD5
				}
			}
		}
	})
	if err != nil {
		logrus.Error("开启conf备份任务错误!")
		return
	}
	logrus.Debug("开启corn成功!!!")
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
					logrus.Error("备份文件错误!", err)
				} else {
					logrus.Debug("备份文件成功!")
				}
			case err, ok := <-watcher.Errors:
				if !ok {
					logrus.Error("文件监听错误:", err)
					return
				}
			}
		}
	}()
	err = watcher.Add(viper.GetString("nginx.confPath"))
	if err != nil {
		logrus.Error("添加监控失败:", viper.GetString("nginx.confPath"))
	} else {
		logrus.Debug("添加监控成功:", viper.GetString("nginx.confPath"))
	}
	<-done
	logrus.Info("文件监控退出!")
}
func FileMD5(filePath string) (string, error) {
	file, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	hash := md5.New()
	_, _ = io.Copy(hash, file)
	return hex.EncodeToString(hash.Sum(nil)), nil
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
			logrus.Error(err)
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
			logrus.Error(err)
			return
		}
		fmt.Println("创建日志归档文件夹成功!")
		logrus.Debug("创建日志归档文件夹成功!")
	}
}
func rmDir(dirPath string) {
	logrus.Debug("删除文件夹,", dirPath)
	exists, err := PathExists(dirPath)
	if err == nil && exists {
		logrus.Debug("开始删除文件夹,", dirPath)
		dirInfo, err := os.Stat(dirPath)
		if err != nil {
			logrus.Error("读取目录错误:", dirPath)
			return
		}
		if dirInfo.IsDir() {
			err := os.RemoveAll(dirPath)
			if err != nil {
				logrus.Error("删除文件夹错误:", err)
			}
		}
	} else {
		logrus.Error("目录不存在,", dirPath)
	}

}

// 清理conf备份
func rmConfBack(path string) {
	var files []string
	var dirList []string
	err := filepath.Walk(path, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		return
	}
	for _, file := range files {
		if file != path {
			dirName := file[strings.LastIndex(file, "\\")+1:]
			dirList = append(dirList, dirName)
		}
	}
	sort.Strings(dirList)
	num := len(dirList)
	backNum := viper.GetInt("back.confNum")
	for _, k := range dirList {
		if num > backNum {
			logrus.Debug("清理多余备份:", path+k)
			err := os.Remove(path + k)
			if err != nil {
				return
			}
			num--
		}
	}
	return
}

func main() {
	logrus.SetLevel(logrus.DebugLevel)
	// 设置日志输出到什么地方去
	// 将日志输出到标准输出，就是直接在控制台打印出来。
	// 先打开一个日志文件
	file, err := os.OpenFile(logPath, os.O_CREATE|os.O_WRONLY|os.O_APPEND, 0666)
	if err == nil {
		// 设置将日志输出到文件
		logrus.SetOutput(file)
	} else {
		logrus.Info("打开日志文件失败，默认输出到stderr")
	}
	//logrus.SetOutput(os.Stdout)
	// 设置为true则显示日志在代码什么位置打印的
	//log.SetReportCaller(true)

	logrus.SetFormatter(&logrus.TextFormatter{
		FullTimestamp:   true,
		TimestampFormat: "2006-01-02 15:04:05", // 设置时间格式
	})
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")
	err = viper.ReadInConfig()
	if err != nil {
		fmt.Println("读取配置文件错误!")
		logrus.Error("读取配置文件错误!")
		return
	}
	initFile()
	logrus.Info("初始化完成!")
	logrus.Info("软件版本v1.4")
	go startCorn()
	select {}
}
