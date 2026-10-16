package ui

import (
	"encoding/json"
	"io/fs"
	"os/exec"
	"regexp"
	"strings"
	"testing"
)

var videoStudioLocales = []string{"cs", "da", "de", "el", "en", "es", "fr", "hi", "it", "ja", "nl", "no", "pl", "pt", "sv", "zh"}

// Every Video Studio string with a {{count}} has a `_one` singular in all sixteen locales, the Desktop plural
// convention (GalleryLibrary.countLabel), and the app reads count strings through trCount only.
func TestVideoStudioCountStringsHaveSingulars(t *testing.T) {
	for _, lang := range videoStudioLocales {
		var words map[string]string
		if err := json.Unmarshal([]byte(readDesktopAssetText(t, "lang/desktop/"+lang+".json")), &words); err != nil {
			t.Fatalf("parse %s: %v", lang, err)
		}
		for key, value := range words {
			if !strings.HasPrefix(key, "videoStudio.") || strings.HasSuffix(key, "_one") || !strings.Contains(value, "{{count}}") {
				continue
			}
			if one := words[key+"_one"]; strings.TrimSpace(one) == "" {
				t.Errorf("%s: %s has no %s_one singular", lang, key, key)
			}
		}
	}
	files, err := fs.Glob(Content, "js/desktop/apps/video-studio*.js")
	if err != nil || len(files) == 0 {
		t.Fatalf("no Video Studio scripts: %v", err)
	}
	direct := regexp.MustCompile(`\{\{count\}\}[^']*'\)\.replace\(|fill\(tr\(s, '[A-Za-z]+', '[^']*\{\{count\}\}`)
	for _, file := range files {
		if match := direct.FindString(readDesktopAssetText(t, file)); match != "" {
			t.Errorf("%s fills {{count}} without trCount: %s", file, match)
		}
	}
}

func TestVideoStudioTrCount(t *testing.T) {
	node, err := exec.LookPath("node")
	if err != nil {
		t.Skip("Node.js required for Video Studio count wording checks")
	}
	script := `
const assert = require('node:assert/strict');
global.window = {};
require('./js/desktop/apps/video-studio-time.js');
require('./js/desktop/apps/video-studio-timeline.js');
require('./js/desktop/apps/video-studio-core.js');
const P = window.VideoStudioParts;
const words = {'videoStudio.clips': '{{count}} clips in {{track}}', 'videoStudio.clips_one': 'One clip in {{track}}'};
const s = {ctx: {t: key => words[key] || key}};
assert.equal(P.trCount(s, 'clips', 1, '', '', {track: 'V1'}), 'One clip in V1');
assert.equal(P.trCount(s, 'clips', 0, '', '', {track: 'V1'}), '0 clips in V1', 'zero uses the plural');
assert.equal(P.trCount(s, 'clips', 2, '', '', {track: 'V1'}), '2 clips in V1');
assert.equal(P.trCount(s, 'missing', 1, '1 item', '{{count}} items'), '1 item', 'a missing singular falls back to the English one');
assert.equal(P.trCount(s, 'missing', 3, '1 item', '{{count}} items'), '3 items');
`
	cmd := exec.Command(node, "-e", script)
	cmd.Dir = "."
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("trCount checks failed: %v\n%s", err, out)
	}
}
