package main
import (
  "fmt"
  "sort"
  "aurago/internal/agent"
  "reflect"
)
func main() {
  ff := agent.ToolFeatureFlags{}
  v := reflect.ValueOf(ff)
  t := reflect.TypeOf(ff)
  for i := 0; i < t.NumField(); i++ {
    v.Field(i).SetBool(true)
  }
  // can't set unexported - use test approach
}
