package gconfig_v2_test

import (
	"context"
	"testing"
	"time"

	"github.com/lanwenhong/lgobase/gconfig_v2"
)

func requireMap(t *testing.T, v any) map[string]any {
	t.Helper()
	m, ok := v.(map[string]any)
	if !ok {
		t.Fatalf("want map[string]any, got %T", v)
	}
	return m
}

func requireSlice(t *testing.T, v any) []any {
	t.Helper()
	s, ok := v.([]any)
	if !ok {
		t.Fatalf("want []any, got %T", v)
	}
	return s
}

func requireTime(t *testing.T, v any) time.Time {
	t.Helper()
	tm, ok := v.(time.Time)
	if !ok {
		t.Fatalf("want time.Time, got %T", v)
	}
	return tm
}

func requireDuration(t *testing.T, v any) time.Duration {
	t.Helper()
	d, ok := v.(time.Duration)
	if !ok {
		t.Fatalf("want time.Duration, got %T", v)
	}
	return d
}

func TestLoadConfFile(t *testing.T) {
	ctx := context.Background()
	//t.Log("start")
	m := make(map[string]any)
	if err := gconfig_v2.UnmarshalFile(ctx, "test_rule.yaml", &m); err != nil {
		t.Fatal(err)
	}
	if m["name"] != "myapp" {
		t.Fatalf("name = %v", m["name"])
	}
	if m["debug"] != true {
		t.Fatalf("debug = %v", m["debug"])
	}
	server := requireMap(t, m["server"])
	if server["host"] != "127.0.0.1" {
		t.Fatalf("server.host = %v", server["host"])
	}
	if server["port"] != 8080 {
		t.Fatalf("server.port = %v", server["port"])
	}
}

func TestLoadConfFile2(t *testing.T) {
	ctx := context.Background()
	//t.Log("start")
	m := make(map[string]any)
	if err := gconfig_v2.UnmarshalFile(ctx, "config1.yaml", &m); err != nil {
		t.Fatal(err)
	}
	if m["version"] != "1.0.0" {
		t.Fatalf("version = %v", m["version"])
	}
	server := requireMap(t, m["server"])
	test1 := requireMap(t, server["test1"])
	test2 := requireMap(t, test1["test2"])
	if test2["tv"] != 3333 || test2["tk"] != 4444 {
		t.Fatalf("server.test1.test2 = %v", test2)
	}
}

func TestLoadConfFile3(t *testing.T) {
	ctx := context.Background()
	//t.Log("start")
	m := make(map[string]any)
	if err := gconfig_v2.UnmarshalFile(ctx, "config5.yaml", &m); err != nil {
		t.Fatal(err)
	}
	if _, ok := m["testlist"]; !ok {
		t.Fatalf("testlist not found: %v", m)
	}
}

func TestUnmarshalScalarSlice(t *testing.T) {
	ctx := context.Background()
	m := make(map[string]any)
	if err := gconfig_v2.UnmarshalFile(ctx, "config3.yaml", &m); err != nil {
		t.Fatal(err)
	}
	list := requireSlice(t, m["testList"])
	want := []any{"test1", "test2", "test3"}
	if len(list) != len(want) {
		t.Fatalf("len(testList) = %d", len(list))
	}
	for i := range want {
		if list[i] != want[i] {
			t.Fatalf("testList[%d] = %v", i, list[i])
		}
	}
}

func TestUnmarshalNamedSliceMaps(t *testing.T) {
	ctx := context.Background()
	m := make(map[string]any)
	if err := gconfig_v2.UnmarshalFile(ctx, "config4.yaml", &m); err != nil {
		t.Fatal(err)
	}
	list := requireSlice(t, m["testlist"])
	if len(list) != 2 {
		t.Fatalf("len(testlist) = %d", len(list))
	}
	first := requireMap(t, list[0])
	t1 := requireMap(t, first["t1"])
	if t1["tt1"] != 1 || t1["tt2"] != 3 {
		t.Fatalf("t1 = %v", t1)
	}
	second := requireMap(t, list[1])
	t2 := requireMap(t, second["t2"])
	if t2["tt1"] != 4 || t2["tt5"] != 5 {
		t.Fatalf("t2 = %v", t2)
	}
}

