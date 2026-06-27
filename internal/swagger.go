package internal

import (
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
)

// swaggerSpec 是仅提取导入所需字段的宽松 spec 结构。
type swaggerSpec struct {
	Swagger  string                                `json:"swagger"`
	OpenAPI  string                                `json:"openapi"`
	BasePath string                                `json:"basePath"`
	Paths    map[string]map[string]json.RawMessage `json:"paths"`
	// Swagger2 定义
	Definitions map[string]json.RawMessage `json:"definitions"`
	// OpenAPI3 schemas
	Components map[string]json.RawMessage `json:"components"`
}

// operation 描述单个 HTTP 操作。
type operation struct {
	OperationID string                     `json:"operationId"`
	Summary     string                     `json:"summary"`
	Description string                     `json:"description"`
	Responses   map[string]json.RawMessage `json:"responses"`
}

// response 描述单个响应。
type response struct {
	Description string                     `json:"description"`
	Schema      json.RawMessage            `json:"schema"`
	Content     map[string]json.RawMessage `json:"content"`
}

// mediaType 描述 OpenAPI3 的 media type 对象。
type mediaType struct {
	Schema  json.RawMessage `json:"schema"`
	Example json.RawMessage `json:"example"`
}

// paramBrace 匹配 {name} 形式的路径参数。
var paramBrace = regexp.MustCompile(`\{(\w+)\}`)

// ImportSwagger 解析 Swagger2/OpenAPI3 spec，为每个 path×method 生成一条 mock，
// 批量写入 store。返回新建的 mock 列表。
func ImportSwagger(spec string, projectID string, enabled bool) ([]*Mock, error) {
	var s swaggerSpec
	if err := json.Unmarshal([]byte(spec), &s); err != nil {
		return nil, fmt.Errorf("invalid JSON: %w", err)
	}
	if len(s.Paths) == 0 {
		return nil, fmt.Errorf("no paths found in spec")
	}

	prefix := basePath(&s)
	root := schemaRoot(&s)

	var mocks []*Mock
	// 按 path 排序保证导入顺序稳定
	paths := make([]string, 0, len(s.Paths))
	for p := range s.Paths {
		paths = append(paths, p)
	}
	sort.Strings(paths)

	for _, p := range paths {
		methods := s.Paths[p]
		// method 也排序，保证稳定
		methodNames := make([]string, 0, len(methods))
		for m := range methods {
			methodNames = append(methodNames, m)
		}
		sort.Strings(methodNames)

		for _, method := range methodNames {
			mUpper := strings.ToUpper(method)
			if !ValidMethods[mUpper] {
				continue // 跳过 parameters 等非方法字段
			}
			var op operation
			if err := json.Unmarshal(methods[method], &op); err != nil {
				continue
			}

			fullPath := prefix + convertPath(p)
			m := &Mock{
				ProjectID: projectID,
				Method:    mUpper,
				Path:      fullPath,
				Name:      mockName(op, mUpper, fullPath),
				Status:    successStatus(op),
				Body:      exampleBody(&op, root),
				Enabled:   enabled,
			}
			mocks = append(mocks, m)
		}
	}

	if len(mocks) == 0 {
		return nil, fmt.Errorf("no importable operations found")
	}
	return mocks, nil
}

// basePath 返回路径前缀：Swagger2 的 basePath，或 OpenAPI3 首个 server 的 pathname。
func basePath(s *swaggerSpec) string {
	if s.BasePath != "" {
		return strings.TrimRight(s.BasePath, "/")
	}
	// OpenAPI3 servers 在顶层无法用简单结构提取，这里忽略；
	// 大多数 mock 场景 basePath 为空即可。
	return ""
}

// schemaRoot 返回用于解析 $ref 的根 map。
func schemaRoot(s *swaggerSpec) map[string]json.RawMessage {
	if s.Components != nil {
		if schemas, ok := s.Components["schemas"]; ok {
			var m map[string]json.RawMessage
			if json.Unmarshal(schemas, &m) == nil {
				return m
			}
		}
	}
	if s.Definitions != nil {
		return s.Definitions
	}
	return nil
}

// convertPath 将 {id} 形式转为 mock 的 :id 形式。
func convertPath(p string) string {
	return paramBrace.ReplaceAllString(p, ":$1")
}

// mockName 生成接口名称：优先 operationId，其次 summary，最后 METHOD path。
func mockName(op operation, method, path string) string {
	if op.OperationID != "" {
		return op.OperationID
	}
	if op.Summary != "" {
		return op.Summary
	}
	return method + " " + path
}

// successStatus 取首个 2xx 响应码，无则 200。
func successStatus(op operation) int {
	codes := make([]string, 0, len(op.Responses))
	for c := range op.Responses {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		if strings.HasPrefix(c, "2") {
			if n := atoiSafe(c); n > 0 {
				return n
			}
		}
	}
	return 200
}

