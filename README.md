## ChatGPT-Proxy-Registry

### 简介

本调度器是一个用于管理和路由OpenAI ChatGPT API密钥的系统。它包括一个主调度器和一个Web扩展和一个简单的前端页面，能够实时监控密钥的使用情况并进行密钥管理。需要搭配ChatGPT-Proxy-Executor使用。

### 安装

#### 需求

- Go语言 (>=1.14)

#### 步骤

1. 克隆仓库到本地

2. 进入项目目录并编译代码

   ```bash
   cd your_project_path
   go build
   ```

3. 运行编译后的可执行文件

   ```
   ./your_executable_name
   ```

### 结构

- `main.go`: 主要的调度器逻辑
- `webExtension.go`: Web扩展逻辑
- `index.html`: 前端界面代码

### 功能

- **API密钥管理**: 能够添加、删除和编辑API密钥
- **实时监控**: 通过前端仪表板实时监控密钥的使用情况
- **扩展支持**: 通过Web扩展提供更多自定义功能

### 使用

打开浏览器并访问调度器的前端界面（例如 `http://localhost:8080/dashboard`）以查看和管理执行器列表。

