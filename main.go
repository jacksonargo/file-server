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
	File      *FileData      `json:"file_data,omitempty"`
	Directory *DirectoryData `json:"directory,omitempty"`
}

const ResponseTypeFile = "file"
const ResponseTypeDirectory = "directory"
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

func main() {
	contentRoot := os.Getenv("FILE_SERVER_CONTENT_ROOT")
	listenAddress := os.Getenv("FILE_SERVER_LISTEN_ADDRESS")

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGet(contentRoot, w, r)
		default:
			writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	log.Printf("listening on %s...", listenAddress)
	log.Fatal(http.ListenAndServe(listenAddress,
		http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			switch r.Method {
			case http.MethodGet:
				handleGet(contentRoot, w, r)
			default:
				writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
			}
		})))
}

func handleGet(contentRoot string, w http.ResponseWriter, r *http.Request) {
	fileName := path.Join(contentRoot, r.URL.Path)
	fileInfo, err := os.Lstat(fileName)
	switch {
	case errors.Is(err, os.ErrNotExist):
		writeErrorResponse(w, http.StatusNotFound, err.Error())
		return
	case errors.Is(err, os.ErrPermission):
		writeErrorResponse(w, http.StatusUnauthorized, err.Error())
		return
	case err != nil:
		writeErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	default:
		break
	}

	path := r.URL.Path
	switch {
	case fileInfo.Mode().IsRegular():
		writeFileResponse(w, path, fileName, fileInfo)
	case fileInfo.Mode().IsDir():
		writeDirResponse(w, path, fileName, fileInfo)
	default:
		writeErrorResponse(w, http.StatusBadRequest, "unsupported file type")
	}
}

func writeFileResponse(w http.ResponseWriter, path, fileName string, fileInfo os.FileInfo) {
	contents, err := ioutil.ReadFile(fileName)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	fileData := NewFileData(path, fileInfo, string(contents))
	writeResponse(w, ResponseBody{
		Status: "ok",
		Type:   ResponseTypeFile,
		File:   &fileData,
	})
}

func writeDirResponse(w http.ResponseWriter, path, dirName string, fileInfo os.FileInfo) {
	dirEntries, err := os.ReadDir(dirName)
	if err != nil {
		writeErrorResponse(w, http.StatusInternalServerError, err.Error())
		return
	}

	dirData := NewDirectoryData(path, fileInfo, dirEntries)
	if path == "/" {
		dirData.Name = "/"
	}
	writeResponse(w, ResponseBody{
		Status:    "ok",
		Type:      ResponseTypeDirectory,
		Directory: &dirData,
	})
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
