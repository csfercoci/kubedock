package types

import (
	"testing"
)

func TestVolumeMatch(t *testing.T) {
	tests := []struct {
		vol   *Volume
		typ   string
		key   string
		val   string
		match bool
		err   bool
	}{
		{
			vol:   &Volume{Name: "myvolume", Driver: "local", Labels: map[string]string{"env": "test"}},
			typ:   "label",
			key:   "env",
			val:   "test",
			match: true,
		},
		{
			vol:   &Volume{Name: "myvolume", Driver: "local", Labels: map[string]string{"env": "test"}},
			typ:   "label",
			key:   "env",
			val:   "prod",
			match: false,
		},
		{
			vol:   &Volume{Name: "myvolume", Driver: "local", Labels: map[string]string{"env": "test"}},
			typ:   "label",
			key:   "missing",
			val:   "test",
			match: false,
		},
		{
			vol:   &Volume{Name: "myvolume", Driver: "local"},
			typ:   "name",
			key:   "myvolume",
			val:   "",
			match: true,
		},
		{
			vol:   &Volume{Name: "myvolume", Driver: "local"},
			typ:   "name",
			key:   "other",
			val:   "",
			match: false,
		},
		{
			vol:   &Volume{Name: "myvolume", Driver: "local"},
			typ:   "name",
			key:   "my.*",
			val:   "",
			match: true,
		},
		{
			vol:   &Volume{Name: "myvolume", Driver: "local"},
			typ:   "driver",
			key:   "local",
			val:   "",
			match: true,
		},
		{
			vol:   &Volume{Name: "myvolume", Driver: "local"},
			typ:   "driver",
			key:   "nfs",
			val:   "",
			match: false,
		},
		{
			vol:   &Volume{Name: "myvolume", Driver: "local"},
			typ:   "unknown",
			key:   "foo",
			val:   "bar",
			match: true,
		},
	}
	for i, tst := range tests {
		match, err := tst.vol.Match(tst.typ, tst.key, tst.val)
		if err != nil && !tst.err {
			t.Errorf("failed test %d - unexpected error: %s", i, err)
		}
		if match != tst.match {
			t.Errorf("failed test %d - expected match=%v, but got %v", i, tst.match, match)
		}
	}
}
