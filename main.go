package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
	"strconv"
	"syscall"
)

type ResponseBody struct {
	Status    string         `json:"status"`
	Type      string         `json:"type"`
	Error     *ErrorData     `json:"error,omitempty"`
	File      *FileData      `json:"file,omitempty"`
	Directory *DirectoryData `json:"directory,omitempty"`
}

const ResponseTypeFile = "file"
const ResponseTypeDirectory = "directory"
const ResponseTypeDeleted = "deleted"
const ResponseTypeError = "error"

func (x ResponseBody) Code() int {
	switch {
	case x.Type != ResponseTypeError:
		return http.StatusOK
	case x.Error != nil:
		return x.Error.Code
	default:
		return http.StatusInternalServerError
	}
}

type ErrorData struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

type FileData struct {
	FileMeta
	Contents string `json:"contents,omitempty"`
}

func NewFileData(filePath string, fileInfo os.FileInfo, contents string) FileData {
	return FileData{FileMeta: NewFileMeta(filePath, fileInfo), Contents: contents}
}

type DirectoryData struct {
	FileMeta
	Entries []DirectoryEntry `json:"entries"`
}

func NewDirectoryData(dirPath string, fileInfo os.FileInfo, dirEntries []os.DirEntry) DirectoryData {
	entries := make([]DirectoryEntry, len(dirEntries))
	for i := range dirEntries {
		entries[i] = NewDirectoryEntry(dirPath, dirEntries[i])
	}

	return DirectoryData{
		FileMeta: NewFileMeta(dirPath, fileInfo),
		Entries:  entries,
	}
}

type DirectoryEntry struct {
	FileMeta
	Type string `json:"type"`
}

const DirectoryEntryTypeFile = ResponseTypeFile
const DirectoryEntryTypeDirectory = ResponseTypeDirectory
const DirectoryEntryTypeSymlink = "symlink"
const DirectoryEntryTypeUnsupported = "unsupported"

func NewDirectoryEntry(dirPath string, dirEntry os.DirEntry) DirectoryEntry {
	info, _ := dirEntry.Info()
	var entryType string
	switch {
	case info.Mode().IsRegular():
		entryType = DirectoryEntryTypeFile
	case info.Mode().IsDir():
		entryType = DirectoryEntryTypeDirectory
	case info.Mode().Type()&os.ModeSymlink != 0:
		entryType = DirectoryEntryTypeSymlink
	default:
		entryType = DirectoryEntryTypeUnsupported
	}

	return DirectoryEntry{
		FileMeta: NewFileMeta(path.Join(dirPath, info.Name()), info),
		Type:     entryType,
	}
}

type FileMeta struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Owner       string `json:"owner"`
	Permissions string `json:"permissions"`
	Size        uint64 `json:"size"`
}

func NewFileMeta(filePath string, fileInfo os.FileInfo) FileMeta {
	userId := strconv.FormatUint(uint64(fileInfo.Sys().(*syscall.Stat_t).Uid), 10)
	return FileMeta{
		Name:        path.Base(filePath),
		Path:        filePath,
		Owner:       userId,
		Size:        uint64(fileInfo.Size()),
		Permissions: fmt.Sprintf("0%o", fileInfo.Mode().Perm()),
	}
}

type PostFileRequest struct {
	Name        string `json:"name"`
	Permissions string `json:"permissions"`
	Content     string `json:"content,omitempty"`
}

type PutFileRequest struct {
	Permissions string `json:"permissions"`
	Content     string `json:"content,omitempty"`
}

func main() {
	contentRoot := os.Getenv("FILE_SERVER_CONTENT_ROOT")
	if contentRoot == "" {
		contentRoot = "."
	}

	listenAddress := os.Getenv("FILE_SERVER_LISTEN_ADDRESS")
	if listenAddress == "" {
		listenAddress = "localhost:8080"
	}

	log.Printf("listening on %s...", listenAddress)
	log.Fatal(http.ListenAndServe(listenAddress, httpHandler(contentRoot)))
}

func httpHandler(contentRoot string) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGet(contentRoot, w, r)
		case http.MethodPost:
			handlePost(contentRoot, w, r)
		case http.MethodPut:
			handlePut(contentRoot, w, r)
		case http.MethodDelete:
			handleDelete(contentRoot, w, r)
		default:
			writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
}

func handleGet(contentRoot string, w http.ResponseWriter, r *http.Request) {
	fileName := path.Join(contentRoot, r.URL.Path)
	fileInfo, err := os.Stat(fileName)
	switch {
	case err == nil:
		break
	case os.IsNotExist(err):
		notFound(w, err)
		return
	default:
		internalServerError(w, err)
		return
	}

	switch {
	case fileInfo.Mode().IsRegular():
		writeFileResponse(w, r.URL.Path, fileName)
	case fileInfo.Mode().IsDir():
		writeDirResponse(w, r.URL.Path, fileName)
	default:
		badRequest(w, "unsupported file type")
	}
}

