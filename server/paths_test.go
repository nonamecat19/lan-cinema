package server

import "testing"

func TestPathsOverlap(t *testing.T) {
	tests := []struct {
		a, b string
		want bool
	}{
		{"/media", "/media", true},
		{"/media", "/media/cache", true},
		{"/media/cache", "/media", true},
		{"/media", "/media-other", false},
		{"/media/series", "/media/cache", false},
	}
	for _, test := range tests {
		if got := pathsOverlap(test.a, test.b); got != test.want {
			t.Errorf("pathsOverlap(%q, %q) = %t, want %t", test.a, test.b, got, test.want)
		}
	}
}
