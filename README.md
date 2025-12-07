# LCMD Terraform Service

This repository contains a wrapper over the LZC SDK, exposing it as an API which can be exposed by a Terraform provider.

## Development Setup

```
lzc-cli project devshell
# 进入容器后
cd ui
npm i
npm run dev
```

## Wrap Application into LPK format

```
lzc-cli project build -o release.lpk
```

会在当前的目录下构建出一个 lpk 包。

## Install LPK onto MicroServer

```
lzc-cli app install release.lpk
```
