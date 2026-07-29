//go:build windows

package localllm

type fileStat struct{ groupID string }

func statFile(string) (fileStat, error) {
	return fileStat{}, nil
}
