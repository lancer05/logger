package logger

import (
	"testing"
)

func TestParseIP(t *testing.T) {
	cases := []struct {
		Input  string
		Expect string
	}{
		{
			Input:  "1.1.1.1",
			Expect: "1.1.1.1",
		},
		{
			Input:  "1.1.1.1:19123",
			Expect: "1.1.1.1",
		},
		{
			Input:  "240a:6b:100:2aac:e4a3:8908:b1f5:b0bd",
			Expect: "240a:6b:100:2aac:e4a3:8908:b1f5:b0bd",
		},
		{
			Input:  "[240a:6b:100:2aac:e4a3:8908:b1f5:b0bd]:2155",
			Expect: "240a:6b:100:2aac:e4a3:8908:b1f5:b0bd",
		},
	}

	for idx, each := range cases {
		actual := parseIP(each.Input)
		if actual != each.Expect {
			t.Fatalf("%d: expect: %s, got: %s", idx, each.Expect, actual)
		}
	}
}
