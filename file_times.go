package main

import (
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/direnv/direnv/gzenv"
)

type FileTime struct {
	Path    string
	Modtime int64
	Exists  bool
}

type FileTimes struct {
	list *[]FileTime
}

func NewFileTimes() (times FileTimes) {
	list := make([]FileTime, 0)
	times.list = &list
	return
}

func (times *FileTimes) stat(path string) (modtime int64, exists bool, err error) {
	stat, err := os.Stat(path)
	if err == nil {
		return stat.ModTime().Unix(), true, nil
	}
	return 0, !os.IsNotExist(err), err
}

func (times *FileTimes) lstat(path string) (modtime int64, exists bool, err error) {
	stat, err := os.Lstat(path)
	if err == nil {
		return stat.ModTime().Unix(), true, nil
	}
	return 0, !os.IsNotExist(err), err
}

func (times *FileTimes) Update(path string) error {
	modtime, exists, err := times.stat(path)
	lmodtime, lexists, lerr := times.lstat(path)

	if err == nil && lerr == nil {
		if lmodtime > modtime {
			return times.NewTime(path, lmodtime, lexists)
		}
		return times.NewTime(path, modtime, exists)
	}

	return times.NewTime(path, 0, exists)
}

func (times *FileTimes) NewTime(path string, modtime int64, exists bool) (err error) {
	var time *FileTime

	path, err = filepath.Abs(path)
	if err != nil {
		return
	}

	path = filepath.Clean(path)

	for _, newTime := range *(times.list) {
		if newTime.Path == path {
			time = &newTime
			break
		}
	}
	if time == nil {
		newTimes := append(*times.list, FileTime{Path: path})
		times.list = &newTimes
		time = &((*times.list)[len(*times.list)-1])
	}

	time.Modtime = modtime
	time.Exists = exists

	return
}

type checkFailed string

func (err checkFailed) Error() string {
	return string(err)
}

func (times *FileTimes) Check() (err error) {
	if len(*times.list) == 0 {
		return checkFailed("Times list is empty")
	}
	for idx := range *times.list {
		err = (*times.list)[idx].Check()
		if err != nil {
			return
		}
	}
	return
}

func (times *FileTimes) CheckOne(path string) (err error) {
	path, err = filepath.Abs(path)
	if err != nil {
		return
	}
	for idx := range *times.list {
		if time := (*times.list)[idx]; time.Path == path {
			err = time.Check()
			return
		}
	}
	return checkFailed(fmt.Sprintf("File %q is unknown", path))
}

func (time FileTime) Check() (err error) {
	stat, err := os.Stat(time.Path)
	lstat, lerr := os.Lstat(time.Path)

	switch {
	case os.IsNotExist(lerr):
		if time.Exists {
			log_debug("Lstat Check: %s: gone", time.Path)
			return checkFailed(fmt.Sprintf("File %q is missing (Lstat)", time.Path))
		}
	case os.IsNotExist(err):
		if time.Exists {
			log_debug("Stat Check: %s: gone", time.Path)
			return checkFailed(fmt.Sprintf("File %q is missing (Stat)", time.Path))
		}
	case lerr != nil:
		log_debug("Lstat Check: %s: ERR: %v", time.Path, lerr)
		return lerr
	case err != nil:
		log_debug("Stat Check: %s: ERR: %v", time.Path, err)
		return err
	case !time.Exists:
		log_debug("Check: %s: appeared", time.Path)
		return checkFailed(fmt.Sprintf("File %q newly created", time.Path))
	case stat.ModTime().Unix() != time.Modtime && lstat.ModTime().Unix() != time.Modtime:
		log_debug("Check: %s: stale (stat: %v, lstat: %v, lastcheck: %v)",
			time.Path, stat.ModTime().Unix(), lstat.ModTime().Unix(),
			time.Modtime)
		return checkFailed(fmt.Sprintf("File %q has changed", time.Path))
	}
	log_debug("Check: %s: up to date", time.Path)
	return nil
}

func (self *FileTime) Formatted(relDir string) string {
	timeBytes, err := time.Unix(self.Modtime, 0).MarshalText()
	if err != nil {
		timeBytes = []byte("<<???>>")
	}
	path, err := filepath.Rel(relDir, self.Path)
	if err != nil {
		path = self.Path
	}
	return fmt.Sprintf("%q - %s", path, timeBytes)
}

func (times *FileTimes) Marshal() string {
	return gzenv.Marshal(*times.list)
}

func (times *FileTimes) Unmarshal(from string) error {
	return gzenv.Unmarshal(from, times.list)
}
