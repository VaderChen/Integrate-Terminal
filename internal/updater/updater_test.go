package updater

import "testing"

func TestCompareVersionsDistinguishesBuilds(t *testing.T) {
	tests := []struct {
		name  string
		left  string
		right string
		want  int
	}{
		{name: "newer build", left: "1.26.0904.1530", right: "1.26.0904.1529", want: 1},
		{name: "same build", left: "1.26.0904.1529", right: "1.26.0904.1529", want: 0},
		{name: "legacy version is older", left: "1.26.0904.1529", right: "1.26.0904", want: 1},
		{name: "older build", left: "1.26.0904.1200", right: "1.26.0904.1529", want: -1},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			got, err := compareVersions(test.left, test.right)
			if err != nil {
				t.Fatalf("compareVersions() error = %v", err)
			}
			if got != test.want {
				t.Fatalf("compareVersions(%q, %q) = %d, want %d", test.left, test.right, got, test.want)
			}
		})
	}
}
