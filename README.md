# go-mock-web

可视化 Mock 接口管理工具。后端 Go + Gin 提供管理 API 与 mock 请求分发；前端单页基于 TailwindCSS + Alpine.js（CDN 引入，无构建步骤）。数据持久化到本地 JSON 文件。

## 特性

- 📁 **项目管理**：按项目组织 mock 接口，项目级增删改查，删除项目级联清理其下接口
- 📥 **Swagger 导入**：粘贴 Swagger 2.0 / OpenAPI 3.0 JSON，自动批量生成接口；`{id}` 路径参数转 `:id`，并按响应 schema 递归生成示例响应体（支持 `$ref`、嵌套对象、数组、enum）
- 📋 可视化管理 mock 接口：增删改查
- 🌐 支持全部 HTTP 方法：GET / POST / PUT / DELETE / PATCH / HEAD / OPTIONS
- 🎯 路径参数模式：`/users/:id`、`/assets/*` 通配
- 🧩 响应模板动态生成：可引用请求的路径参数 / 查询参数 / 请求头 / 请求体 JSON / 表单字段
- ⏱ 模拟延迟、自定义状态码与响应头
- 🔁 curl 导入 / 导出：粘贴 curl 快速建接口，或导出调用命令到终端验证
- 💾 JSON 文件持久化，重启不丢失；旧版扁平数据自动迁移为项目结构

## 运行

```bash
go run .
```

默认监听 `:8080`，可用环境变量 `ADDR` 修改：

```bash
ADDR=:9090 go run .
```

浏览器访问 `http://localhost:8080` 管理。

## 使用流程

### 1. 新建项目
首页点击「+ 新建项目」，填写名称与描述。

### 2. 导入 Swagger
进入项目后点击「导入 Swagger」，粘贴 spec JSON。例如：

```json
{
  "swagger": "2.0",
  "basePath": "/api/v1",
  "definitions": {
    "User": {
      "type": "object",
      "properties": {
        "id": { "type": "integer" },
        "name": { "type": "string" },
        "role": { "type": "string", "enum": ["admin", "user"] }
      }
    }
  },
  "paths": {
    "/users/{id}": {
      "get": {
        "operationId": "getUser",
        "responses": { "200": { "description": "ok", "schema": { "$ref": "#/definitions/User" } } }
      }
    },
    "/users": {
      "post": {
        "summary": "创建用户",
        "responses": { "201": { "description": "created", "schema": { "$ref": "#/definitions/User" } } }
      }
    }
  }
}
```

导入后得到两条接口：
- `GET /api/v1/users/:id` → 200，响应体 `{"id":0,"name":"string","role":"admin"}`
- `POST /api/v1/users` → 201，响应体同上

可勾选「导入后立即启用」，否则导入为禁用状态需手动开启。

### 3. 调用接口
启用后用任意 HTTP 客户端访问：

```bash
curl http://localhost:8080/api/v1/users/42
# {"id":0,"name":"string","role":"admin"}
```

### 4. 接口测试
列表点击「测试」可直接发送请求并查看状态码与响应；点击「curl」导出调用命令。

## 模板上下文

响应体可使用模板语法动态生成：

| 字段 | 说明 | 示例 |
| --- | --- | --- |
| `.Method` | 请求方法 | `{{.Method}}` |
| `.Path.<name>` | 路径参数 | `{{.Path.id}}` |
| `.Query.<name>` | 查询参数 | `{{.Query.page}}` |
| `.Body.<field>` | 请求体 JSON 字段（支持嵌套） | `{{.Body.user.id}}` |
| `.Form.<field>` | 表单字段 | `{{.Form.name}}` |
| `.RawBody` | 原始请求体字符串 | `{{.RawBody}}` |
| `.Header` | 请求头（键含特殊字符用 index） | `{{index .Header "X-Token"}}` |

## 管理 API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/projects` | 项目列表（含接口数） |
| POST | `/api/projects` | 新建项目 |
| PUT | `/api/projects/:id` | 更新项目 |
| DELETE | `/api/projects/:id` | 删除项目（级联接口） |
| POST | `/api/projects/:id/import` | 导入 Swagger，body: `{"spec":"...","enabled":true}` |
| GET | `/api/mocks?projectId=&q=&method=` | 接口列表（按项目过滤） |
| POST | `/api/mocks` | 新建接口 |
| GET | `/api/mocks/:id` | 接口详情 |
| PUT | `/api/mocks/:id` | 更新接口 |
| DELETE | `/api/mocks/:id` | 删除接口 |

## 项目结构

```
go-mock-web/
├── main.go                 # 入口
├── internal/
│   ├── model.go            # Project / Mock 模型与校验
│   ├── store.go            # JSON 持久化、项目与接口 CRUD、旧数据迁移
│   ├── swagger.go          # Swagger/OpenAPI 解析与示例生成
│   ├── matcher.go          # 路径匹配与特异性排序
│   ├── render.go           # 模板上下文与渲染
│   └── server/             # Gin 路由、管理 API、mock 分发
├── web/index.html          # 单页 UI
└── data/store.json         # 持久化数据（自动创建）
```

## 数据迁移

从旧版（扁平 `data/mocks.json`）升级时，启动会自动读取旧文件、创建「默认项目」并将所有旧接口归入该项目，写入新的 `data/store.json`，原有数据不丢失。
