package main

import (
	"bufio"
	"fmt"
	_ "github.com/codyguo/godaemon"
	"github.com/fsnotify/fsnotify"
	"github.com/robfig/cron/v3"
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

// 返回一个支持至 秒 级别的 cron
func newWithSeconds() *cron.Cron {
	secondParser := cron.NewParser(cron.Second | cron.Minute |
		cron.Hour | cron.Dom | cron.Month | cron.DowOptional | cron.Descriptor)
	return cron.New(cron.WithParser(secondParser), cron.WithChain())
}

// 日志归档
func logC() {
	c := newWithSeconds()
	spec := "1 1 1 * * ?"
	c.AddFunc(spec, func() {
		date := time.Now().AddDate(0, 0, -1).Format("20060102")
		err := os.Rename(viper.GetString("nginx.logPath")+"/access.log", viper.GetString("auxiliary.logPath")+date+"_access.log")
		if err != nil {
			fmt.Println(err)
			return
		}
		err = os.Rename(viper.GetString("nginx.logPath")+"/error.log", viper.GetString("auxiliary.logPath")+date+"_error.log")
		if err != nil {
			fmt.Println(err)
			return
		}
		// 使用ioutil一次性读取文件
		data, err := os.ReadFile("/usr/local/nginx/logs/nginx.pid")
		if err != nil {
			fmt.Println("read file err:", err.Error())
			return
		}

		// 打印文件内容
		fmt.Println(string(data))
		err = exec.Command("bash", "-c", "kill -USR1 "+strings.Replace(string(data), "\n", "", 1)).Run()
		if err != nil {
			fmt.Println(err)
			return
		}
	})
	c.Start()
}

func jk() {
	watcher, err := fsnotify.NewWatcher()
	if err != nil {
		fmt.Println("创建观察者失败: ", err)
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
				fmt.Println("%s %s\n", event.Name, event.Op)
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
				fmt.Println("error: ", err)
			}
		}
	}()
	err = watcher.Add(viper.GetString("nginx.confPath"))
	if err != nil {
		fmt.Println("add failed:", err)
	}
	<-done
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
	}
}
func main() {
	viper.SetConfigName("config")
	viper.SetConfigType("yaml")
	viper.AddConfigPath("./")
	err := viper.ReadInConfig()
	if err != nil {
		fmt.Println("读取配置文件错误!")
		return
	}
	initFile()

	go jk()
	go logC()
	select {}
}
