package main

import (
	"fmt"
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
	Contents    string `json:"contents,omitempty"`
}

type PutFileRequest struct {
	Permissions string `json:"permissions"`
	Contents     string `json:"contents,omitempty"`
}