func handlePost(contentRoot string, w http.ResponseWriter, r *http.Request) {
	dirName := path.Join(contentRoot, r.URL.Path)

	info, err := os.Stat(dirName)
	switch {
	case err == nil && info.IsDir():
		break
	case os.IsNotExist(err):
		if err := os.MkdirAll(dirName, 0700); err != nil {
			internalServerError(w, err)
			return
		}
		break
	case err != nil:
		internalServerError(w, err)
		return
	default:
		badRequest(w, dirName+" is not a directory")
		return
	}

	var data []PostFileRequest
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		invalidJson(w, err)
		return
	}

	type createFileArgs struct {
		fileName string
		content  []byte
		perms    os.FileMode
	}
	var args []createFileArgs
	for _, fileData := range data {
		fileName := path.Join(dirName, fileData.Name)
		perms, err := strconv.ParseUint(fileData.Permissions, 8, 32)
		if err != nil {
			invalidPermissions(w, fileName)
			return
		}

		args = append(args, createFileArgs{
			fileName,
			[]byte(fileData.Content),
			os.FileMode(perms),
		})
	}

	for i := range args {
		if err := os.WriteFile(args[i].fileName, args[i].content, args[i].perms); err != nil {
			internalServerError(w, err)
			return
		}
	}
	writeDirResponse(w, r.URL.Path, dirName)
}

func handlePut(contentRoot string, w http.ResponseWriter, r *http.Request) {
	var data PutFileRequest
	if err := json.NewDecoder(r.Body).Decode(&data); err != nil {
		invalidJson(w, err)
		return
	}

	fileName := path.Join(contentRoot, r.URL.Path)
	dirName := path.Dir(fileName)

	_, err := os.Stat(dirName)
	switch {
	case os.IsNotExist(err):
		if err := os.MkdirAll(dirName, 0700); err != nil {
			internalServerError(w, err)
			return
		}
	case err != nil:
		internalServerError(w, err)
		return
	}

	info, err := os.Stat(fileName)
	switch {
	case err == nil && info.Mode().IsRegular():
		break
	case os.IsNotExist(err):
		break
	case err != nil:
		internalServerError(w, err)
	default:
		badRequest(w, fileName+" is not a file")
		return
	}

	perms, err := strconv.ParseUint(data.Permissions, 8, 32)
	if err != nil {
		invalidPermissions(w, fileName)
		return
	}

	if err := os.WriteFile(fileName, []byte(data.Content), os.FileMode(perms)); err != nil {
		internalServerError(w, err)
		return
	}

	writeFileResponse(w, r.URL.Path, fileName)
}

func handleDelete(contentRoot string, w http.ResponseWriter, r *http.Request) {
	fileName := path.Join(contentRoot, r.URL.Path)

	var err error
	if r.FormValue("recursive") == "true" {
		err = os.RemoveAll(fileName)
	} else {
		err = os.Remove(fileName)
	}

	switch {
	case err == nil:
		writeResponse(w, ResponseBody{Status: "ok", Type: ResponseTypeDeleted})
	case errors.Is(err, syscall.ENOTEMPTY):
		badRequest(w, err.Error())
	case os.IsNotExist(err):
		notFound(w, err)
	default:
		internalServerError(w, err)
	}
}

func writeFileResponse(w http.ResponseWriter, urlPath, filePath string) {
	fileInfo, err := os.Stat(filePath)
	if err != nil {
		internalServerError(w, err)
		return
	}

	contents, err := ioutil.ReadFile(filePath)
	if err != nil {
		internalServerError(w, err)
		return
	}

	fileData := NewFileData(urlPath, fileInfo, string(contents))
	writeResponse(w, ResponseBody{
		Status: "ok",
		Type:   ResponseTypeFile,
		File:   &fileData,
	})
}

func writeDirResponse(w http.ResponseWriter, urlPath, dirName string) {
	dirInfo, err := os.Stat(dirName)
	if err != nil {
		internalServerError(w, err)
		return
	}

	dirEntries, err := os.ReadDir(dirName)
	if err != nil {
		internalServerError(w, err)
		return
	}

	dirData := NewDirectoryData(urlPath, dirInfo, dirEntries)
	if urlPath == "/" {
		dirData.Name = "/"
	}
	writeResponse(w, ResponseBody{
		Status:    "ok",
		Type:      ResponseTypeDirectory,
		Directory: &dirData,
	})
}

func notFound(w http.ResponseWriter, err error) {
	writeErrorResponse(w, http.StatusNotFound, err.Error())
}

func invalidJson(w http.ResponseWriter, err error) {
	badRequest(w, fmt.Sprintf("invalid json: %v", err))
}

func invalidPermissions(w http.ResponseWriter, fileName string) {
	badRequest(w, fmt.Sprintf("%s has invalid octal permissions", fileName))
}

func badRequest(w http.ResponseWriter, reason string) {
	writeErrorResponse(w, http.StatusBadRequest, reason)
}

func internalServerError(w http.ResponseWriter, err error) {
	log.Println(err)
	writeErrorResponse(w, http.StatusInternalServerError, err.Error())
}

func writeErrorResponse(w http.ResponseWriter, code int, reason string) {
	writeResponse(w, ResponseBody{
		Status: "error",
		Type:   ResponseTypeError,
		Error:  &ErrorData{Code: code, Error: reason},
	})
}

func writeResponse(w http.ResponseWriter, response ResponseBody) {
	w.Header().Add("Content-Type", "application/json")
	w.WriteHeader(response.Code())
	if err := json.NewEncoder(w).Encode(&response); err != nil {
		log.Println(err)
	}
}
