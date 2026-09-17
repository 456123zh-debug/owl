# OWL 本地开发与 Windows 运行说明

## 一键重新构建并启动 Windows 版本

在终端中进入项目目录并执行：

```bash
cd D:\code\owl_workspace\owl
make dev/windows
```

脚本会构建当前源码，停止发布目录中的服务，备份并替换 `owl.exe`，然后以完整功能模式重新启动。旧程序保存在 `dist\owl-windows-amd64\owl.exe.bak`。

日志位置：

```text
dist\owl-windows-amd64\run\owl.out
dist\owl-windows-amd64\run\owl.err
```

## IDE 调试

调试入口使用项目根目录的 `main.go`，工作目录必须设置为：

```text
D:\code\owl_workspace\owl\dist\owl-windows-amd64
```

环境变量：

```text
OWL_ONE_CLICK=1
GOSUMDB=sum.golang.org
GOTOOLCHAIN=auto
```

这样会复用发布包中的配置、数据库、前端资源、录像目录和 MediaServer，功能与发布包保持一致。

## 修改后实时生效

Go 后端修改后需要重新编译和重启。再次执行下面的命令即可自动完成，无需手工复制文件：

```bash
make dev/windows
```

前端的 `www` 是构建产物。前端开发应在前端源码项目中运行开发服务器；构建完成后，将生成的 `dist` 内容更新到 `dist\owl-windows-amd64\www`。

## 录像过滤接口

只查询存在录像的通道：

```text
GET /local/channels?page=1&size=20&has_recording=true
```

只查询不存在录像的通道：

```text
GET /local/channels?page=1&size=20&has_recording=false
```
