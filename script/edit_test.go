package script

import (
	"errors"
	"strings"
	"testing"
)

const kept = `@size = 2
# The first flag.
first = { size = @size }

second = {
	size = 1
} # second's comment
third = { size = 3 }
`

// A definition takes the place of the one of its key where it stands, and
// everything around it stays as it was.
func TestPutDefinitionReplaces(t *testing.T) {
	merged, replaced, err := PutDefinition(kept, "second", "second", "second = {\n\tsize = 5\n}")
	if err != nil || !replaced {
		t.Fatalf("replaced %v, err %v", replaced, err)
	}

	want := strings.Replace(kept, "second = {\n\tsize = 1\n}", "second = {\n\tsize = 5\n}", 1)
	if merged != want {
		t.Errorf("merged\n%s\nwant\n%s", merged, want)
	}
}

// A key the file does not define gets the definition at the end, a blank line
// apart, and an empty file gets it alone after a byte order mark.
func TestPutDefinitionAdds(t *testing.T) {
	merged, replaced, err := PutDefinition(kept, "fourth", "fourth", "fourth = { size = 4 }")
	if err != nil || replaced {
		t.Fatalf("replaced %v, err %v", replaced, err)
	}

	if want := strings.TrimRight(kept, "\n") + "\n\nfourth = { size = 4 }\n"; merged != want {
		t.Errorf("merged\n%q\nwant\n%q", merged, want)
	}

	for _, empty := range []string{"", byteOrderMark, " \n\n"} {
		merged, replaced, err := PutDefinition(empty, "first", "first", "first = {\n}")
		if err != nil || replaced || merged != byteOrderMark+"first = {\n}\n" {
			t.Errorf("into %q: %q, replaced %v, err %v", empty, merged, replaced, err)
		}
	}
}

// A renamed definition takes the place of the one it was read as, and is
// refused a key another definition already has.
func TestPutDefinitionRenames(t *testing.T) {
	merged, replaced, err := PutDefinition(kept, "second", "renamed", "renamed = { size = 2 }")
	if err != nil || !replaced || !strings.Contains(merged, "\nrenamed = { size = 2 } # second's comment") || Defines(merged, "second") {
		t.Errorf("renamed: %q, replaced %v, err %v", merged, replaced, err)
	}

	if _, _, err := PutDefinition(kept, "second", "third", "third = { size = 2 }"); !errors.Is(err, ErrKeyTaken) {
		t.Errorf("renamed to a key the file has: err %v, want ErrKeyTaken", err)
	}
}

// The file's own line breaks are kept, and its byte order mark.
func TestPutDefinitionKeepsLineBreaks(t *testing.T) {
	file := byteOrderMark + strings.ReplaceAll(kept, "\n", "\r\n")

	merged, _, err := PutDefinition(file, "fourth", "fourth", "fourth = {\n\tsize = 4\n}")
	if err != nil {
		t.Fatal(err)
	}

	if !strings.HasPrefix(merged, byteOrderMark) || strings.Count(merged, "\n") != strings.Count(merged, "\r\n") {
		t.Errorf("merged %q, want the byte order mark and \r\n throughout", merged)
	}
}

// A file the parser had to read past something in is left alone.
func TestPutDefinitionRefusesBrokenFiles(t *testing.T) {
	if _, _, err := PutDefinition("first = {\n\tsize = 1\n", "first", "first", "first = { }"); !errors.Is(err, ErrUnreadable) {
		t.Errorf("err %v, want ErrUnreadable", err)
	}
}

func FuzzPutDefinition(f *testing.F) {
	f.Add(kept, "second", "renamed")
	f.Add("a = {", "a", "a")

	f.Fuzz(func(t *testing.T, file, replace, key string) {
		merged, _, err := PutDefinition(file, replace, key, key+" = { }")
		if err == nil && !Defines(merged, key) && IsKey(key) {
			t.Errorf("%q does not define %q after putting it in", merged, key)
		}
	})
}
