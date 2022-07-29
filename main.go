package main

import (
	"fmt"
	"io/ioutil"
	"log"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"time"
	"unicode"

	"hotjk/unzip2here/unzip"

	"github.com/gosuri/uilive"
	"github.com/mozillazg/go-pinyin"
)

var sema = make(chan struct{}, runtime.NumCPU())
var done = make(chan string)
var wg sync.WaitGroup
var pinyinStyle = pinyin.NewArgs()
var status = make(map[string]int)
var abort = make(chan struct{})
var writer = uilive.New()

func init() {
	writer.Start()
}

func zhCharToPinyin(p string) (s string) {
	for _, r := range p {
		if unicode.Is(unicode.Han, r) {
			s += string(strings.Title(pinyin.Pinyin(string(r), pinyinStyle)[0][0]))
		} else {
			s += string(r)
		}
	}
	return
}

func unzipFile(fullname, dir string) {
	defer wg.Done()
	defer func() { done <- dir }()
	sema <- struct{}{}
	defer func() { <-sema }()

	if err := unzip.UnzipSource(fullname, dir); err != nil {
		//log.Println(err, " - ", fullname)
	} else {
		os.Remove(fullname)
	}
}

func unzipFolder(dir string) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err, " - ", dir)
	}

	for _, file := range files {
		fullname := filepath.Join(dir, file.Name())
		//fmt.Println(fullname)
		if file.IsDir() {
			unzipFolder(fullname)
		} else {
			wg.Add(1)
			go unzipFile(fullname, dir)
		}
	}
}

func renameFolder(dir string) {
	files, err := ioutil.ReadDir(dir)
	if err != nil {
		log.Fatal(err)
	}

	for _, file := range files {
		fullname := filepath.Join(dir, file.Name())
		//fmt.Println(fullname)
		if file.IsDir() {
			renameFolder(fullname)
		} else {
			newName := zhCharToPinyin(file.Name())
			//fmt.Println(newName)
			if err := os.Rename(fullname, filepath.Join(dir, newName)); err != nil {
				log.Fatal(err, " - ", fullname)
			}
		}
	}
}

func printStatus() {
	for k, v := range status {
		fmt.Fprintf(writer, "[%d]\t%s\n", v, k)
	}
}

func main() {
	if len(os.Args) != 2 {
		log.Println("unzip2here folder")
		return
	}

	path := os.Args[1]
	fmt.Println("unzip2here", path)

	start := time.Now()
	unzipFolder(path)

	go func() {
		for dir := range done {
			status[dir]++
		}
	}()

	ticker := time.NewTicker(1 * time.Second)
	go func() {
		for {
			select {
			case <-ticker.C:
				printStatus()
			case <-abort:
				printStatus()
				return
			}
		}
	}()

	wg.Wait()
	close(done)
	ticker.Stop()
	abort <- struct{}{}
	writer.Stop()

	elapsed := time.Since(start)
	log.Printf("unzip took %s", elapsed)

	start = time.Now()
	renameFolder(path)
	elapsed = time.Since(start)
	log.Printf("rename took %s", elapsed)
}
