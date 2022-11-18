package sip

import (
	"testing"
)

func TestAddress_Equals(t *testing.T) {
	addr1 := &Address{
		Uri: &SipUri{
			FUser: String{"dummy"},
			FHost: "example.com",
		},
	}
	addr2 := addr1.Clone()
	addr2.Params = NewParams(HeaderParams).Add("a", String{"qwerty"})
	tests := []struct {
		name        string
		input       *Address
		compareWith *Address
		expected    bool
	}{
		{"nil to nil", nil, nil, true},
		{"addr to nil", addr1, nil, false},
		{"addr to empty addr", addr1, &Address{}, false},
		{"addr to same addr", addr1, addr1.Clone(), true},
		{"addr to addr2", addr1, addr2, false},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if r := test.input.Equals(test.compareWith); r != test.expected {
				t.Errorf("Expected %v, but got %v", test.expected, r)
			}
		})
	}
}