func TestUnmarshalComplexConfig(t *testing.T) {
	ctx := context.Background()
	m := make(map[string]any)
	if err := gconfig_v2.UnmarshalFile(ctx, "config_complex.yaml", &m); err != nil {
		t.Fatal(err)
	}

	app := requireMap(t, m["app"])
	if app["name"] != "payment-api" || app["enabled"] != true || app["version"] != 2.5 {
		t.Fatalf("app = %v", app)
	}
	if app["empty_value"] != nil {
		t.Fatalf("app.empty_value = %v", app["empty_value"])
	}
	launchDate := requireTime(t, app["launch_date"])
	if launchDate.Format("2006-01-02") != "2026-06-24" {
		t.Fatalf("app.launch_date = %v", launchDate)
	}
	tags := requireSlice(t, app["tags"])
	if len(tags) != 3 || tags[0] != "core" || tags[2] != "v2" {
		t.Fatalf("app.tags = %v", tags)
	}
	thresholds := requireMap(t, app["thresholds"])
	if thresholds["qps"] != 1200 || thresholds["ratio"] != 0.75 {
		t.Fatalf("app.thresholds = %v", thresholds)
	}
	timeout := requireTime(t, thresholds["timeout"])
	if !timeout.Equal(time.Date(2026, 6, 24, 8, 30, 0, 0, time.UTC)) {
		t.Fatalf("app.thresholds.timeout = %v", timeout)
	}

	database := requireMap(t, m["database"])
	primary := requireMap(t, database["primary"])
	if primary["host"] != "db.local" || primary["port"] != 3306 {
		t.Fatalf("database.primary = %v", primary)
	}
	flags := requireMap(t, primary["flags"])
	if flags["ssl"] != true || flags["pool"] != 20 || flags["mode"] != "rw" {
		t.Fatalf("database.primary.flags = %v", flags)
	}
	replicas := requireSlice(t, database["replicas"])
	if len(replicas) != 2 {
		t.Fatalf("len(database.replicas) = %d", len(replicas))
	}
	firstReplica := requireMap(t, replicas[0])
	secondReplica := requireMap(t, replicas[1])
	if firstReplica["name"] != "r1" || firstReplica["weight"] != 10 ||
		secondReplica["name"] != "r2" || secondReplica["weight"] != 20 {
		t.Fatalf("database.replicas = %v", replicas)
	}

	routes := requireSlice(t, m["routes"])
	if len(routes) != 2 {
		t.Fatalf("len(routes) = %d", len(routes))
	}
	loginRoute := requireMap(t, requireMap(t, routes[0])["login"])
	loginMethods := requireSlice(t, loginRoute["methods"])
	if loginRoute["path"] != "/login" || loginRoute["auth"] != true ||
		len(loginMethods) != 2 || loginMethods[0] != "GET" || loginMethods[1] != "POST" {
		t.Fatalf("routes[0].login = %v", loginRoute)
	}
	healthRoute := requireMap(t, requireMap(t, routes[1])["health"])
	healthMethods := requireSlice(t, healthRoute["methods"])
	if healthRoute["path"] != "/health" || healthRoute["auth"] != false ||
		len(healthMethods) != 1 || healthMethods[0] != "GET" {
		t.Fatalf("routes[1].health = %v", healthRoute)
	}
}

