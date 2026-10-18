package mrs

import (
	"os"
	"testing"
)

func BenchmarkGenerate(b *testing.B) {
	buf, err := os.ReadFile("../../testdata/fixtures/mrs/Custom_Direct_Domain.src.txt")
	if err != nil {
		b.Skipf("fixture not found: %v", err)
	}
	b.ResetTimer()
	for i := 0; i < b.N; i++ {
		_, err := Generate(buf, "domain", "mrs")
		if err != nil {
			b.Fatal(err)
		}
	}
}
