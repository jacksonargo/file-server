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