func TestUnmarshalScalarTypes(t *testing.T) {
	ctx := context.Background()
	m := make(map[string]any)
	if err := gconfig_v2.UnmarshalFile(ctx, "config_scalar_types.yaml", &m); err != nil {
		t.Fatal(err)
	}

	numbers := requireMap(t, m["numbers"])
	ints := requireSlice(t, numbers["ints"])
	if len(ints) != 3 || ints[0] != 1 || ints[1] != 2 || ints[2] != -3 {
		t.Fatalf("numbers.ints = %v", ints)
	}
	floats := requireSlice(t, numbers["floats"])
	if len(floats) != 3 || floats[0] != 1.25 || floats[1] != -2.5 || floats[2] != 300.0 {
		t.Fatalf("numbers.floats = %v", floats)
	}

	booleans := requireMap(t, m["booleans"])
	if booleans["yes_value"] != true || booleans["no_value"] != false ||
		booleans["on_value"] != true || booleans["off_value"] != false {
		t.Fatalf("booleans = %v", booleans)
	}

	nulls := requireMap(t, m["nulls"])
	if nulls["null_value"] != nil || nulls["tilde_value"] != nil {
		t.Fatalf("nulls = %v", nulls)
	}

	times := requireMap(t, m["times"])
	if !requireTime(t, times["rfc3339"]).Equal(time.Date(2026, 6, 24, 8, 30, 0, 0, time.UTC)) {
		t.Fatalf("times.rfc3339 = %v", times["rfc3339"])
	}
	if requireTime(t, times["space_time"]).Format("2006-01-02 15:04:05") != "2026-06-24 10:20:30" {
		t.Fatalf("times.space_time = %v", times["space_time"])
	}
	if requireTime(t, times["date_only"]).Format("2006-01-02") != "2026-06-24" {
		t.Fatalf("times.date_only = %v", times["date_only"])
	}

	durations := requireMap(t, m["durations"])
	if requireDuration(t, durations["connect_timeout"]) != time.Millisecond {
		t.Fatalf("durations.connect_timeout = %v", durations["connect_timeout"])
	}
	if requireDuration(t, durations["read_timeout"]) != 2400*time.Nanosecond {
		t.Fatalf("durations.read_timeout = %v", durations["read_timeout"])
	}
	if requireDuration(t, durations["retry_backoff"]) != 3*time.Second {
		t.Fatalf("durations.retry_backoff = %v", durations["retry_backoff"])
	}
	durationList := requireSlice(t, durations["batch"])
	if requireDuration(t, durationList[0]) != time.Millisecond ||
		requireDuration(t, durationList[1]) != 2400*time.Nanosecond ||
		requireDuration(t, durationList[2]) != 3*time.Second {
		t.Fatalf("durations.batch = %v", durationList)
	}
	durationMap := requireMap(t, durations["inline_map"])
	if requireDuration(t, durationMap["connect"]) != time.Millisecond ||
		requireDuration(t, durationMap["read"]) != 2400*time.Nanosecond {
		t.Fatalf("durations.inline_map = %v", durationMap)
	}

	explicitStrings := requireMap(t, m["explicit_strings"])
	if explicitStrings["bool_text"] != "on" ||
		explicitStrings["duration_text"] != "1ms" ||
		explicitStrings["number_text"] != "123" ||
		explicitStrings["quoted_text"] != "host:3306" ||
		explicitStrings["empty_text"] != "" {
		t.Fatalf("explicit_strings = %v", explicitStrings)
	}
	explicitStringMap := requireMap(t, explicitStrings["inline_map"])
	if explicitStringMap["flag"] != "on" || explicitStringMap["duration"] != "1ms" {
		t.Fatalf("explicit_strings.inline_map = %v", explicitStringMap)
	}
	explicitStringList := requireSlice(t, explicitStrings["inline_list"])
	if len(explicitStringList) != 2 || explicitStringList[0] != "on" || explicitStringList[1] != "1ms" {
		t.Fatalf("explicit_strings.inline_list = %v", explicitStringList)
	}

	multiline := requireMap(t, m["multiline"])
	if multiline["literal"] != "select *\nfrom users\nwhere id = ?\n" {
		t.Fatalf("multiline.literal = %q", multiline["literal"])
	}
	if multiline["literal_strip"] != "no trailing newline" {
		t.Fatalf("multiline.literal_strip = %q", multiline["literal_strip"])
	}
	if multiline["folded"] != "hello world\n" {
		t.Fatalf("multiline.folded = %q", multiline["folded"])
	}
	if multiline["tagged_literal"] != "on\n1ms\n" {
		t.Fatalf("multiline.tagged_literal = %q", multiline["tagged_literal"])
	}
	multilineList := requireSlice(t, multiline["list"])
	if len(multilineList) != 2 ||
		multilineList[0] != "item line 1\nitem line 2\n" ||
		multilineList[1] != "folded item\n" {
		t.Fatalf("multiline.list = %v", multilineList)
	}
	namedMultilineList := requireSlice(t, multiline["named_list"])
	if len(namedMultilineList) != 1 {
		t.Fatalf("len(multiline.named_list) = %d", len(namedMultilineList))
	}
	namedScript := requireMap(t, namedMultilineList[0])
	if namedScript["script"] != "echo hi\nexit 0\n" {
		t.Fatalf("multiline.named_list[0].script = %q", namedScript["script"])
	}

	stringsMap := requireMap(t, m["strings"])
	if stringsMap["quoted_colon"] != "host:3306" ||
		stringsMap["single_quote"] != "it's ok" ||
		stringsMap["plain_version"] != "1.0.0" {
		t.Fatalf("strings = %v", stringsMap)
	}
}

