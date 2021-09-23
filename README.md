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

## Endpoints

### Get File Content

```
GET /PATH/TO/FILE
```

Returns a json response with the file contents and metadata.

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

### Get Directory Content

```
GET /PATH/TO/DIRECTORY
```

Returns a json response with the directory metadata and metadata for all files and directories within it.

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
        "type": "directory"
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

### Create a File

```
PUT /PATH/TO/FILE
```

#### JSON Request Params
*Object*
|Field|Type|Summary|
|-----|----|-------|
|`permissions`|`string`|The file octal permissions.|
|`contents`|`string`|The file contents.|

Create the file with the provided content and permissions. Any intermediate directories are created with permissions 0700. Returns a json response with the created file's contents and metadata.

```bash
$ curl -s -XPUT localhost:8080/some/new/path/hello.txt -d'{"permissions":"0600","contents":"hello\n"}'|jq .
{
  "status": "ok",
  "type": "file",
  "file_data": {
    "name": "hello.txt",
    "path": "/some/new/path/hello.txt",
    "owner": "1000",
    "permissions": "0600",
    "size": 6,
    "contents": "hello\n"
  }
}
```

### Create Many Files

```
POST /PATH/TO/DIRECTORY
```

#### JSON Request Params
*Array of*
|Field|Type|Summary|
|-----|----|-------|
|`name`|`string`|The file name.|
|`permissions`|`string`|The file octal permissions.|
|`contents`|`string`|The file contents.|

Create all files with the provided content and permissions. Any intermediate directories are created with permissions 0700. Returns a json response with the directory contents and metadata.

```bash
$ curl -s -XPOST localhost:8080 -d'[{"name": "file.txt", "permissions": "0600", "content": "hello\n"}]'|jq .
{
  "status": "ok",
  "type": "directory",
  "directory": {
    "name": "/",
    "path": "/",
    "owner": "0",
    "permissions": "0755",
    "size": 4096,
    "entries": [
      {
        "name": "file.txt",
        "path": "/file.txt",
        "owner": "0",
        "permissions": "0600",
        "size": 6,
        "type": "file"
      }
    ]
  }
}

```


### Deleting Files

```
DELETE /PATH/TO/FILE
```

Deletes the file.

```bash
$ curl -s -XDELETE localhost:8080/hello.txt
{"status":"ok","type":"deleted"}
```

### Deleting Directories

```
DELETE /PATH/TO/DIRECTORY
```

#### URL Query Params
|Field|Type|Summary|
|-----|----|-------|
|`recursive`|`*boolean`|(Optional) If true, delete the directory recursively.|

If the path is a directory, it must be empty or provide `recursive=true` as a url param to delete recursively.

```bash
$ curl -s -XDELETE localhost:8080/scratch?recursive=true
{"status":"ok","type":"deleted"}
```


## Response Data

### `Response`
*Object*

The generic reponse object returned by all requests.

|Field|Type|Summary|
|-----|----|-------|
|`status`|`string`|Ok when successful or error if an error occured.|
|`type`|`ResponseType`|Type of data in this document.|
|`error`|`*ErrorData`|(Optional) An error code and message. Null unless type is error.|
|`file`|`*FileData`|(Optional) The file contents and metadata. Null unless type is file.|
|`directory`|`*DirectoryData`|(Optional) The directory contents and metadata. Null unless type is directory.|

### `ResponseType`
*String*

Used to indicate type of data in the response. Can be one of the following:

|ResponseType|Summary|
|----|-------|
|`"error"`|An error occured while executing the request.|
|`"file"`|The requested file is a regular file.|
|`"directory"`|The requested file is a directory.|
|`"deleted"`|The requested file was deleted.|

### `ErrorData`
*Object*

Contains an error code and message. This is returned if an error occurs while handling the request.

|Field|Type|Summary|
|-----|----|-------|
|`code`|`string`|The name of the file.|
|`error`|`string`|The url path to the file.|

### `FileData`
*Object*

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
*Object*

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
*Object*

|Field|Type|Summary|
|-----|----|-------|
|`type`|`DirectoryEntryType`|The type of entry.|
|`name`|`string`|The name of the entry.|
|`path`|`string`|The url path to the entry.|
|`owner`|`string`|The numeric id of the owner.|
|`permissions`|`string`|The octal permissions.|
|`size`|`int`|The size in bytes.|

### `DirectoryEntryType`
*String*

The file type for a directory entry. Can be one of the following:

|DirectoryEntryType|Summary|
|----|-------|
|`"file"`|Entry is a regular file.|
|`"directory"`|Entry is a directory.|
|`"symlink"`|Entry is a symlink.|
|`"unsupported"`|Entry is neither a file, directory, or symlink.|


