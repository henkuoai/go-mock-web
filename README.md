# go-mock-web

可视化 Mock 接口管理工具。后端 Go + Gin 提供管理 API 与 mock 请求分发；前端单页基于 TailwindCSS + Alpine.js（CDN 引入，无构建步骤）。数据持久化到本地 JSON 文件。

## 特性

- 📋 可视化管理 mock 接口：增删改查
- 🌐 支持全部 HTTP 方法：GET / POST / PUT / DELETE / PATCH / HEAD / OPTIONS
- 🎯 路径参数模式：`/users/:id`、`/assets/*` 通配
- 🧩 响应模板动态生成：可引用请求的路径参数 / 查询参数 / 请求头 / 请求体 JSON / 表单字段
- ⏱ 模拟延迟、自定义状态码与响应头
- 💾 JSON 文件持久化，重启不丢失

## 运行

```bash
go run .
```

默认监听 `:8080`，可用环境变量 `ADDR` 修改：

```bash
ADDR=:9090 go run .
```

浏览器访问 `http://localhost:8080` 管理 mock 接口。

## 使用

1. 在管理页点击「新建接口」，填写名称、方法、路径、状态码、响应体等。
2. 响应体可使用模板语法，例如：

   ```
   {"id":"{{.Path.id}}","name":"{{.Body.name}}","page":{{.Query.page}}}
   ```

3. 启用后，用任意 HTTP 客户端访问对应路径即可命中：

```bash
curl -X POST http://localhost:8080/users/42?page=1 \
  -H 'Content-Type: application/json' \
  -d '{"name":"alice"}'
```

4. 在列表中点击「测试」可直接发送请求并查看返回。

## 模板上下文

| 字段 | 说明 | 示例 |
| --- | --- | --- |
| `.Method` | 请求方法 | `{{.Method}}` |
| `.Path.<name>` | 路径参数 | `{{.Path.id}}` |
| `.Query.<name>` | 查询参数 | `{{.Query.page}}` |
| `.Body.<field>` | 请求体 JSON 字段（支持嵌套） | `{{.Body.user.id}}` |
| `.Form.<field>` | 表单字段 | `{{.Form.name}}` |
| `.RawBody` | 原始请求体字符串 | `{{.RawBody}}` |
| `.Header` | 请求头（键含特殊字符用 index） | `{{index .Header "X-Token"}}` |

## 项目结构

```
go-mock-web/
├── main.go                 # 入口
├── internal/
│   ├── model.go            # Mock 模型与校验
│   ├── store.go            # JSON 持久化与 CRUD
│   ├── matcher.go          # 路径匹配与特异性排序
│   ├── render.go           # 模板上下文与渲染
│   └── server/             # Gin 路由、管理 API、mock 分发
├── web/index.html          # 单页 UI
└── data/mocks.json         # 持久化数据（自动创建）
```

## 管理 API

| 方法 | 路径 | 说明 |
| --- | --- | --- |
| GET | `/api/mocks?q=&method=` | 列表（可过滤） |
| POST | `/api/mocks` | 新建 |
| GET | `/api/mocks/:id` | 详情 |
| PUT | `/api/mocks/:id` | 更新 |
| DELETE | `/api/mocks/:id` | 删除 |
