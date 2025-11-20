# Go SOCKS5 代理服务器使用指南

本文档将指导您如何编译、运行和使用 gosocks5.go 文件提供的 SOCKS5 代理服务器。

### 步骤一：环境准备

在开始之前，请确保您的系统已经安装了 Go 语言环境。您可以通过以下命令检查 Go 是否已安装及其版本：
```bash
go version
```
如果未安装，请访问 Go 官方网站 下载并安装。

### 步骤二：编译程序

将 gosocks5.go 文件保存在任意目录中。打开您的终端，进入该目录，然后执行以下命令来编译程序：
```bash
CGO_ENABLED=0 go build -ldflags="-s -w" -trimpath -o gosocks5 gosocks5.go
```
执行成功后，您会在当前目录下看到一个名为 gosocks5 (在 Windows 上是 gosocks5.exe) 的可执行文件。这就是我们的代理服务器程序。

### 步骤三：运行代理服务器

程序支持通过命令行标志（flag）来指定监听端口，非常灵活。

#### 1. 使用默认端口运行

直接运行可执行文件，服务器将启动并监听默认端口 50440。
```bash
./gosocks5
```
您将看到如下输出，表示服务器已成功启动：
```bash
SOCKS5 proxy server started successfully on port: 50440
```
#### 2. 指定自定义端口运行

如果您想使用其他端口（例如，标准的SOCKS端口 1080），可以使用 -port 标志：
```bash
./gosocks5 -port 1080
```
服务器将在 1080 端口上启动：
```bash
SOCKS5 proxy server started successfully on port: 1080
```
#### 3. 查看帮助信息

如果您忘记了如何使用，可以随时使用 -h 或 --help 标志来查看帮助信息：
```bash
./gosocks5 -h
```
输出：
```bash
Usage of ./gosocks5:
  -port string
        The port number for the SOCKS5 proxy to listen on (default "50440")
```
