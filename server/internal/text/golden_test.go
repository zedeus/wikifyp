package text

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

var update = flag.Bool("update", false, "rewrite golden files from current output")

// TestSplitGolden locks the splitter's output on a realistic encyclopedia
// paragraph. Addresses are stable forever, so any diff here means stored
// addresses would break: change the golden file only on purpose, with
// -update, and look hard at the diff.
func TestSplitGolden(t *testing.T) {
	input, err := os.ReadFile(filepath.Join("testdata", "paragraph.txt"))
	if err != nil {
		t.Fatal(err)
	}
	got := Split(strings.TrimSuffix(string(input), "\n"))

	goldenPath := filepath.Join("testdata", "paragraph.golden.json")
	if *update {
		data, err := json.MarshalIndent(got, "", "  ")
		if err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(goldenPath, append(data, '\n'), 0o644); err != nil {
			t.Fatal(err)
		}
	}

	data, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatal(err)
	}
	var want []Sentence
	if err := json.Unmarshal(data, &want); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(got, want) {
		gotJSON, _ := json.MarshalIndent(got, "", "  ")
		t.Errorf("Split output differs from %s:\n%s\n(run with -update only if the change is intended)",
			goldenPath, gotJSON)
	}
	checkInvariants(t, string(input))
}
