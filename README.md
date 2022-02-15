# goctl-go-compact

This plugin is used to merge all the routes in one file.

### 1. install goctl-go-compact

```
$ GO111MODULE=on GOPROXY=https://goproxy.cn/,direct go install github.com/zeromicro/goctl-go-compact@latest
```

### 2. environment setup

Make sure the installed `goctl-go-compact` in your `$PATH`

### 3. Usage

```
$ goctl api plugin -p goctl-go-compact -api user.api -dir .
```
