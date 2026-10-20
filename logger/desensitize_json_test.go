package logger

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func setupJSONDesensitizeTest(t *testing.T, fields string) *DesensitizeHandler {
	t.Helper()
	previous := Gfilelog
	Gfilelog = &Glog{
		Logconf:            &Glogconf{DesensitizeField: fields},
		DesensitizeFuncMap: make(map[string]DesensitizeFunc),
	}
	Gfilelog.loadDesensitizeField()
	t.Cleanup(func() { Gfilelog = previous })
	return NewDesensitizeHandler()
}

func renderJSONAttr(t *testing.T, attrs ...slog.Attr) map[string]json.RawMessage {
	t.Helper()
	var out bytes.Buffer
	log := slog.New(slog.NewJSONHandler(&out, &slog.HandlerOptions{ReplaceAttr: jsonReplaceAttr}))
	log.LogAttrs(context.Background(), slog.LevelInfo, "request", attrs...)
	var fields map[string]json.RawMessage
	if err := json.Unmarshal(out.Bytes(), &fields); err != nil {
		t.Fatalf("invalid log JSON: %s: %v", out.String(), err)
	}
	return fields
}

func TestJSONFieldRetainsStructure(t *testing.T) {
	for _, fields := range []string{"", "password"} {
		t.Run("mask="+fields, func(t *testing.T) {
			setupJSONDesensitizeTest(t, fields)
			const input = ` {"password":"test-secret","id":7516676763448115301,"nested":{"password":"nested-secret"},"items":[{"password":"item-secret"}],"decimal":1.2300,"large":1e400} `
			raw := json.RawMessage(input)
			for name, value := range map[string]any{"string": input, "raw": raw, "raw-pointer": &raw} {
				t.Run(name, func(t *testing.T) {
					log := renderJSONAttr(t, slog.Any("data", value))
					var data map[string]json.RawMessage
					if err := json.Unmarshal(log["data"], &data); err != nil {
						t.Fatalf("data must be an object rather than an escaped string or byte array: %s: %v", log["data"], err)
					}
					for key, want := range map[string]string{"id": "7516676763448115301", "decimal": "1.2300", "large": "1e400"} {
						if string(data[key]) != want {
							t.Errorf("%s = %s, want %s", key, data[key], want)
						}
					}
					want := `"test-secret"`
					if fields != "" {
						want = `"******"`
						if bytes.Contains(log["data"], []byte("secret")) {
							t.Fatalf("nested sensitive data survived masking: %s", log["data"])
						}
					}
					if string(data["password"]) != want {
						t.Fatalf("password = %s, want %s", data["password"], want)
					}
				})
			}
		})
	}
}

func TestJSONFieldArrayAndNestedJSONString(t *testing.T) {
	setupJSONDesensitizeTest(t, "password")
	log := renderJSONAttr(t,
		slog.String("data", `[{"password":"array-secret","id":9007199254740993}]`),
		slog.Any("container", map[string]any{
			"data": `{"password":"nested-secret","id":9007199254740993}`,
			"raw":  json.RawMessage(`{"password":"raw-secret"}`),
		}),
		slog.String("nestedString", `{"body":"{\"password\":\"encoded-secret\"}"}`),
	)
	if string(log["data"]) != `[{"id":9007199254740993,"password":"******"}]` {
		t.Fatalf("array = %s", log["data"])
	}
	var container map[string]json.RawMessage
	if err := json.Unmarshal(log["container"], &container); err != nil {
		t.Fatal(err)
	}
	if string(container["data"]) != `{"id":9007199254740993,"password":"******"}` || string(container["raw"]) != `{"password":"******"}` {
		t.Fatalf("nested JSON lost structure: %s", log["container"])
	}
	if string(log["nestedString"]) != `{"body":{"password":"******"}}` {
		t.Fatalf("nested encoded JSON = %s", log["nestedString"])
	}
}

func TestJSONFieldInvalidAndOrdinaryStringsStayStrings(t *testing.T) {
	for _, fields := range []string{"", "password"} {
		t.Run("mask="+fields, func(t *testing.T) {
			setupJSONDesensitizeTest(t, fields)
			for _, input := range []string{"ordinary text", `{"missing":`, `{"a":1} {"b":2}`, `[] trailing`, `123`, `true`, `null`, `"quoted text"`} {
				log := renderJSONAttr(t, slog.String("data", input))
				var actual string
				if err := json.Unmarshal(log["data"], &actual); err != nil || actual != input {
					t.Fatalf("input %q became %s, err=%v", input, log["data"], err)
				}
			}
		})
	}
}

