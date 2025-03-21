package httpd

import (
	"fmt"
	"io"
	"net/url"
	"os"
	pathv1 "path"
	"path/filepath"
	"strings"
	"sync"
	"time"

	"github.com/LeeEirc/elfinder"
	"github.com/pkg/sftp"
	"k8s.io/client-go/kubernetes"

	"github.com/jumpserver/koko/pkg/jms-sdk-go/common"
	"github.com/jumpserver/koko/pkg/jms-sdk-go/model"
	"github.com/jumpserver/koko/pkg/jms-sdk-go/service"
	"github.com/jumpserver/koko/pkg/logger"
	"github.com/jumpserver/koko/pkg/podtool"
	"github.com/jumpserver/koko/pkg/srvconn"
)

func NewUserVolume(jmsService *service.JMService, user *model.User, addr, hostId string) *UserVolume {
	var userSftp *srvconn.UserSftpConn
	var containerOptions *srvconn.ContainerOptions
	var isPod bool
	homename := "Home"
	basePath := "/"
	switch {
	case hostId == "":
		isPod = false
		userSftp = srvconn.NewUserSftpConn(jmsService, user, addr)
	case strings.Contains(hostId, "namespace"):
		logger.Debug("NewUserVolume hostId is:", hostId)
		isPod = true
		u, err := url.Parse("?" + hostId)
		if err != nil {
			logger.Errorf("url parse failed: %s", err)
		}
		uuid := u.Query().Get("app_id")
		podname := u.Query().Get("pod")
		namespace := u.Query().Get("namespace")
		container := u.Query().Get("container")
		systemUserId := u.Query().Get("system_user_id")
		systemUserAuthInfo, err := jmsService.GetUserApplicationAuthInfo(systemUserId, uuid, user.ID, user.Name)
		if err != nil {
			logger.Errorf("Get systemuser auth info failed: %s", err)
		}
		application, err := jmsService.GetApplicationById(uuid)
		if err != nil {
			logger.Errorf("Get application failed: %s", err)
		}
		homename = podname
		basePath = filepath.Join("/", homename)
		containerOptions = &srvconn.ContainerOptions{
			Host:          application.Attrs.Cluster,
			Token:         systemUserAuthInfo.Token,
			SystemUser:    systemUserAuthInfo.Name,
			PodName:       podname,
			Namespace:     namespace,
			ContainerName: container,
			IsSkipTls:     true,
		}

		userSftp = srvconn.NewUserContainerWithPod(jmsService, user, addr, containerOptions)
	default:
		isPod = false
		assets, err := jmsService.GetUserAssetByID(user.ID, hostId)
		if err != nil {
			logger.Errorf("Get user asset failed: %s", err)
		}
		if len(assets) == 1 {
			folderName := assets[0].Hostname
			if strings.Contains(folderName, "/") {
				folderName = strings.ReplaceAll(folderName, "/", "_")
			}
			homename = folderName
			basePath = filepath.Join("/", homename)
		}
		userSftp = srvconn.NewUserSftpConnWithAssets(jmsService, user, addr, assets...)
	}
	rawID := fmt.Sprintf("%s@%s", user.Username, addr)
	uVolume := &UserVolume{
		Uuid:          elfinder.GenerateID(rawID),
		UserSftp:      userSftp,
		Homename:      homename,
		basePath:      basePath,
		chunkFilesMap: make(map[int]*sftp.File),
		lock:          new(sync.Mutex),
		IsPod:         isPod,
		PodConn:       containerOptions,
	}
	return uVolume
}

type UserVolume struct {
	Uuid     string
	UserSftp *srvconn.UserSftpConn
	Homename string
	basePath string

	chunkFilesMap map[int]*sftp.File
	lock          *sync.Mutex
	IsPod         bool
	PodConn       *srvconn.ContainerOptions
}

func (u *UserVolume) IsPods() bool {
	return u.IsPod
}

func (u *UserVolume) ID() string {
	return u.Uuid
}

func (u *UserVolume) PodInfo(path string) (elfinder.FileDir, error) {
	var dirs elfinder.FileDir
	if path == "/" {
		return u.RootFileDir(), nil
	}
	pt, err := GetPodTool(u.PodConn)
	if err != nil {
		logger.Errorf("err is %s", err)
	}
	dirs, err = pt.DirInfo(path, u.Uuid)
	return dirs, err

}

