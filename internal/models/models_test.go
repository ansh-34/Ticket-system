package models

import "testing"

func TestCanTransitionTo(t *testing.T) {
	cases := []struct {
		from, to Status
		allowed  bool
	}{
		{StatusOpen, StatusInProgress, true},
		{StatusInProgress, StatusClosed, true},
		{StatusOpen, StatusClosed, true}, // forward shortcut is allowed
		{StatusOpen, StatusOpen, false},
		{StatusClosed, StatusOpen, false},       // no reopening
		{StatusClosed, StatusInProgress, false}, // no reopening
		{StatusInProgress, StatusOpen, false},   // no backward move
		{StatusOpen, Status("banana"), false},   // invalid target
		{Status("banana"), StatusOpen, false},   // invalid source
	}
	for _, tc := range cases {
		if got := tc.from.CanTransitionTo(tc.to); got != tc.allowed {
			t.Errorf("%s -> %s: got %v want %v", tc.from, tc.to, got, tc.allowed)
		}
	}
}
