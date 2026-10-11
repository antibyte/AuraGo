package persondetect

import (
	"bytes"
	"testing"
)

// FuzzPatchVectorMatMuls checks the invariants on arbitrary bytes: no panic,
// the input is never modified, an error returns no model, count 0 returns the
// input unchanged, and patching a patched model changes nothing.
func FuzzPatchVectorMatMuls(f *testing.F) {
	nodes, initializers := integralChain("integral", "w", "Reshape", false)
	patchable := testModel(nodes, append(initializers, testShapeTensor), []string{"integral_x"}, []string{"integral_y"}, []string{"integral_m", "keep"})
	refusedNodes, refusedInitializers := integralChain("integral", "w", "Add", false)
	refused := testModel(refusedNodes, append(refusedInitializers, testShapeTensor), nil, []string{"integral_y"}, nil)
	f.Add(patchable)
	f.Add(refused)
	f.Add(patchable[:len(patchable)/2])
	f.Add([]byte{0x0a, 0x05, 0x01})
	f.Fuzz(func(t *testing.T, model []byte) {
		original := bytes.Clone(model)
		patched, count, err := PatchVectorMatMuls(model)
		if !bytes.Equal(model, original) {
			t.Fatal("input modified")
		}
		if err != nil {
			if patched != nil || count != 0 {
				t.Fatalf("error %v with count %d and %d bytes", err, count, len(patched))
			}
			return
		}
		if count < 0 {
			t.Fatalf("count = %d", count)
		}
		if count == 0 {
			if !bytes.Equal(patched, model) {
				t.Fatal("count 0 but the model changed")
			}
			return
		}
		again, againCount, againErr := PatchVectorMatMuls(patched)
		if againErr != nil || againCount != 0 || !bytes.Equal(again, patched) {
			t.Fatalf("re-patch: count %d, err %v, unchanged %v", againCount, againErr, bytes.Equal(again, patched))
		}
	})
}