func (u *UserVolume) Info(path string) (elfinder.FileDir, error) {
	logger.Debug("Volume Info: ", path)
	var rest elfinder.FileDir
	if path == "/" {
		return u.RootFileDir(), nil
	}
	originFileInfo, err := u.UserSftp.Stat(filepath.Join(u.basePath, path))
	if err != nil {
		return rest, err
	}
	dirPath := filepath.Dir(path)
	filename := filepath.Base(path)
	rest.Read, rest.Write = elfinder.ReadWritePem(originFileInfo.Mode())
	if filename != originFileInfo.Name() {
		rest.Read, rest.Write = 1, 1
		logger.Debug("Info filename no equal")
	}
	if filename == "." {
		filename = originFileInfo.Name()
	}
	rest.Name = filename
	rest.Hash = hashPath(u.Uuid, path)
	rest.Phash = hashPath(u.Uuid, dirPath)
	if rest.Hash == rest.Phash {
		rest.Phash = ""
	}
	rest.Size = originFileInfo.Size()
	rest.Ts = originFileInfo.ModTime().Unix()
	rest.Volumeid = u.Uuid
	if originFileInfo.IsDir() {
		rest.Mime = "directory"
		rest.Dirs = 1
	} else {
		rest.Mime = "file"
		rest.Dirs = 0
	}
	return rest, err
}

func GetPodTool(podconn *srvconn.ContainerOptions) (podtool.PodTool, error) {
	var pt podtool.PodTool
	k8sCfg := podconn.K8sCfg()
	clientset, err := kubernetes.NewForConfig(k8sCfg)
	if err != nil {
		return pt, err
	}
	pt = podtool.PodTool{
		Namespace:     podconn.Namespace,
		PodName:       podconn.PodName,
		ContainerName: podconn.ContainerName,
		K8sClient:     clientset,
		RestClient:    k8sCfg,
	}

	return pt, nil
}

func (u *UserVolume) PodFileList(path string) []elfinder.FileDir {
	dirs := make([]elfinder.FileDir, 0)
	pt, err := GetPodTool(u.PodConn)
	if err != nil {
		logger.Errorf("err is %s", err)
	}
	dirs = pt.ListFiles(path, u.Uuid)
	return dirs
}

func (u *UserVolume) List(path string) []elfinder.FileDir {
	dirs := make([]elfinder.FileDir, 0)
	logger.Debug("Volume List: ", path)
	originFileInfolist, err := u.UserSftp.ReadDir(filepath.Join(u.basePath, path))
	if err != nil {
		return dirs
	}
	for i := 0; i < len(originFileInfolist); i++ {
		if originFileInfolist[i].Mode()&os.ModeSymlink != 0 {
			linkInfo := NewElfinderFileInfo(u.Uuid, path, originFileInfolist[i])
			_, err := u.UserSftp.ReadDir(filepath.Join(u.basePath, path, originFileInfolist[i].Name()))
			if err != nil {
				logger.Errorf("link file %s is not dir err: %s", originFileInfolist[i].Name(), err)
			} else {
				logger.Infof("link file %s is dir", originFileInfolist[i].Name())
				linkInfo.Mime = "directory"
				linkInfo.Dirs = 1
			}
			dirs = append(dirs, linkInfo)
			continue
		}

		dirs = append(dirs, NewElfinderFileInfo(u.Uuid, path, originFileInfolist[i]))
	}
	return dirs
}

func (u *UserVolume) Parents(path string, dep int) []elfinder.FileDir {
	logger.Debug("volume Parents: ", path)
	dirs := make([]elfinder.FileDir, 0)
	dirPath := path
	for {
		tmps, err := u.UserSftp.ReadDir(filepath.Join(u.basePath, dirPath))
		if err != nil {
			return dirs
		}

		for i := 0; i < len(tmps); i++ {
			dirs = append(dirs, NewElfinderFileInfo(u.Uuid, dirPath, tmps[i]))
		}

		if dirPath == "/" {
			break
		}
		dirPath = filepath.Dir(dirPath)
	}
	return dirs
}

