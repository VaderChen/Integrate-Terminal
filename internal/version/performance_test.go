package version

import "testing"

var benchmarkVersion string

func BenchmarkCurrent(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkVersion = Current()
	}
}

func BenchmarkUpdateVersion(b *testing.B) {
	b.ReportAllocs()
	for i := 0; i < b.N; i++ {
		benchmarkVersion = UpdateVersion()
	}
}
