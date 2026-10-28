package dockerhost

import (
	"bytes"
	"strings"
	"testing"
)

func TestDemuxStreamsSplitsStdoutAndStderr(t *testing.T) {
	var frames bytes.Buffer
	frames.Write(muxFrame(1, "out one\n"))
	frames.Write(muxFrame(2, "err one\n"))
	frames.Write(muxFrame(0, "stdin frame\n")) // stream 0 is delivered with stdout, like Docker's StdCopy
	frames.Write(muxFrame(1, "out two\n"))

	var stdout, stderr bytes.Buffer
	if err := DemuxStreams(bytes.NewReader(frames.Bytes()), &stdout, &stderr); err != nil {
		t.Fatalf("DemuxStreams: %v", err)
	}
	if got, want := stdout.String(), "out one\nstdin frame\nout two\n"; got != want {
		t.Fatalf("stdout = %q, want %q", got, want)
	}
	if got, want := stderr.String(), "err one\n"; got != want {
		t.Fatalf("stderr = %q, want %q", got, want)
	}
}

func TestDemuxStreamsDiscardsAStreamWithoutAWriter(t *testing.T) {
	var frames bytes.Buffer
	frames.Write(muxFrame(1, "out\n"))
	frames.Write(muxFrame(2, "err\n"))

	var stdout bytes.Buffer
	if err := DemuxStreams(bytes.NewReader(frames.Bytes()), &stdout, nil); err != nil {
		t.Fatalf("DemuxStreams: %v", err)
	}
	if stdout.String() != "out\n" {
		t.Fatalf("stdout = %q, want only the stdout frames", stdout.String())
	}
	stdout.Reset()
	var stderr bytes.Buffer
	if err := DemuxStreams(bytes.NewReader(frames.Bytes()), nil, &stderr); err != nil {
		t.Fatalf("DemuxStreams: %v", err)
	}
	if stderr.String() != "err\n" || stdout.Len() != 0 {
		t.Fatalf("stderr = %q, want only the stderr frames", stderr.String())
	}
	if err := DemuxStreams(bytes.NewReader(frames.Bytes()), nil, nil); err != nil {
		t.Fatalf("DemuxStreams with no writers: %v", err)
	}
}

func TestDemuxStreamsCopiesAPlainStreamToStdout(t *testing.T) {
	text := "already demultiplexed by a proxy, longer than one header\n"
	var stdout, stderr bytes.Buffer
	if err := DemuxStreams(strings.NewReader(text), &stdout, &stderr); err != nil {
		t.Fatalf("DemuxStreams: %v", err)
	}
	if stdout.String() != text || stderr.Len() != 0 {
		t.Fatalf("stdout = %q stderr = %q, want the text verbatim on stdout", stdout.String(), stderr.String())
	}
}

func TestDemuxLogsStillMergesBothStreams(t *testing.T) {
	var frames bytes.Buffer
	frames.Write(muxFrame(1, "a\n"))
	frames.Write(muxFrame(2, "b\n"))
	var merged bytes.Buffer
	if err := demuxLogs(bytes.NewReader(frames.Bytes()), &merged); err != nil {
		t.Fatalf("demuxLogs: %v", err)
	}
	if merged.String() != "a\nb\n" {
		t.Fatalf("merged = %q, want arrival order a, b", merged.String())
	}
}

func TestStoreDBIsTheSharedHandle(t *testing.T) {
	s, _ := openTestStore(t)
	if s.DB() == nil {
		t.Fatal("DB() = nil")
	}
	if _, err := s.DB().Exec("CREATE TABLE IF NOT EXISTS docker_probe (id TEXT PRIMARY KEY)"); err != nil {
		t.Fatalf("exec on the shared handle: %v", err)
	}
	var n int
	if err := s.DB().QueryRow("SELECT count(*) FROM docker_hosts").Scan(&n); err != nil {
		t.Fatalf("the shared handle must see docker_hosts: %v", err)
	}
}