func (u *UserVolume) PodGetFile(path string) (reader io.ReadCloser, err error) {
	pt, err := GetPodTool(u.PodConn)
	if err != nil {
		logger.Errorf("err is %s", err)
	}
	var fileP string
	fileNameWithSuffix := pathv1.Base(path)
	fileType := pathv1.Ext(fileNameWithSuffix)
	fileName := strings.TrimSuffix(fileNameWithSuffix, fileType)
	logger.Debug("fileNameWithSuffix, fileType, fileName, path 分别是:", fileNameWithSuffix, fileType, fileName, path)
	fileP = filepath.Join(os.TempDir(), fmt.Sprintf("%d", time.Now().UnixNano()))
	err = os.MkdirAll(fileP, os.ModePerm)
	if err != nil {
		logger.Errorf("err is %s", err)
	}
	fileP = filepath.Join(fileP, fileName)
	// fileP = filepath.Join(fileP, fileName+".tar")
	err = pt.CopyFromPod(path, fileP)
	if err != nil {
		logger.Errorf("err is %s", err)
	}
	reader, err = os.Open(fileP)
	operate := model.OperateDownload
	if err != nil {
		isSuccess := false
		data := model.FTPLog{
			User:       u.UserSftp.User.Username,
			Hostname:   u.Homename,
			OrgID:      "00000000-0000-0000-0000-000000000002",
			SystemUser: u.PodConn.SystemUser,
			RemoteAddr: u.UserSftp.Addr,
			Operate:    operate,
			Path:       "/tmp/" + fileNameWithSuffix,
			DateStart:  common.NewNowUTCTime(),
			IsSuccess:  isSuccess,
		}
		u.UserSftp.LogChan <- &data
		return nil, err
	}
	os.RemoveAll(fileP)
	isSuccess := true
	data := model.FTPLog{
		User:       u.UserSftp.User.Username,
		Hostname:   u.Homename,
		OrgID:      "00000000-0000-0000-0000-000000000002",
		SystemUser: u.PodConn.SystemUser,
		RemoteAddr: u.UserSftp.Addr,
		Operate:    operate,
		Path:       "/tmp/" + fileNameWithSuffix,
		DateStart:  common.NewNowUTCTime(),
		IsSuccess:  isSuccess,
	}
	u.UserSftp.LogChan <- &data
	return
}

func (u *UserVolume) GetFile(path string) (reader io.ReadCloser, err error) {
	logger.Debug("GetFile path: ", path)
	sftpFile, err := u.UserSftp.Open(filepath.Join(u.basePath, TrimPrefix(path)))
	if err != nil {
		return nil, err
	}
	// 屏蔽 sftp*File 的 WriteTo 方法，防止调用 sftp stat 命令
	return &fileReader{sftpFile}, nil
}

func (u *UserVolume) PodUploadFile(dirPath, uploadPath, filename string, reader io.Reader) (elfinder.FileDir, error) {
	var path string
	switch {
	case strings.Contains(uploadPath, filename):
		path = filepath.Join(dirPath, TrimPrefix(uploadPath))
	case uploadPath != "":
		path = filepath.Join(dirPath, TrimPrefix(uploadPath), filename)
	default:
		path = filepath.Join(dirPath, filename)

	}
	logger.Debug("PodUploadFile upload file path: ", path, " ", filename, " ", uploadPath)
	var rest elfinder.FileDir

	pt, err := GetPodTool(u.PodConn)
	pt.ExecConfig.Stdin = reader
	err = pt.CopyToContainer(filename)

	operate := model.OperateUpload
	if err != nil {
		isSuccess := false
		data := model.FTPLog{
			User:       u.UserSftp.User.Username,
			Hostname:   u.Homename,
			OrgID:      "00000000-0000-0000-0000-000000000002",
			SystemUser: u.PodConn.SystemUser,
			RemoteAddr: u.UserSftp.Addr,
			Operate:    operate,
			Path:       "/tmp/" + filename,
			DateStart:  common.NewNowUTCTime(),
			IsSuccess:  isSuccess,
		}
		u.UserSftp.LogChan <- &data
		return rest, err
	}
	isSuccess := true
	data := model.FTPLog{
		User:       u.UserSftp.User.Username,
		Hostname:   u.Homename,
		OrgID:      "00000000-0000-0000-0000-000000000002",
		SystemUser: u.PodConn.SystemUser,
		RemoteAddr: u.UserSftp.Addr,
		Operate:    operate,
		Path:       "/tmp/" + filename,
		DateStart:  common.NewNowUTCTime(),
		IsSuccess:  isSuccess,
	}
	logger.Debug("Debug sftpvolume.go 317 FTP 日志", data)
	u.UserSftp.LogChan <- &data
	return u.PodInfo(path)
}

