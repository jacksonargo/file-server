# file-server

An http api for retrieving file contents and metadata from a local filesystem.

## System Requirements

* [Docker](https://www.docker.com/products/docker-desktop) | For remote deployments or local testing.
* [Golang](https://golang.org/dl/) | For local developemnt.

## Basic Usage

```bash
# Clone the repo
git clone https://github.com/jacksonargo/file-server.git && cd file-server
# Launch the service. By default it listens on localhost:8080.
./run.sh /super/important/content
```

## Advanced Usage

### Environment Variables

The file-server binary supports configuration via the following environment variables:

|Name|Default|Description|
|----|-------|-----------|
|`FILE_SERVER_LISTEN_ADDRESS`|`localhost:8080`|Http listen address.|
|`FILE_SERVER_CONTENT_ROOT`|`.`|Path to the content directory.|

For greater control over the port mappings and other options in docker deployments, you can build and launch the service using the docker client directly.

```bash
docker build . -t file-server
docker run --rm \
  --publish 8080:8080 \
  --name "file-server-1" \
  --volume "/super/important/content:/srv:ro" \
  --env "FILE_SERVER_LISTEN_ADDRESS=0.0.0.0:8080" \
  --env "FILE_SERVER_CONTENT_ROOT=/srv" \
  file-server
```

## Api Endpoints

### Get File and Directory Content

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
$ curl -s -XGET localhost:8080/|jq .
{
  "status": "ok",
  "type": "directory",
  "directory": {
    "name": "/",
    "path": "/",
    "owner": "1000",
    "permissions": "0700",
    "size": 512,
    "entries": [
      {
        "name": "extras",
        "path": "/extras",
        "owner": "1000",
        "permissions": "0700",
        "size": 13,
        "type": "file"
      },
      {
        "name": "hello.txt",
        "path": "/hello.txt",
        "owner": "1000",
        "permissions": "0600",
        "size": 6,
        "type": "file"
      }
    ]
  }
}

```

## Api Objects

### `Response`

The generic reponse object returned by all requests.

|Field|Type|Summary|
|-----|----|-------|
|`status`|`string`|Ok when successful or error if an error occured.|
|`type`|`ResponseType`|Type of data in this document.|
|`file`|`*FileData`|(Optional) The file contents and metadata. Null unless type is file.|
|`directory`|`*DirectoryData`|(Optional) The directory contents and metadata. Null unless type is directory.|
|`error`|`*ErrorData`|(Optional) An error code and message.Null unless type is error.|

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
|`"unsupported"`|Entry is neither a file, directory, or symlink.|