func TestUnmarshalBytes(t *testing.T) {
	ctx := context.Background()
	content := []byte(`
name: bytes-config
enabled: true
timeout: 1ms
server:
  host: 127.0.0.1
  port: 8080
script: |
  echo hi
  exit 0
`)

	m := make(map[string]any)
	if err := gconfig_v2.Unmarshal(ctx, content, &m); err != nil {
		t.Fatal(err)
	}
	if m["name"] != "bytes-config" || m["enabled"] != true {
		t.Fatalf("m = %v", m)
	}
	if requireDuration(t, m["timeout"]) != time.Millisecond {
		t.Fatalf("timeout = %v", m["timeout"])
	}
	server := requireMap(t, m["server"])
	if server["host"] != "127.0.0.1" || server["port"] != 8080 {
		t.Fatalf("server = %v", server)
	}
	if m["script"] != "echo hi\nexit 0\n" {
		t.Fatalf("script = %q", m["script"])
	}

	type Server struct {
		Host string `yaml:"host"`
		Port int    `yaml:"port"`
	}
	type Config struct {
		Name    string        `yaml:"name"`
		Enabled bool          `yaml:"enabled"`
		Timeout time.Duration `yaml:"timeout"`
		Server  Server        `yaml:"server"`
		Script  string        `yaml:"script"`
	}

	var cfg Config
	if err := gconfig_v2.Unmarshal(ctx, content, &cfg); err != nil {
		t.Fatal(err)
	}
	if cfg.Name != "bytes-config" ||
		!cfg.Enabled ||
		cfg.Timeout != time.Millisecond ||
		cfg.Server.Host != "127.0.0.1" ||
		cfg.Server.Port != 8080 ||
		cfg.Script != "echo hi\nexit 0\n" {
		t.Fatalf("cfg = %+v", cfg)
	}
}

