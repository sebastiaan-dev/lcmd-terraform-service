# Terraform Service

This repository contains a wrapper over the LZC SDK, exposing it as an API which can be exposed by a Terraform provider.

## 开发

### 启动后端

启动一个终端,执行下面命令

```
lzc-cli project devshell
# 进入容器后
cd backend
go run .
```

### 启动前端

```
lzc-cli project devshell
# 进入容器后
cd ui
npm i
npm run dev
```

## 构建

```
lzc-cli project build -o release.lpk
```

会在当前的目录下构建出一个 lpk 包。

## 安装

```
lzc-cli app install release.lpk
```

会安装在你的微服应用中,安装成功后可在懒猫微服启动器中查看!

## 交流和帮助

你可以在 https://bbs.lazycat.cloud/ 畅所欲言。
