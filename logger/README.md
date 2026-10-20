# JSON 字段与脱敏

JSON 对象或数组可以直接作为字段字符串传入，调用方式不变：

```go
logger.Info(ctx, "请求", "data", `{"password":"secret","id":7516676763448115301}`)
```

配置 `DesensitizeField: "password"` 后，JSON 日志保留 `data` 的对象结构：

```json
{"msg":"请求","data":{"id":7516676763448115301,"password":"******"}}
```

`data` 不会成为 `"{\"id\":...}"` 这样的转义字符串。内部使用带 `json.RawMessage` 的日志值，让 slog 按 JSON 输出，同时通过 `String()` 保证 TEXT 格式显示可读 JSON，不会显示字节数字。

- 未配置脱敏字段时，直接传入的有效 JSON 对象、数组字符串也保持结构。
- 开启脱敏后，JSON 对象、数组、嵌套 JSON 字符串会递归脱敏。`json.RawMessage` 及其指针也可直接传入，按 JSON 处理。
- 解析时使用 `json.Number`，避免 ID、大整数或小数先转成 `float64` 后丢失精度。
- 普通字符串、无效 JSON、`"123"` 这类普通标量文本保持字符串；XML 保持原有脱敏和字符串输出方式。
- 整个字段本身被配置为敏感字段时，仍优先对整个字段脱敏。日志正文 `msg` 和原有忽略字段的处理保持不变。
- 如果自定义脱敏函数返回无法序列化的值，整个 JSON 字段替换成 `******`，避免回退输出原始敏感内容。

注意自定义脱敏回调的数字类型：**JSON 内部的数字字段现在传入 `json.Number`，不再是 `float64`**。例如：

```go
log.RegisterDesensitizeFunc(ctx, "account", func(v any) any {
    // 对 JSON 内的数字字段，可用 v.(json.Number).String() 获取精确文本。
    return "******"
})
```

普通 Go map / struct 中的原生数字类型不变。