func (u *UserVolume) UploadFile(dirPath, uploadPath, filename string, reader io.Reader) (elfinder.FileDir, error) {
	var path string
	switch {
	case strings.Contains(uploadPath, filename):
		path = filepath.Join(dirPath, TrimPrefix(uploadPath))
	case uploadPath != "":
		path = filepath.Join(dirPath, TrimPrefix(uploadPath), filename)
	default:
		path = filepath.Join(dirPath, filename)

	}
	logger.Debug("Volume upload file path: ", path, " ", filename, " ", uploadPath)
	var rest elfinder.FileDir
	fd, err := u.UserSftp.Create(filepath.Join(u.basePath, path))
	if err != nil {
		return rest, err
	}
	defer fd.Close()

	_, err = io.Copy(fd, reader)
	if err != nil {
		return rest, err
	}
	return u.Info(path)
}

func (u *UserVolume) UploadChunk(cid int, dirPath, uploadPath, filename string, rangeData elfinder.ChunkRange, reader io.Reader) error {
	var err error
	var path string
	u.lock.Lock()
	fd, ok := u.chunkFilesMap[cid]
	u.lock.Unlock()
	if !ok {
		switch {
		case strings.Contains(uploadPath, filename):
			path = filepath.Join(dirPath, TrimPrefix(uploadPath))
		case uploadPath != "":
			path = filepath.Join(dirPath, TrimPrefix(uploadPath), filename)
		default:
			path = filepath.Join(dirPath, filename)

		}
		fd, err = u.UserSftp.Create(filepath.Join(u.basePath, path))
		if err != nil {
			return err
		}
		_, err = fd.Seek(rangeData.Offset, 0)
		if err != nil {
			return err
		}
		u.lock.Lock()
		u.chunkFilesMap[cid] = fd
		u.lock.Unlock()
	}
	_, err = io.Copy(fd, reader)
	if err != nil {
		_ = fd.Close()
		u.lock.Lock()
		delete(u.chunkFilesMap, cid)
		u.lock.Unlock()
	}
	return err
}

func (u *UserVolume) MergeChunk(cid, total int, dirPath, uploadPath, filename string) (elfinder.FileDir, error) {
	var path string
	switch {
	case strings.Contains(uploadPath, filename):
		path = filepath.Join(dirPath, TrimPrefix(uploadPath))
	case uploadPath != "":
		path = filepath.Join(dirPath, TrimPrefix(uploadPath), filename)
	default:
		path = filepath.Join(dirPath, filename)

	}
	logger.Debug("Merge chunk path: ", path)
	u.lock.Lock()
	if fd, ok := u.chunkFilesMap[cid]; ok {
		_ = fd.Close()
		delete(u.chunkFilesMap, cid)
	}
	u.lock.Unlock()
	return u.Info(path)
}

func (u *UserVolume) MakeDir(dir, newDirname string) (elfinder.FileDir, error) {
	logger.Debug("Volume Make Dir: ", newDirname)
	path := filepath.Join(dir, TrimPrefix(newDirname))
	var rest elfinder.FileDir
	err := u.UserSftp.MkdirAll(filepath.Join(u.basePath, path))
	if err != nil {
		return rest, err
	}
	return u.Info(path)
}

func (u *UserVolume) MakeFile(dir, newFilename string) (elfinder.FileDir, error) {
	logger.Debug("Volume MakeFile")

	path := filepath.Join(dir, newFilename)
	var rest elfinder.FileDir
	fd, err := u.UserSftp.Create(filepath.Join(u.basePath, path))
	if err != nil {
		return rest, err
	}
	_ = fd.Close()
	res, err := u.UserSftp.Stat(filepath.Join(u.basePath, path))

	return NewElfinderFileInfo(u.Uuid, dir, res), err
}

func (u *UserVolume) Rename(oldNamePath, newName string) (elfinder.FileDir, error) {

	logger.Debug("Volume Rename")
	var rest elfinder.FileDir
	newNamePath := filepath.Join(filepath.Dir(oldNamePath), newName)
	err := u.UserSftp.Rename(filepath.Join(u.basePath, oldNamePath), filepath.Join(u.basePath, newNamePath))
	if err != nil {
		return rest, err
	}
	return u.Info(newNamePath)
}

