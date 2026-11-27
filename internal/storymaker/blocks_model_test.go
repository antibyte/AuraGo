package storymaker

import (
	"bytes"
	"reflect"
	"testing"
)

const blocksStorySrc = `{"schema":"storymaker/1","title":"B","format":{"aspect":"16:9","fps":30},"style":{"palette":"sunny","font":"default","captions":true},"cast":[],"scenes":[{"id":"s1","narration":"Anfrage Server {1}x","beats":[
 {"do":"show","diagram":{"layout":"flow","flow":true,"nodes":[{"id":"req","label":"Anfrage","icon":"icon.send","cue":1},{"id":"srv","label":"Server","points":["a","b"]}]},"at":"full","entry":"draw"},
 {"do":"show","icon":"icon.cpu","style":"card","text":"CPU","at":"board","entry":"draw"},
 {"do":"show","chart":"bar","data":{"labels":["Warten","Rechnen"],"values":[90,10],"unit":"%","title":"Zeit"},"at":"full","entry":"fade"}]}]}`

func TestDecodeStoryKeepsDiagrams(t *testing.T) {
	s := mustDecode(t, blocksStorySrc)
	first, err := CanonicalJSON(s)
	if err != nil {
		t.Fatal(err)
	}
	again, err := CanonicalJSON(mustDecode(t, string(first)))
	if err != nil || !bytes.Equal(first, again) {
		t.Fatalf("canonical form is not stable:\n%s\n%s", first, again)
	}
	b := s.Scenes[0].Beats
	if b[0].Diagram == nil || b[0].Diagram.Layout != "flow" || !b[0].Diagram.Flow || len(b[0].Diagram.Nodes) != 2 ||
		b[0].Diagram.Nodes[0].Cue != 1 || b[0].Diagram.Nodes[0].Icon != "icon.send" || len(b[0].Diagram.Nodes[1].Points) != 2 {
		t.Fatalf("diagram lost: %+v", b[0].Diagram)
	}
	if b[2].Data == nil || b[2].Data.Unit != "%" || b[2].Data.Title != "Zeit" {
		t.Fatalf("chart unit or title lost: %+v", b[2].Data)
	}
	if got, want := deriveShowIDs(&s.Scenes[0]), []string{"diagram-1", "icon-cpu-1", "chart-1"}; !reflect.DeepEqual(got, want) {
		t.Fatalf("show ids = %v, want %v", got, want)
	}
	if BlockAreas[len(BlockAreas)-1] != "board" || len(BlockAreas) != 6 {
		t.Fatalf("BlockAreas = %v", BlockAreas)
	}
}
