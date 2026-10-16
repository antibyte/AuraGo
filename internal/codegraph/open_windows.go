package codegraph

import "os"

func openSource(root *os.Root, name string) (*os.File, error) { return root.Open(name) }
