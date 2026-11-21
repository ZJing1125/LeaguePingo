package core

import "testing"

func TestShardsPreserveMissingMiddleAndRepeatedID(t *testing.T) {
	a, b := 5008, 5001
	got := shardSlots(&a, nil, &b)
	if len(got) != 3 || got[0] != 5008 || got[1] != 0 || got[2] != 5001 {
		t.Fatal("missing shard shifted the other slots")
	}
	got = shardSlots(&a, &a, &b)
	if got[0] != got[1] {
		t.Fatal("repeated shard was removed")
	}
}
