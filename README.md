# goctl-go-compact

### 1. 编译goctl-go-compact插件

```
$ GO111MODULE=on GOPROXY=https://goproxy.cn/,direct go get -u github.com/zeromicro/goctl-go-compact
```

### 2. 配置环境
将$GOPATH/bin中的goctl-go-compact添加到环境变量

### 3. 使用姿势

$ goctl api plugin -p goctl-go-compact -api user.api -dir .
