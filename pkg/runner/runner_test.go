package runner

import "testing"

const sampleSysbenchLog = `
Running warmup
Running a OLTP test
transactions: 12345 (1234.56 per sec.)
queries: 246900 (24691.20 per sec.)
ignored errors: 0 (0.00 per sec.)
reconnects: 0 (0.00 per sec.)
`

func TestParseTPS(t *testing.T) {
	tps, ok := ParseTPS(sampleSysbenchLog)
	if !ok {
		t.Fatal("expected tps")
	}
	if tps < 1234 || tps > 1235 {
		t.Fatalf("unexpected tps: %v", tps)
	}
}

func TestParseTPSMissing(t *testing.T) {
	if _, ok := ParseTPS("no metrics here"); ok {
		t.Fatal("expected false")
	}
}
