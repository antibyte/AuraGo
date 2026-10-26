package agent

import (
	"encoding/json"
	"encoding/xml"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"aurago/internal/selftest"
	"github.com/BurntSushi/toml"
	"gopkg.in/yaml.v3"
)

type selfTestFixture struct {
	dir      string
	root     *os.Root
	name     string
	expected string
	verified bool
}

func newSelfTestFixture(dc *DispatchContext, test selftest.Case) (*selfTestFixture, error) {
	supported := map[string]string{"FILE-01": "marker.txt", "FILE-02": "marker.txt", "FILE-03": "marker.txt", "FILE-04": "marker.txt", "FILE-05": "marker.txt", "FILE-06": "marker.txt", "FILE-08": "marker.txt", "FILE-10": "fixture.json", "FILE-11": "fixture.yaml", "FILE-12": "fixture.toml", "FILE-13": "fixture.xml"}
	name, ok := supported[test.TemplateID]
	if !ok {
		return nil, nil
	}
	if test.Profile == "W" && !dc.Cfg.Agent.AllowFilesystemWrite {
		return nil, fmt.Errorf("filesystem writes disabled")
	}
	workspace := dc.Cfg.Directories.WorkspaceDir
	if workspace == "" {
		return nil, fmt.Errorf("workspace unavailable")
	}
	dir, err := os.MkdirTemp(workspace, "_aurago-selftest-")
	if err != nil {
		return nil, err
	}
	dir, err = filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	root, err := os.OpenRoot(dir)
	if err != nil {
		_ = os.Remove(dir)
		return nil, err
	}
	f := &selfTestFixture{dir: dir, root: root, name: name, expected: "AURAGO_SELFTEST_äöü\nmarker=neu\nkeep=false\n"}
	content := "AURAGO_SELFTEST_äöü\nmarker=alt\nkeep=false\n"
	switch test.TemplateID {
	case "FILE-01":
		content = ""
		f.name = "marker.txt"
	case "FILE-10":
		content = `{"value":7,"keep":false,"empty":null,"label":"äöü"}`
		f.expected = `{"value":42,"keep":false,"empty":null,"label":"äöü"}`
	case "FILE-11":
		content = "value: 7\nkeep: false\nlabel: äöü\n"
		f.expected = "value: 42\nkeep: false\nlabel: äöü\n"
	case "FILE-12":
		content = "value = 7\nkeep = false\nlabel = \"äöü\"\n"
		f.expected = "value = 42\nkeep = false\nlabel = \"äöü\"\n"
	case "FILE-13":
		content = "<root><value>7</value><keep>false</keep><label>äöü</label></root>"
		f.expected = "<root><value>42</value><keep>false</keep><label>äöü</label></root>"
	}
	if test.TemplateID != "FILE-01" {
		if err := root.WriteFile(name, []byte(content), 0600); err != nil {
			_ = f.close()
			return nil, err
		}
	}
	return f, nil
}

func (f *selfTestFixture) binding() string {
	return fmt.Sprintf("\nIsoliertes lokales Fixture: Ordner %q, einzige Testdatei %q. Textmarker AURAGO_SELFTEST. Textänderung ausschließlich marker=alt → marker=neu; neue Textdatei erhält exakt %q. Strukturierte Datei: ausschließlich value von 7 auf 42 ändern, alle anderen Werte erhalten. Prüfe danach mit einem erneuten Leseaufruf. Die Laufzeit überprüft und bereinigt nur dieses Fixture automatisch. Keine anderen Dateien oder Verzeichnisse verwenden.", f.dir, filepath.Join(f.dir, f.name), "AURAGO_SELFTEST_äöü\nmarker=neu\nkeep=false\n")
}

func (f *selfTestFixture) allowed(call ToolCall) bool {
	if call.Action != "filesystem" && call.Action != "file_editor" && call.Action != "file_reader_advanced" && call.Action != "smart_file_read" && call.Action != "file_search" && call.Action != "detect_file_type" && call.Action != "json_editor" && call.Action != "yaml_editor" && call.Action != "toml_editor" && call.Action != "xml_editor" {
		return false
	}
	pathFound := false
	for key, value := range call.Params {
		if key != "path" && key != "file_path" && key != "directory" && key != "root" && key != "destination" && key != "source" {
			continue
		}
		s, ok := value.(string)
		if !ok || s == "" {
			continue
		}
		pathFound = true
		if !filepath.IsAbs(s) {
			return false
		}
		clean := filepath.Clean(s)
		if clean == f.dir {
			op := toolCallSnapshotOperation(call)
			if op != "list_dir" && op != "stat" && call.Action != "file_search" {
				return false
			}
			continue
		}
		if clean != filepath.Join(f.dir, f.name) {
			return false
		}
		if info, err := os.Lstat(clean); err == nil && info.Mode()&os.ModeSymlink != 0 {
			return false
		}
		actual, err := filepath.EvalSymlinks(filepath.Dir(clean))
		if err != nil || actual != f.dir {
			return false
		}
	}
	return pathFound
}

func (f *selfTestFixture) check(test selftest.Case, output string) {
	f.verified = false
	b, err := f.root.ReadFile(f.name)
	if err != nil {
		return
	}
	if test.Profile == "R" {
		if strings.Contains(output, "AURAGO_SELFTEST") || test.Tool == "detect_file_type" {
			f.verified = true
		}
		return
	}
	if strings.HasSuffix(f.name, ".txt") {
		if string(b) == f.expected {
			f.verified = true
		}
		return
	}
	actual, err := selfTestStructuredFile(f.name, b)
	expected, expectedErr := selfTestStructuredFile(f.name, []byte(f.expected))
	f.verified = err == nil && expectedErr == nil && selfTestHash(actual) == selfTestHash(expected)
}

func selfTestStructuredFile(name string, data []byte) (any, error) {
	var value any
	var err error
	switch filepath.Ext(name) {
	case ".json":
		err = json.Unmarshal(data, &value)
	case ".yaml":
		err = yaml.Unmarshal(data, &value)
	case ".toml":
		var object map[string]any
		err = toml.Unmarshal(data, &object)
		value = object
	case ".xml":
		// Compare the complete token stream, including attributes and extra fields.
		decoder := xml.NewDecoder(strings.NewReader(string(data)))
		var tokens []any
		for {
			token, tokenErr := decoder.Token()
			if tokenErr != nil {
				if tokenErr == io.EOF {
					return tokens, nil
				}
				return nil, tokenErr
			}
			if text, ok := token.(xml.CharData); ok && strings.TrimSpace(string(text)) == "" {
				continue
			}
			tokens = append(tokens, xml.CopyToken(token))
		}
	default:
		err = fmt.Errorf("unsupported fixture format")
	}
	return value, err
}

func (f *selfTestFixture) close() error {
	// The root was created by MkdirTemp inside the authorized workspace; rooted
	// deletion never follows a subsequently planted link outside that directory.
	entries, err := os.ReadDir(f.dir)
	if err != nil {
		_ = f.root.Close()
		if os.IsNotExist(err) {
			return nil
		}
		return err
	}
	for _, entry := range entries {
		if err := f.root.RemoveAll(entry.Name()); err != nil {
			_ = f.root.Close()
			return err
		}
	}
	if err := f.root.Close(); err != nil {
		return err
	}
	return os.Remove(f.dir)
}
