package daemon

import (
	"testing"
)

func TestShouldInitiateICEStable(t *testing.T) {
	if !shouldInitiateICE("aaa", "bbb") {
		t.Fatal("lower id should initiate")
	}
	if shouldInitiateICE("bbb", "aaa") {
		t.Fatal("higher id should wait")
	}
}