func TestUnmarshalStruct(t *testing.T) {
	type Thresholds struct {
		QPS     int       `yaml:"qps"`
		Ratio   float64   `yaml:"ratio"`
		Timeout time.Time `yaml:"timeout"`
	}
	type App struct {
		Name       string     `yaml:"name"`
		Enabled    bool       `yaml:"enabled"`
		Version    float64    `yaml:"version"`
		LaunchDate time.Time  `yaml:"launch_date"`
		Tags       []string   `yaml:"tags"`
		Thresholds Thresholds `yaml:"thresholds"`
	}
	type Flags struct {
		SSL  bool   `yaml:"ssl"`
		Pool int    `yaml:"pool"`
		Mode string `yaml:"mode"`
	}
	type Primary struct {
		Host  string `yaml:"host"`
		Port  int    `yaml:"port"`
		Flags Flags  `yaml:"flags"`
	}
	type Replica struct {
		Name   string `yaml:"name"`
		Host   string `yaml:"host"`
		Weight int    `yaml:"weight"`
	}
	type Database struct {
		Primary  *Primary  `yaml:"primary"`
		Replicas []Replica `yaml:"replicas"`
	}
	type Config struct {
		App      App      `yaml:"app"`
		Database Database `yaml:"database"`
	}

	ctx := context.Background()
	var cfg Config
	if err := gconfig_v2.UnmarshalFile(ctx, "config_complex.yaml", &cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.App.Name != "payment-api" || !cfg.App.Enabled || cfg.App.Version != 2.5 {
		t.Fatalf("cfg.App = %+v", cfg.App)
	}
	if cfg.App.LaunchDate.Format("2006-01-02") != "2026-06-24" {
		t.Fatalf("cfg.App.LaunchDate = %v", cfg.App.LaunchDate)
	}
	if len(cfg.App.Tags) != 3 || cfg.App.Tags[0] != "core" || cfg.App.Tags[2] != "v2" {
		t.Fatalf("cfg.App.Tags = %v", cfg.App.Tags)
	}
	if cfg.App.Thresholds.QPS != 1200 ||
		cfg.App.Thresholds.Ratio != 0.75 ||
		!cfg.App.Thresholds.Timeout.Equal(time.Date(2026, 6, 24, 8, 30, 0, 0, time.UTC)) {
		t.Fatalf("cfg.App.Thresholds = %+v", cfg.App.Thresholds)
	}
	if cfg.Database.Primary == nil {
		t.Fatal("cfg.Database.Primary is nil")
	}
	if cfg.Database.Primary.Host != "db.local" ||
		cfg.Database.Primary.Port != 3306 ||
		cfg.Database.Primary.Flags.SSL != true ||
		cfg.Database.Primary.Flags.Pool != 20 ||
		cfg.Database.Primary.Flags.Mode != "rw" {
		t.Fatalf("cfg.Database.Primary = %+v", cfg.Database.Primary)
	}
	if len(cfg.Database.Replicas) != 2 ||
		cfg.Database.Replicas[0].Name != "r1" ||
		cfg.Database.Replicas[0].Weight != 10 ||
		cfg.Database.Replicas[1].Name != "r2" ||
		cfg.Database.Replicas[1].Weight != 20 {
		t.Fatalf("cfg.Database.Replicas = %+v", cfg.Database.Replicas)
	}
}

func TestUnmarshalStructScalarTypes(t *testing.T) {
	type Durations struct {
		ConnectTimeout time.Duration            `yaml:"connect_timeout"`
		ReadTimeout    time.Duration            `yaml:"read_timeout"`
		RetryBackoff   time.Duration            `yaml:"retry_backoff"`
		Batch          []time.Duration          `yaml:"batch"`
		InlineMap      map[string]time.Duration `yaml:"inline_map"`
	}
	type ExplicitStrings struct {
		BoolText     string            `yaml:"bool_text"`
		DurationText string            `yaml:"duration_text"`
		NumberText   string            `yaml:"number_text"`
		QuotedText   string            `yaml:"quoted_text"`
		EmptyText    string            `yaml:"empty_text"`
		InlineMap    map[string]string `yaml:"inline_map"`
		InlineList   []string          `yaml:"inline_list"`
	}
	type NamedScript struct {
		Script string `yaml:"script"`
	}
	type Multiline struct {
		Literal      string        `yaml:"literal"`
		LiteralStrip string        `yaml:"literal_strip"`
		Folded       string        `yaml:"folded"`
		NamedList    []NamedScript `yaml:"named_list"`
	}
	type Config struct {
		Durations       Durations       `yaml:"durations"`
		ExplicitStrings ExplicitStrings `yaml:"explicit_strings"`
		Multiline       Multiline       `yaml:"multiline"`
	}

	ctx := context.Background()
	var cfg Config
	if err := gconfig_v2.UnmarshalFile(ctx, "config_scalar_types.yaml", &cfg); err != nil {
		t.Fatal(err)
	}

	if cfg.Durations.ConnectTimeout != time.Millisecond ||
		cfg.Durations.ReadTimeout != 2400*time.Nanosecond ||
		cfg.Durations.RetryBackoff != 3*time.Second {
		t.Fatalf("cfg.Durations = %+v", cfg.Durations)
	}
	if len(cfg.Durations.Batch) != 3 ||
		cfg.Durations.Batch[0] != time.Millisecond ||
		cfg.Durations.Batch[1] != 2400*time.Nanosecond ||
		cfg.Durations.Batch[2] != 3*time.Second {
		t.Fatalf("cfg.Durations.Batch = %v", cfg.Durations.Batch)
	}
	if cfg.Durations.InlineMap["connect"] != time.Millisecond ||
		cfg.Durations.InlineMap["read"] != 2400*time.Nanosecond {
		t.Fatalf("cfg.Durations.InlineMap = %v", cfg.Durations.InlineMap)
	}
	if cfg.ExplicitStrings.BoolText != "on" ||
		cfg.ExplicitStrings.DurationText != "1ms" ||
		cfg.ExplicitStrings.NumberText != "123" ||
		cfg.ExplicitStrings.QuotedText != "host:3306" ||
		cfg.ExplicitStrings.EmptyText != "" {
		t.Fatalf("cfg.ExplicitStrings = %+v", cfg.ExplicitStrings)
	}
	if cfg.ExplicitStrings.InlineMap["flag"] != "on" ||
		cfg.ExplicitStrings.InlineMap["duration"] != "1ms" ||
		len(cfg.ExplicitStrings.InlineList) != 2 ||
		cfg.ExplicitStrings.InlineList[0] != "on" ||
		cfg.ExplicitStrings.InlineList[1] != "1ms" {
		t.Fatalf("cfg.ExplicitStrings inline values = %+v", cfg.ExplicitStrings)
	}
	if cfg.Multiline.Literal != "select *\nfrom users\nwhere id = ?\n" ||
		cfg.Multiline.LiteralStrip != "no trailing newline" ||
		cfg.Multiline.Folded != "hello world\n" {
		t.Fatalf("cfg.Multiline = %+v", cfg.Multiline)
	}
	if len(cfg.Multiline.NamedList) != 1 ||
		cfg.Multiline.NamedList[0].Script != "echo hi\nexit 0\n" {
		t.Fatalf("cfg.Multiline.NamedList = %+v", cfg.Multiline.NamedList)
	}
}
