package collector

import (
	"fmt"
	"reflect"
	"testing"
)

func TestSelectDlogFilesUsesLatestFilenameDate(t *testing.T) {
	var files []string
	for i := 0; i < 48; i++ {
		files = append(files, fmt.Sprintf("1JDLM01_20260701_%03d_%06d.dlog", i, i))
	}
	files = append(files,
		"1JDLM01_20260809_000_000100.dlog",
		"1JDLM01_20260810_000_000101.dlog",
		"1JDLM01_20260811_000_000102.dlog",
	)

	selected, err := selectDlogFiles(files, "1JDLM01_20260811_000_000102.dlog", 1)
	if err != nil {
		t.Fatal(err)
	}

	want := []string{
		"1JDLM01_20260810_000_000101.dlog",
		"1JDLM01_20260811_000_000102.dlog",
	}
	if !reflect.DeepEqual(selected, want) {
		t.Fatalf("selectDlogFiles() = %#v, want %#v", selected, want)
	}
}

func TestSelectDlogFilesCopiesAllWhenFewerThan50(t *testing.T) {
	files := []string{
		"1JDLM01_20260806_000_000037.dlog",
		"1JDLM01_20260811_002_000043.dlog",
	}

	selected, err := selectDlogFiles(files, "1JDLM01_20260811_002_000043.dlog", 1)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(selected, files) {
		t.Fatalf("selectDlogFiles() = %#v, want %#v", selected, files)
	}
}

func TestSelectDlogFilesCopiesAllWhenDlogDaysIsZero(t *testing.T) {
	var files []string
	for i := 0; i < 50; i++ {
		files = append(files, fmt.Sprintf("1JDLM01_20260811_%03d_%06d.dlog", i, i))
	}

	selected, err := selectDlogFiles(files, files[len(files)-1], 0)
	if err != nil {
		t.Fatal(err)
	}

	if !reflect.DeepEqual(selected, files) {
		t.Fatalf("selectDlogFiles() = %#v, want %#v", selected, files)
	}
}

func TestSelectDlogFilesAlwaysIncludesCurrentLinkTarget(t *testing.T) {
	var files []string
	current := "1JDLM01_20260701_000_000001.dlog"
	files = append(files, current)
	for i := 0; i < 50; i++ {
		files = append(files, fmt.Sprintf("1JDLM01_20260811_%03d_%06d.dlog", i, i+10))
	}

	selected, err := selectDlogFiles(files, current, 1)
	if err != nil {
		t.Fatal(err)
	}

	found := false
	for _, name := range selected {
		if name == current {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("selectDlogFiles() did not include current target %q", current)
	}
}

func TestParseDlogDateRejectsInvalidFilename(t *testing.T) {
	if _, err := parseDlogDate("1JDLM01-20260811-002-000043.dlog"); err == nil {
		t.Fatal("parseDlogDate() accepted invalid filename")
	}
}