// exampleBody 为操作的成功响应生成示例 JSON 字符串。
func exampleBody(op *operation, root map[string]json.RawMessage) string {
	// 找到成功响应
	var resp response
	codes := make([]string, 0, len(op.Responses))
	for c := range op.Responses {
		codes = append(codes, c)
	}
	sort.Strings(codes)
	for _, c := range codes {
		if strings.HasPrefix(c, "2") {
			_ = json.Unmarshal(op.Responses[c], &resp)
			break
		}
	}
	if len(op.Responses) == 0 {
		return "{}"
	}

	// OpenAPI3: content.application/json.schema
	if resp.Content != nil {
		if raw, ok := resp.Content["application/json"]; ok {
			var mt mediaType
			if json.Unmarshal(raw, &mt) == nil {
				if len(mt.Example) > 0 {
					return compactJSON(mt.Example)
				}
				if len(mt.Schema) > 0 {
					ex := generateExample(mt.Schema, root, map[string]bool{})
					return compactJSON(ex)
				}
			}
		}
	}
	// Swagger2: schema
	if len(resp.Schema) > 0 {
		ex := generateExample(resp.Schema, root, map[string]bool{})
		return compactJSON(ex)
	}
	return "{}"
}

// generateExample 根据 schema 递归生成示例值，支持 $ref 环检测。
func generateExample(schema json.RawMessage, root map[string]json.RawMessage, seen map[string]bool) any {
	// 解析为通用 map 以读取 type/$ref/properties 等
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(schema, &raw); err != nil {
		return "string"
	}

	// example / default 直接用
	if v, ok := raw["example"]; ok {
		var out any
		if json.Unmarshal(v, &out) == nil {
			return out
		}
	}
	if v, ok := raw["default"]; ok {
		var out any
		if json.Unmarshal(v, &out) == nil {
			return out
		}
	}

	// $ref
	if refRaw, ok := raw["$ref"]; ok {
		var ref string
		if json.Unmarshal(refRaw, &ref) != nil {
			return "string"
		}
		name := refName(ref)
		if name == "" || seen[name] {
			return "{}"
		}
		seen[name] = true
		defer delete(seen, name)
		if root != nil {
			if target, ok := root[name]; ok {
				return generateExample(target, root, seen)
			}
		}
		return "{}"
	}

	// allOf: 合并
	if allOf, ok := raw["allOf"]; ok {
		var arr []json.RawMessage
		if json.Unmarshal(allOf, &arr) == nil && len(arr) > 0 {
			merged := map[string]any{}
			for _, sub := range arr {
				subEx := generateExample(sub, root, seen)
				if sm, ok := subEx.(map[string]any); ok {
					for k, v := range sm {
						merged[k] = v
					}
				}
			}
			return merged
		}
	}
	// oneOf / anyOf: 取首项
	for _, key := range []string{"oneOf", "anyOf"} {
		if arrRaw, ok := raw[key]; ok {
			var arr []json.RawMessage
			if json.Unmarshal(arrRaw, &arr) == nil && len(arr) > 0 {
				return generateExample(arr[0], root, seen)
			}
		}
	}

	// type
	var typ string
	if t, ok := raw["type"]; ok {
		_ = json.Unmarshal(t, &typ)
	}

	switch typ {
	case "object":
		return objectExample(raw, root, seen)
	case "array":
		if items, ok := raw["items"]; ok {
			return []any{generateExample(items, root, seen)}
		}
		return []any{}
	case "string":
		if enum, ok := raw["enum"]; ok {
			var arr []any
			if json.Unmarshal(enum, &arr) == nil && len(arr) > 0 {
				return arr[0]
			}
		}
		return "string"
	case "integer", "number":
		return 0
	case "boolean":
		return false
	default:
		// 无 type 但有 properties 视为 object
		if _, ok := raw["properties"]; ok {
			return objectExample(raw, root, seen)
		}
		return "string"
	}
}

// objectExample 从 properties 生成对象示例。
func objectExample(raw map[string]json.RawMessage, root map[string]json.RawMessage, seen map[string]bool) map[string]any {
	out := map[string]any{}
	var props map[string]json.RawMessage
	if p, ok := raw["properties"]; ok {
		if json.Unmarshal(p, &props) != nil {
			return out
		}
	}
	// 按 key 排序保证稳定
	keys := make([]string, 0, len(props))
	for k := range props {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	for _, k := range keys {
		out[k] = generateExample(props[k], root, seen)
	}
	return out
}

// refName 从 $ref 字符串提取末段名，支持 #/definitions/X 与 #/components/schemas/X。
func refName(ref string) string {
	ref = strings.TrimPrefix(ref, "#/")
	parts := strings.Split(ref, "/")
	if len(parts) == 0 {
		return ""
	}
	return parts[len(parts)-1]
}

// compactJSON 将任意 JSON 字节/值序列化为紧凑字符串，失败回退 "{}"。
func compactJSON(v any) string {
	var b []byte
	switch x := v.(type) {
	case []byte:
		var anyVal any
		if json.Unmarshal(x, &anyVal) != nil {
			return "{}"
		}
		b, _ = json.Marshal(anyVal)
	default:
		b, _ = json.Marshal(v)
	}
	if len(b) == 0 {
		return "{}"
	}
	return string(b)
}

// atoiSafe 安全地把字符串转成 int。
func atoiSafe(s string) int {
	var n int
	if _, err := fmt.Sscanf(s, "%d", &n); err != nil {
		return 0
	}
	return n
}
