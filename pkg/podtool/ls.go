package podtool

import (
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/LeeEirc/elfinder"
	"github.com/jumpserver/koko/pkg/logger"
)

func (p *PodTool) ListFiles(path, id string) []elfinder.FileDir {
	var files []elfinder.FileDir
	// rootPath := "/tmp"
	cmdir := "/tmp" + path
	commands := []string{"ls", "-l", "--full-time", cmdir}

	res, err := p.ExecCommand(commands)
	if err != nil {
		logger.Error(err)
	}
	result := string(res)
	array := strings.Split(result, "\n")
	i := 0
	if strings.Contains(array[0], "total") {
		i = 1
	}
	for ; i < len(array); i++ {
		line := array[i]
		fArray := strings.Fields(line)
		if len(fArray) >= 9 {
			f := elfinder.FileDir{
				// Size:      fArray[4],
				// Read:  1,
				// Write: 1,
				Mime: "file",
				// Hash:  "aaa",
				// Phash: "ff73093944b703cc573955b0c8f889e7_Lw",
				// Ts: 1656655873,
			}
			size := fArray[4]
			f.Size, _ = strconv.ParseInt(size, 10, 64)
			mode := fArray[0]

			modTime := fArray[5] + " " + fArray[6]
			timeFormat := "2006-01-02 15:04:05"
			mtime, _ := time.ParseInLocation(timeFormat, modTime, time.Local)
			f.Ts = mtime.Unix()
			var name string
			if strings.HasPrefix(mode, "l") && len(fArray) > 10 {
				// getLink(line)
				name = getLinkName(fArray[8], line)
			} else {
				name = getName(fArray[8], line)
			}
			if strings.HasPrefix(mode, "d") {
				f.Mime = "directory"
			}
			modebyte := []byte(mode)
			if string(modebyte[1]) == "r" {
				f.Read = 1
			}
			if string(modebyte[2]) == "w" {
				f.Write = 1
			}
			f.Name = name
			f.Phash = hashPath(id, path)
			f.Hash = hashPath(id, filepath.Join(path, f.Name))
			files = append(files, f)
		}
	}
	return files
}

func (p *PodTool) DirInfo(path, id string) (elfinder.FileDir, error) {
	var files elfinder.FileDir
	var filename string
	// rootPath := "/tmp"
	cmdir := "/tmp" + path
	commands := []string{"ls", "-ld", "--full-time", cmdir}

	res, err := p.ExecCommand(commands)
	if err != nil {
		logger.Error(err)
	}
	result := string(res)
	array := strings.Split(result, "\n")
	i := 0
	if strings.Contains(array[0], "total") {
		i = 1
	}
	for ; i < len(array); i++ {
		line := array[i]
		fArray := strings.Fields(line)
		if len(fArray) >= 9 {
			f := elfinder.FileDir{
				// Size:      fArray[4],
				// Read:  1,
				// Write: 1,
				Mime: "file",
				// Hash:  "aaa",
				// Phash: "ff73093944b703cc573955b0c8f889e7_Lw",
				// Ts: 1656655873,
			}
			size := fArray[4]
			f.Size, _ = strconv.ParseInt(size, 10, 64)
			mode := fArray[0]

			modTime := fArray[5] + " " + fArray[6]
			timeFormat := "2006-01-02 15:04:05"
			mtime, _ := time.ParseInLocation(timeFormat, modTime, time.Local)
			f.Ts = mtime.Unix()
			var name string
			if strings.HasPrefix(mode, "l") && len(fArray) > 10 {
				// getLink(line)
				name = getLinkName(fArray[8], line)
				filename = filepath.Base(name)
			} else {
				name = getName(fArray[8], line)
				filename = filepath.Base(name)
			}
			if strings.HasPrefix(mode, "d") {
				f.Mime = "directory"
			}
			modebyte := []byte(mode)
			if string(modebyte[1]) == "r" {
				f.Read = 1
			}
			if string(modebyte[2]) == "w" {
				f.Write = 1
			}
			f.Name = filename
			dirPath := filepath.Dir(name)
			f.Phash = hashPath(id, dirPath)
			f.Hash = hashPath(id, path)
			files = f
		}
	}
	return files, err
}

func getName(sub string, line string) string {
	return strings.TrimSpace(line[strings.Index(line, sub):])
}

func getLink(line string) string {
	const linkTag = "->"
	in := strings.Index(line, linkTag)
	return strings.TrimSpace(line[in+len(linkTag):])
}

func getLinkName(sub string, line string) string {
	const linkTag = "->"
	linkIn := strings.Index(line, linkTag)
	nameIn := strings.Index(line, sub)
	return strings.TrimSpace(line[nameIn:linkIn])
}

func hashPath(id, path string) string {
	return elfinder.CreateHash(id, path)
}
