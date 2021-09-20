# file-server

## Installation

Clone the repo
```bash
git clone https://github.com/jacksonargo/file-server.git
```

## Basic Usage

```bash
./run.sh /some/directory/to/serve
```

## Advanced Usage

```bash
docker build . -t "$IMAGE_NAME"
docker run --rm \
  --publish 8080:8080 \
  --name "$CONTAINER_NAME" \
  --volume "${CONTENT_ROOT}:/srv:ro" \
  --env "FILE_SERVER_LISTEN_ADDRESS=0.0.0.0:8080" \
  --env "FILE_SERVER_CONTENT_ROOT=/srv" \
  "$IMAGE_NAME"
```

## Get File and Directory Content

Files are returned with the full content.

```
GET /PATH/TO/FILE
```

```bash
$ curl -s -XGET localhost:8080/hello.txt|jq .
{
  "status": "ok",
  "type": "file",
  "file_data": {
    "name": "hello.txt",
    "path": "/hello.txt",
    "owner": "1000",
    "permissions": "0600",
    "size": 6,
    "contents": "hello\n"
  }
}
```

If the path is a directory, the response contains the directory metadata along with metadata for all files and directories within it.

```
GET /PATH/TO/DIRECTORY
```

```bash
$ curl -s -XGET localhost:8080/hello.txt|jq .
{
  "status": "ok",
  "type": "directory",
  "directory": {
    "name": "/",
    "path": "/",
    "owner": "1000",
    "permissions": "0777",
    "size": 512,
    "entries": [
      {
        "name": "extras",
        "path": "/extras",
        "owner": "1000",
        "permissions": "0777",
        "size": 13,
        "type": "file"
      },
      {
        "name": "hello.txt",
        "path": "/hello.txt",
        "owner": "1000",
        "permissions": "0777",
        "size": 6,
        "type": "file"
      }
    ]
  }
}

```

## Objects

### `Response`

The generic reponse object returned by all requests.

|Field|Type|Summary|
|-----|----|-------|
|`status`|`string`|Ok when successful or error if an error occured.|
|`type`|`ResponseType`|Type of data in this document.|
|`file`|`*FileData`|The file contents and metadata. Null unless type is file.|
|`directory`|`*DirectoryData`|The directory contents and metadata. Null unless type is directory|
|`error`|`*ErrorData`|An error code and message.Null unless type is error.|

### `ResponseType`

Used to indicate type of data in the response. Can be one of the following:

| ResponseType |Summary|
|----|-------|
|`"error"`|Response contains ErrorData.|
|`"file"`|Response contains FileData.|
|`"directory"`|Response contains DirectoryData.|

### `ErrorData`

Contains an error code and message. This is returned if an error occurs while handling the request.

|Field|Type|Summary|
|-----|----|-------|
|`code`|`string`|The name of the file.|
|`error`|`string`|The url path to the file.|

### `FileData`

File metadata and contents. This is returned when the request path is a file or a symlink to a file.

|Field|Type|Summary|
|-----|----|-------|
|`name`|`string`|The name of the file.|
|`path`|`string`|The url path to the file.|
|`owner`|`string`|The numeric id of the owner.|
|`permissions`|`string`|The file octal permissions.|
|`size`|`int`|The size of the file in bytes.|
|`contents`|`string`|The file contents.|

### `DirectoryData`

Directory metadata and a list of directory entries. This is returned when the request path is a directory or a symlink to a directory.

|Field|Type|Summary|
|-----|----|-------|
|`name`|`string`|The name of the directory.|
|`path`|`string`|The url path to the ditrectory.|
|`owner`|`string`|The numeric id of the owner.|
|`permissions`|`string`|The file octal permissions.|
|`size`|`int`|The size of the directory in bytes.|
|`entries`|`List of DirectoryEntry`|The directory contents.|

### `DirectoryEntry`

|Field|Type|Summary|
|-----|----|-------|
|`type`|`DirectoryEntryType`|The type of entry.|
|`name`|`string`|The name of the entry.|
|`path`|`string`|The url path to the entry.|
|`owner`|`string`|The numeric id of the owner.|
|`permissions`|`string`|The octal permissions.|
|`size`|`int`|The size in bytes.|

### `DirectoryEntryType`

The file type for a directory entry. Can be one of the following:

| ResponseType |Summary|
|----|-------|
|`"file"`|Entry is a regular file.|
|`"directory"`|Entry is a directory.|
|`"symlink"`|Entry is a symlink.|
|`"unsupport"`|Entry is neither a file, directory, or symlink.|


