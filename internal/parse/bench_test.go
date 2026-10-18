package parse

import (
	"os"
	"testing"
)

func BenchmarkDecodeMRS(b *testing.B) {
	buf, err := os.ReadFile("../../testdata/fixtures/mrs/Custom_Direct_Domain.mrs")
	if err != nil {
		b.Skipf("fixture not found: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := DecodeMRS(buf)
		if err != nil {
			b.Fatal(err)
		}
	}
}

func BenchmarkIsMRS(b *testing.B) {
	buf, err := os.ReadFile("../../testdata/fixtures/mrs/Custom_Direct_Domain.mrs")
	if err != nil {
		b.Skipf("fixture not found: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		IsMRS(buf)
	}
}
