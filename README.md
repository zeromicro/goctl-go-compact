# goctl-go-compact

这个插件作用是将goctl默认的一个路由一个文件合并成一个文件

### 1. 编译goctl-go-compact插件

```
$ go install github.com/zeromicro/goctl-go-compact
```

### 2. 配置环境
将$GOPATH/bin中的goctl-go-compact添加到环境变量

### 3. 使用姿势

$ goctl api plugin -p goctl-go-compact -api user.api -dir .