func (u *UserVolume) Remove(path string) error {

	logger.Debug("Volume remove", path)
	var res os.FileInfo
	var err error
	res, err = u.UserSftp.Stat(filepath.Join(u.basePath, path))
	if err != nil {
		return err
	}
	if res.IsDir() {
		return u.UserSftp.RemoveDirectory(filepath.Join(u.basePath, path))
	}
	return u.UserSftp.Remove(filepath.Join(u.basePath, path))
}

func (u *UserVolume) Paste(dir, filename, suffix string, reader io.ReadCloser) (elfinder.FileDir, error) {
	defer reader.Close()
	var rest elfinder.FileDir
	path := filepath.Join(dir, filename)
	_, err := u.UserSftp.Stat(filepath.Join(u.basePath, path))
	if err == nil {
		path += suffix
	}
	fd, err := u.UserSftp.Create(filepath.Join(u.basePath, path))
	logger.Debug("volume paste: ", path, err)
	if err != nil {
		return rest, err
	}
	defer fd.Close()
	_, err = io.Copy(fd, reader)
	if err != nil {
		return rest, err
	}
	return u.Info(path)
}

func (u *UserVolume) RootFileDir() elfinder.FileDir {
	logger.Debug("Root File Dir")
	var (
		size int64
	)
	tz := time.Now().UnixNano()
	readPem := byte(1)
	writePem := byte(0)
	if fInfo, err := u.UserSftp.Stat(u.basePath); err == nil {
		size = fInfo.Size()
		tz = fInfo.ModTime().Unix()
		readPem, writePem = elfinder.ReadWritePem(fInfo.Mode())
	}
	var rest elfinder.FileDir
	rest.Name = u.Homename
	rest.Hash = hashPath(u.Uuid, "/")
	rest.Size = size
	rest.Volumeid = u.Uuid
	rest.Mime = "directory"
	rest.Dirs = 1
	rest.Read, rest.Write = readPem, writePem
	rest.Locked = 1
	rest.Ts = tz
	return rest
}

func (u *UserVolume) Close() {
	u.UserSftp.Close()
	logger.Infof("User %s's volume close", u.UserSftp.User.Name)
}

func (u *UserVolume) Search(path, key string, mimes ...string) (res []elfinder.FileDir, err error) {
	originFileInfolist, err := u.UserSftp.Search(key)
	if err != nil {
		return nil, err
	}
	res = make([]elfinder.FileDir, 0, len(originFileInfolist))
	searchPath := fmt.Sprintf("/%s", srvconn.SearchFolderName)
	for i := 0; i < len(originFileInfolist); i++ {
		res = append(res, NewElfinderFileInfo(u.Uuid, searchPath, originFileInfolist[i]))

	}
	return
}

func NewElfinderFileInfo(id, dirPath string, originFileInfo os.FileInfo) elfinder.FileDir {
	var rest elfinder.FileDir
	rest.Name = originFileInfo.Name()
	rest.Hash = hashPath(id, filepath.Join(dirPath, originFileInfo.Name()))
	rest.Phash = hashPath(id, dirPath)
	if rest.Hash == rest.Phash {
		rest.Phash = ""
	}
	rest.Size = originFileInfo.Size()
	rest.Volumeid = id
	if originFileInfo.IsDir() {
		rest.Mime = "directory"
		rest.Dirs = 1
	} else {
		rest.Mime = "file"
		rest.Dirs = 0
	}
	rest.Ts = originFileInfo.ModTime().Unix()
	rest.Read, rest.Write = elfinder.ReadWritePem(originFileInfo.Mode())
	return rest
}

func hashPath(id, path string) string {
	return elfinder.CreateHash(id, path)
}

func TrimPrefix(path string) string {
	return strings.TrimPrefix(path, "/")
}

var (
	_ io.ReadCloser = (*fileReader)(nil)
)

type fileReader struct {
	read io.ReadCloser
}

func (f *fileReader) Read(p []byte) (nr int, err error) {
	return f.read.Read(p)
}

func (f *fileReader) Close() error {
	return f.read.Close()
}