func TestJSONFieldTextOutputRemainsReadable(t *testing.T) {
	for _, fields := range []string{"", "password"} {
		t.Run("mask="+fields, func(t *testing.T) {
			h := setupJSONDesensitizeTest(t, fields)
			text := &CustomHandler{}
			input := `{"password":"test-secret","id":9007199254740993}`
			for _, value := range []any{input, json.RawMessage(input)} {
				out := string(text.appendTextAttr(nil, slog.Any("data", value)))
				if !strings.HasPrefix(out, " data={") || !strings.Contains(out, `"id":9007199254740993`) {
					t.Fatalf("text JSON became byte numbers or escaped text: %s", out)
				}
				if fields != "" && (strings.Contains(out, "test-secret") || !strings.Contains(out, `"password":"******"`)) {
					t.Fatalf("text JSON not masked: %s", out)
				}
			}
			if fields != "" {
				out := string(text.appendTextAttr(nil, slog.Any("container", map[string]any{"data": input})))
				if strings.Contains(out, "test-secret") || !strings.Contains(out, `{"id":9007199254740993,"password":"******"}`) {
					t.Fatalf("nested text JSON = %s", out)
				}
				// A previously desensitized value may be logged again.
				out = string(text.appendTextAttr(nil, slog.Any("data", h.Desensitize(input))))
				if !strings.HasPrefix(out, " data={") || strings.Contains(out, "test-secret") {
					t.Fatalf("repeated masking changed JSON into bytes: %s", out)
				}
			}
		})
	}
}

func TestJSONFieldKeepsWholeFieldAndCustomMasking(t *testing.T) {
	setupJSONDesensitizeTest(t, "password,cardNo,amount")
	Gfilelog.DesensitizeFuncMap["cardNo"] = func(v any) any { return "masked-" + v.(string)[len(v.(string))-4:] }
	Gfilelog.DesensitizeFuncMap["amount"] = func(v any) any {
		if n, ok := v.(json.Number); !ok || n.String() != "9007199254740993" {
			t.Fatalf("custom numeric mask received %T(%v)", v, v)
		}
		return "hidden-amount"
	}
	log := renderJSONAttr(t,
		slog.String("password", `{"password":"whole-field-secret"}`),
		slog.String("data", `{"cardNo":"1234567890123456","amount":9007199254740993}`),
	)
	if string(log["password"]) != `"******"` || string(log["data"]) != `{"amount":"hidden-amount","cardNo":"masked-3456"}` {
		t.Fatalf("custom/whole-field masking = %s / %s", log["password"], log["data"])
	}
}

func TestJSONFieldMarshalFailureDoesNotExposeOriginal(t *testing.T) {
	setupJSONDesensitizeTest(t, "password")
	Gfilelog.DesensitizeFuncMap["password"] = func(any) any { return make(chan int) }
	log := renderJSONAttr(t, slog.String("data", `{"password":"test-secret"}`))
	if string(log["data"]) != `"******"` {
		t.Fatalf("failed masking should replace the complete field: %s", log["data"])
	}
}

func TestJSONFieldPreservesXMLMessagesAndErrors(t *testing.T) {
	setupJSONDesensitizeTest(t, "password")
	const xml = `<user><password>test-secret</password><name>alice</name></user>`
	log := renderJSONAttr(t, slog.String("xml", xml), slog.Any("error", errors.New("test error")))
	var maskedXML string
	if err := json.Unmarshal(log["xml"], &maskedXML); err != nil || maskedXML != `<user><password>******</password><name>alice</name></user>` {
		t.Fatalf("XML = %s, %v", log["xml"], err)
	}
	if string(log["error"]) != `"test error"` {
		t.Fatalf("error = %s", log["error"])
	}
	attr := DesensitizeReplaceAttr(nil, slog.String(slog.MessageKey, `{"password":"message-content"}`))
	if attr.Value.Kind() != slog.KindString || attr.Value.String() != `{"password":"message-content"}` {
		t.Fatalf("message handling unexpectedly changed: %v", attr)
	}
}

func TestJSONFieldPublicLoggerWritesStructuredData(t *testing.T) {
	for _, format := range []int{JSON_FORMAT, TEXT_FORMAT} {
		t.Run(map[int]string{JSON_FORMAT: "json", TEXT_FORMAT: "text"}[format], func(t *testing.T) {
			previous, previousDefault := Gfilelog, slog.Default()
			dir := t.TempDir()
			log := Newglog(dir, "structured.log", "structured.log.err", &Glogconf{
				RotateMethod: ROTATE_FILE_DAILY, Loglevel: INFO, Format: format, DesensitizeField: "password",
			})
			t.Cleanup(func() {
				log.LogObj.logfile.Close()
				log.LogObj.logfile_err.Close()
				Gfilelog = previous
				slog.SetDefault(previousDefault)
			})
			Info(context.Background(), "request", "data", `{"password":"test-secret","id":7516676763448115301}`)
			raw, err := os.ReadFile(filepath.Join(dir, "structured.log"))
			if err != nil {
				t.Fatal(err)
			}
			if bytes.Contains(raw, []byte("test-secret")) {
				t.Fatalf("sensitive value in output: %s", raw)
			}
			if format == JSON_FORMAT {
				var record map[string]json.RawMessage
				if err := json.Unmarshal(raw, &record); err != nil || string(record["data"]) != `{"id":7516676763448115301,"password":"******"}` {
					t.Fatalf("logger.Info output = %s, err=%v", raw, err)
				}
			} else if !bytes.Contains(raw, []byte(`data={"id":7516676763448115301,"password":"******"}`)) {
				t.Fatalf("logger.Info text output = %s", raw)
			}
		})
	}
}
