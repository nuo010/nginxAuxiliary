package main

import (
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

func main() {
	var files []string
	var dirList []string

	root := "C:\\Users\\Erin\\Desktop\\1\\"
	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		files = append(files, path)
		return nil
	})
	if err != nil {
		return
	}
	for _, file := range files {
		fmt.Println(file)
		if file != root {
			dirName := file[strings.LastIndex(file, "\\")+1:]
			dirList = append(dirList, dirName)
		}
	}
	sort.Strings(dirList)
	fmt.Println(dirList)

}
