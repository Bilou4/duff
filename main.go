package main

import (
	"crypto/md5"
	"encoding/hex"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Data struct {
	sync.RWMutex
	Data []string
}

func WalkDir(dirname string, recursive bool, minSize int64, m *sync.Map, wg *sync.WaitGroup) error {
	defer wg.Done()
	entries, err := os.ReadDir(dirname)
	if err != nil {
		return err
	}
	for _, f := range entries {
		fullpath, err := filepath.Abs(filepath.Join(dirname, f.Name()))
		if err != nil {
			return err
		}
		if f.IsDir() {
			if recursive {
				wg.Add(1)
				go WalkDir(fullpath, recursive, minSize, m, wg)
			}
			continue
		}

		info, err := f.Info()
		if err != nil {
			return err
		}
		size := info.Size()
		if size <= minSize {
			continue
		}
		// https://stackoverflow.com/questions/77568020/concurrent-map-with-a-slice-in-golang
		d := &Data{}
		entry, _ := m.LoadOrStore(size, d)
		dfEntry, _ := entry.(*Data)
		dfEntry.Lock()
		dfEntry.Data = append(dfEntry.Data, fullpath)
		dfEntry.Unlock()
	}
	return nil
}
func computeHash(path string) (string, error) {
	f, err := os.Open(path)
	if err != nil {
		return "", err
	}
	defer f.Close()
	h := md5.New()
	_, err = io.Copy(h, f)
	if err != nil {
		return "", err
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func main() {
	var recursive bool
	var minSize int64
	flag.BoolVar(&recursive, "recursive", false, "Set this flag to true if you want recursiveness.")
	flag.Int64Var(&minSize, "min-size", 1024, "Determine the minimum file size you want to check.")

	flag.Parse()
	if flag.NArg() == 0 {
		fmt.Println("no directory to analyze")
		os.Exit(1)
	}

	begin := time.Now()
	m := new(sync.Map)
	wg := new(sync.WaitGroup)
	listDirs := flag.Args()
	for _, d := range listDirs {
		if _, err := os.Stat(d); errors.Is(err, os.ErrNotExist) {
			fmt.Printf("[-] %q does not exist\n", d)
		}
		wg.Add(1)
		go WalkDir(d, recursive, minSize, m, wg)
	}

	wg.Wait()

	res := make(map[string][]string)
	m.Range(func(key, value any) bool {
		v := value.(*Data)
		if len(v.Data) >= 2 {
			for _, path := range v.Data {
				ch, err := computeHash(path)
				if err != nil {
					return false
				}
				res[ch] = append(res[ch], path)
			}
		}
		return true
	})
	nbIdentic, nbGroups := 0, 0
	for h, listfiles := range res {
		if len(listfiles) >= 2 {
			fmt.Println(h)
			nbGroups++
			nbIdentic += len(listfiles)
			for _, s := range listfiles {
				fmt.Println(s)
			}
			fmt.Println("***********")
		}
	}
	fmt.Printf("[+] number of groups: %d - number of identic files: %d\n", nbGroups, nbIdentic)
	fmt.Println("[+] total exec time", time.Since(begin))

}
