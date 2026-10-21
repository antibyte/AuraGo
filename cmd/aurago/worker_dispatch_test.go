package main

import (
	"bytes"
	"strings"
	"testing"
)

func TestRunIsolatedWorkerDispatchesWorkerModes(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, handled := runIsolatedWorker([]string{"aurago", "--person-detect-worker"}, strings.NewReader(""), &stdout, &stderr)
	if !handled || code != 2 || !strings.Contains(stderr.String(), "usage: aurago --person-detect-worker") {
		t.Fatalf("person-detect-worker without subcommand: handled %v, code %d, stderr %q", handled, code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code, handled = runIsolatedWorker([]string{"aurago", "--person-detect-worker", "serve"}, strings.NewReader(""), &stdout, &stderr)
	if !handled || code != 2 || !strings.Contains(stdout.String(), `"type":"hello"`) || !strings.Contains(stdout.String(), "--runtime and --model are required") {
		t.Fatalf("serve without flags: handled %v, code %d, stdout %q", handled, code, stdout.String())
	}

	stdout.Reset()
	stderr.Reset()
	code, handled = runIsolatedWorker([]string{"aurago", "--embedding-worker"}, strings.NewReader(""), &stdout, &stderr)
	if !handled || code != 2 || !strings.Contains(stderr.String(), "runtime, model, and tokenizer are required") {
		t.Fatalf("embedding-worker without flags: handled %v, code %d, stderr %q", handled, code, stderr.String())
	}

	// Near misses of the worker flags stay with the normal start-up (the flag
	// must match exactly; single-dash and "=value" spellings are not workers).
	for _, args := range [][]string{
		{"aurago"},
		{"aurago", "-config", "config.yaml"},
		{"aurago", "-person-detect-worker"},
		{"aurago", "--person-detect-worker=serve"},
		{"aurago", "-embedding-worker"},
		{"aurago", "serve", "--person-detect-worker"},
	} {
		if _, handled := runIsolatedWorker(args, strings.NewReader(""), &stdout, &stderr); handled {
			t.Fatalf("runIsolatedWorker handled %v", args)
		}
	}
}

func TestRunIsolatedWorkerDispatchesPixelMatteWorker(t *testing.T) {
	var stdout, stderr bytes.Buffer
	code, handled := runIsolatedWorker([]string{"aurago", "--pixel-matte-worker"}, strings.NewReader(""), &stdout, &stderr)
	if !handled || code != 2 || !strings.Contains(stderr.String(), "usage: aurago --pixel-matte-worker run") {
		t.Fatalf("pixel-matte-worker without subcommand: handled %v, code %d, stderr %q", handled, code, stderr.String())
	}

	stdout.Reset()
	stderr.Reset()
	code, handled = runIsolatedWorker([]string{"aurago", "--pixel-matte-worker", "run"}, strings.NewReader("not read"), &stdout, &stderr)
	if !handled || code != 2 || stdout.Len() != 0 || !strings.Contains(stderr.String(), "--runtime and --model are required") {
		t.Fatalf("run without flags: handled %v, code %d, stdout %q, stderr %q", handled, code, stdout.String(), stderr.String())
	}

	for _, args := range [][]string{
		{"aurago", "-pixel-matte-worker"},
		{"aurago", "--pixel-matte-worker=run"},
		{"aurago", "run", "--pixel-matte-worker"},
	} {
		if _, handled := runIsolatedWorker(args, strings.NewReader(""), &stdout, &stderr); handled {
			t.Fatalf("runIsolatedWorker handled %v", args)
		}
	}
}
