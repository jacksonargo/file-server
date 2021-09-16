package main

import (
	"encoding/json"
	"errors"
	"flag"
	"io/ioutil"
	"log"
	"net/http"
	"os"
	"path"
)

const ResponseTypeFile = "file"
const ResponseTypeDirectory = "directory"
const ResponseTypeError = "error"

type ResponseBody struct {
	Status    string         `json:"status"`
	Type      string         `json:"type"`
	File      *FileData      `json:"file_data,omitempty"`
	Directory *DirectoryData `json:"directory,omitempty"`
	Error     *ErrorData     `json:"error,omitempty"`
}

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

type FileData struct {
	Name        string `json:"name"`
	Path        string `json:"path"`
	Owner       string `json:"owner"`
	Size        uint64 `json:"size"`
	Permissions uint8  `json:"permissions"`
	Contents    string `json:"contents"`
}

func NewFileData(path string, fileInfo os.FileInfo, contents string) FileData {
	return FileData{
		Name:        fileInfo.Name(),
		Path:        path,
		Owner:       "implement me", // TODO: Implement file owner
		Size:        uint64(fileInfo.Size()),
		Permissions: uint8(fileInfo.Mode().Perm()),
		Contents:    contents,
	}
}

type DirectoryData struct {
	Name        string   `json:"name"`
	Path        string   `json:"path"`
	Owner       string   `json:"owner"`
	Size        uint64   `json:"size"`
	Permissions uint8    `json:"permissions"`
	Contents    []string `json:"contents"`
}

func NewDirectoryData(path string, fileInfo os.FileInfo, dirEntries []os.DirEntry) DirectoryData {
	contents := make([]string, len(dirEntries))
	for i := range dirEntries {
		contents[i] = dirEntries[i].Name()
	}

	fileData := NewFileData(path, fileInfo, "")
	return DirectoryData{
		Name:        fileData.Name,
		Path:        fileData.Path,
		Owner:       fileData.Owner,
		Size:        fileData.Size,
		Permissions: fileData.Permissions,
		Contents:    contents,
	}
}

type ErrorData struct {
	Code  int    `json:"code"`
	Error string `json:"error"`
}

func main() {
	var contentRoot string
	flag.StringVar(&contentRoot, "root", ".", "directory to serve")
	flag.Parse()

	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		switch r.Method {
		case http.MethodGet:
			handleGet(contentRoot, w, r)
		default:
			writeErrorResponse(w, http.StatusMethodNotAllowed, "method not allowed")
		}
	})
	log.Println("listening on localhost:8080...")
	log.Fatal(http.ListenAndServe("localhost:8080", nil))
}

func handleGet(contentRoot string, w http.ResponseWriter, r *http.Request) {
	fileName := path.Join(contentRoot, r.URL.Path)
	fileInfo, err := os.Stat(fileName)
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
