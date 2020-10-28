package util

import (
	"testing"
)

func Test_Gen(t *testing.T) {
	g, err := NewUUID(1023)

	if nil != err {
		t.Fail()
	}

	var id uint64 = 0
	for i := 0; i < 5000; i++ {
		temp := g.Generate()
		if temp == id {
			t.Fail()
		}
		temp = id
	}
}

func Benchmark_Gen(b *testing.B) {
	g, _ := NewUUID(1023)
	m := make(map[uint64]bool)
	//这里没次超过4096个都要等到下一纳秒
	for i := 0; i < b.N; i++ {
		t := g.Generate()
		if _, ok := m[t]; ok {
			panic("has repeat data")
		}
		m[t] = true
	}
}
