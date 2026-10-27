package dockerstacks

import (
	"encoding/json"
	"fmt"
	"reflect"
	"strings"
	"testing"
	"time"
)

// diagKeys lists the diagnostics as "line:code" in result order.
func diagKeys(res LintResult) []string {
	out := []string{}
	for _, d := range res.Diagnostics {
		out = append(out, fmt.Sprintf("%d:%s", d.Line, d.Code))
	}
	return out
}

func findDiag(t *testing.T, res LintResult, code string, line int) Diagnostic {
	t.Helper()
	for _, d := range res.Diagnostics {
		if d.Code == code && d.Line == line {
			return d
		}
	}
	t.Fatalf("no %s diagnostic at line %d in %v", code, line, diagKeys(res))
	return Diagnostic{}
}

const cleanCompose = `services:
  web:
    image: nginx:1.27
    restart: unless-stopped
`

func TestLintDiagnostics(t *testing.T) {
	cases := []struct {
		name    string
		compose string
		env     string
		want    []string
	}{
		{"clean file", cleanCompose, "", []string{}},
		{"empty file", "", "", []string{"0:no_services"}},
		{"comments only", "# nothing here\n", "", []string{"0:no_services"}},
		{"tab in the indentation", "services:\n\tweb:\n    image: x\n", "", []string{"2:yaml_syntax"}},
		{"unclosed quote", "services:\n  web:\n    image: \"nginx\n", "", []string{"3:yaml_syntax"}},
		{"root is a list", "- a\n- b\n", "", []string{"1:not_a_mapping"}},
		{"no services key", "name: demo\n", "", []string{"0:no_services"}},
		{"services is empty", "services:\n", "", []string{"1:no_services"}},
		{"services is an empty mapping", "services: {}\n", "", []string{"1:no_services"}},
		{"services is a list", "services:\n  - web\n", "", []string{"2:not_a_mapping"}},
		{"service is null", "services:\n  web:\n", "", []string{"2:not_a_mapping"}},
		{"neither image nor build", "services:\n  web:\n    restart: always\n", "", []string{"2:service_no_image"}},
		{"build is enough", "services:\n  web:\n    build: .\n    restart: always\n", "", []string{}},
		{"extends is enough", "services:\n  web:\n    extends:\n      file: base.yaml\n      service: web\n", "", []string{}},
		{"errors sort before hints on one line", "services:\n  web:\n    cpus: 1\n", "", []string{"2:service_no_image", "2:restart_missing"}},
		{"restart policy under deploy counts", "services:\n  web:\n    image: nginx:1.27\n    deploy:\n      restart_policy:\n        condition: any\n", "", []string{}},
		{"hints", `services:
  web:
    image: nginx:latest
    privileged: true
    network_mode: host
    cap_add:
      - NET_ADMIN
    volumes:
      - /var/run/docker.sock:/var/run/docker.sock
`, "", []string{"2:restart_missing", "3:image_latest", "4:privileged", "5:network_mode_host", "6:cap_add", "9:docker_socket"}},
		{"image tags", `services:
  a:
    image: nginx
    restart: always
  b:
    image: nginx:1.27
    restart: always
  c:
    image: ghcr.io/x/y@sha256:abc
    restart: always
  d:
    image: localhost:5000/app
    restart: always
  e:
    image: nginx:${TAG}
    restart: always
  f:
    image: app:latest
    build: .
    restart: always
`, "", []string{"3:image_no_tag", "12:image_no_tag", "15:variable_missing"}},
		{"invalid ports", `services:
  web:
    image: nginx:1.27
    restart: always
    ports:
      - "80:80:80:80"
      - abc
      - "70000:80"
      - target: 80
        published: 8080
      - published: 8081
      - 8082
`, "", []string{"6:invalid_port", "7:invalid_port", "8:invalid_port", "11:invalid_port"}},
		{"same host port twice", `services:
  a:
    image: nginx:1.27
    restart: always
    ports:
      - "8080:80"
  b:
    image: nginx:1.27
    restart: always
    ports:
      - "8080:81"
      - "127.0.0.1:9090:80"
      - "192.168.1.5:9090:80"
      - "8080:81/udp"
`, "", []string{"11:port_duplicate"}},
		{"wildcard and address overlap", "services:\n  web:\n    image: nginx:1.27\n    restart: always\n    ports:\n      - \"9090:80\"\n      - \"192.168.1.5:9090:81\"\n",
			"", []string{"7:port_duplicate"}},
		{"variables", `services:
  db:
    image: mysql:8
    restart: always
    environment:
      MYSQL_ROOT_PASSWORD: ${DB_ROOT}
      API_KEY: ${API_KEY:?set it}
      A: ${PRESENT}
      B: ${EMPTY}
      C: ${EMPTY:?must not be empty}
      D: ${OPT:-fallback}
      E: $$HOME and $${NOT_A_VAR}
      F: "${DB_ROOT} again"
    command: |
      echo ${IN_BLOCK}
# ${IN_COMMENT}
`, "PRESENT=1\nEMPTY=\n", []string{"6:variable_missing", "7:variable_missing", "10:variable_missing", "15:variable_missing"}},
		{"values from .env count as set", "services:\n  db:\n    image: mysql:8\n    restart: always\n    environment:\n      P: ${DB_ROOT}\n",
			"export DB_ROOT='s3cret'\n", []string{}},
		{"bad .env line", cleanCompose, "A=1\nnot valid\n", []string{"2:env_syntax"}},
		{"yaml error still reports the .env", "services:\n\tweb:\n", "bad\n", []string{"1:env_syntax", "2:yaml_syntax"}},
		{"anchors and merge keys", `x-base: &base
  image: nginx:1.27
  restart: always
services:
  a:
    <<: *base
  b:
    <<: *base
    ports:
      - "81:80"
`, "", []string{}},
		{"variables in ports and volumes are expanded", `services:
  web:
    image: nginx:1.27
    restart: always
    ports:
      - "${WEB_PORT:-8080}:80"
      - "${OTHER_PORT}:81"
    volumes:
      - ${DATA_DIR:-./data}:/data
`, "", []string{"7:variable_missing"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			res := Lint(LintInput{Compose: tc.compose, Env: tc.env}, nil)
			if got := diagKeys(res); !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("diagnostics = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestLintVariableFixes(t *testing.T) {
	compose := "services:\n  db:\n    image: mysql:8\n    restart: always\n    environment:\n      A: ${DB_ROOT}\n      B: ${API_KEY:?set it}\n      C: ${EMPTY:?x}\n"
	res := Lint(LintInput{Compose: compose, Env: "EMPTY=\n"}, nil)
	if d := findDiag(t, res, "variable_missing", 6); d.Severity != "warning" || d.Fix == nil || d.Fix.Kind != "env_random" || d.Fix.Variable != "DB_ROOT" || d.Params["variable"] != "DB_ROOT" {
		t.Fatalf("DB_ROOT diagnostic = %+v, want a warning with the env_random fix", d)
	}
	if d := findDiag(t, res, "variable_missing", 7); d.Fix != nil {
		t.Fatalf("a required variable must not offer a random value, got %+v", d.Fix)
	}
	if d := findDiag(t, res, "variable_missing", 8); d.Fix != nil {
		t.Fatalf("an empty required variable must not offer a random value, got %+v", d.Fix)
	}
	want := []VariablePreview{
		{Name: "DB_ROOT", Line: 6},
		{Name: "API_KEY", Line: 7, Required: true},
		{Name: "EMPTY", Line: 8, Required: true, InEnv: true},
	}
	if !reflect.DeepEqual(res.Preview.Variables, want) {
		t.Fatalf("variables = %+v, want %+v", res.Preview.Variables, want)
	}
}

func TestLintPortConflicts(t *testing.T) {
	published := []PublishedPort{
		{HostIP: "0.0.0.0", Port: 8080, Protocol: "tcp", Container: "grafana", Project: "mon"},
		{HostIP: "127.0.0.1", Port: 9090, Protocol: "tcp", Container: "prom", Project: "mon"},
		{Port: 5353, Protocol: "udp", Container: "dns"},
	}
	compose := `services:
  web:
    image: nginx:1.27
    restart: always
    ports:
      - "8080:80"
      - "9090:80"
      - "5353:53/udp"
      - "5353:53"
      - "192.168.1.5:9091:80"
`
	conflicts := func(res LintResult) []string {
		var out []string
		for _, p := range res.Preview.Services[0].Ports {
			out = append(out, p.Host+"="+p.Conflict)
		}
		return out
	}
	res := Lint(LintInput{Compose: compose}, published)
	if got, want := diagKeys(res), []string{"6:port_conflict", "7:port_conflict", "8:port_conflict"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("diagnostics = %v, want %v", got, want)
	}
	if got, want := conflicts(res), []string{"8080=grafana", "9090=prom", "5353=dns", "5353=", "9091="}; !reflect.DeepEqual(got, want) {
		t.Fatalf("conflicts = %v, want %v", got, want)
	}
	d := findDiag(t, res, "port_conflict", 6)
	if d.Severity != "warning" || d.Service != "web" || d.Params["container"] != "grafana" || d.Params["project"] != "mon" || d.Params["port"] != "8080" {
		t.Fatalf("conflict diagnostic = %+v", d)
	}

	own := Lint(LintInput{Compose: compose, Stack: "mon"}, published)
	if got, want := diagKeys(own), []string{"8:port_conflict"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("with Stack=mon the ports of project mon are not conflicts: diagnostics = %v, want %v", got, want)
	}
}

func TestLintPreview(t *testing.T) {
	compose := `services:
  db:
    image: postgres:16
    restart: always
    ports:
      - "127.0.0.1:5432:5432"
      - "8000-8002:8000-8002/udp"
      - 9000
    volumes:
      - pgdata:/var/lib/postgresql/data
      - ./init:/docker-entrypoint-initdb.d:ro
      - /srv/backup:/backup
      - /tmp
      - type: tmpfs
        target: /run
      - type: volume
        source: ext
        target: /ext
    tmpfs:
      - /scratch:size=64m
  app:
    build: .
    restart: always
volumes:
  pgdata:
  ext:
    external: true
  unused:
`
	res := Lint(LintInput{Compose: compose}, nil)
	if len(res.Diagnostics) != 0 {
		t.Fatalf("diagnostics = %v, want none", diagKeys(res))
	}
	want := Preview{
		Services: []ServicePreview{
			{
				Name: "db", Line: 2, Image: "postgres:16",
				Ports: []PortPreview{
					{HostIP: "127.0.0.1", Host: "5432", Container: "5432", Protocol: "tcp"},
					{Host: "8000-8002", Container: "8000-8002", Protocol: "udp"},
					{Container: "9000", Protocol: "tcp"},
				},
				Volumes: []VolumePreview{
					{Kind: "named", Source: "pgdata", Target: "/var/lib/postgresql/data"},
					{Kind: "relative", Source: "./init", Target: "/docker-entrypoint-initdb.d", ReadOnly: true},
					{Kind: "bind", Source: "/srv/backup", Target: "/backup"},
					{Kind: "anonymous", Target: "/tmp"},
					{Kind: "tmpfs", Target: "/run"},
					{Kind: "named", Source: "ext", Target: "/ext"},
					{Kind: "tmpfs", Target: "/scratch"},
				},
			},
			{Name: "app", Line: 21, Build: true, Ports: []PortPreview{}, Volumes: []VolumePreview{}},
		},
		NamedVolumes: []string{"pgdata"},
		Variables:    []VariablePreview{},
	}
	if !reflect.DeepEqual(res.Preview, want) {
		got, _ := json.MarshalIndent(res.Preview, "", " ")
		exp, _ := json.MarshalIndent(want, "", " ")
		t.Fatalf("preview =\n%s\nwant\n%s", got, exp)
	}
}

func TestLintJSONHasNoNullLists(t *testing.T) {
	raw, err := json.Marshal(Lint(LintInput{Compose: "services:\n  web:\n    image: nginx:1.27\n    restart: always\n"}, nil))
	if err != nil {
		t.Fatal(err)
	}
	text := string(raw)
	for _, want := range []string{`"diagnostics":[]`, `"named_volumes":[]`, `"variables":[]`, `"ports":[]`, `"volumes":[]`} {
		if !strings.Contains(text, want) {
			t.Fatalf("lint JSON %s lacks %s", text, want)
		}
	}
	if strings.Contains(text, "null") {
		t.Fatalf("lint JSON must not contain null lists: %s", text)
	}
	empty, _ := json.Marshal(Lint(LintInput{}, nil))
	if strings.Contains(string(empty), "null") {
		t.Fatalf("lint JSON of an empty file must not contain null lists: %s", empty)
	}
}

func TestLintSurvivesAMergeKeyBomb(t *testing.T) {
	var b strings.Builder
	b.WriteString("x0: &x0 {image: nginx:1.27}\n")
	for i := 1; i <= 12; i++ {
		fmt.Fprintf(&b, "x%d: &x%d {<<: [%s]}\n", i, i, strings.TrimSuffix(strings.Repeat(fmt.Sprintf("*x%d, ", i-1), 10), ", "))
	}
	b.WriteString("services:\n  web:\n    <<: *x12\n")
	start := time.Now()
	res := Lint(LintInput{Compose: b.String()}, nil)
	if elapsed := time.Since(start); elapsed > 5*time.Second {
		t.Fatalf("lint took %v on a merge-key bomb", elapsed)
	}
	if len(res.Preview.Services) != 1 {
		t.Fatalf("services = %+v, want the one service", res.Preview.Services)
	}
}
