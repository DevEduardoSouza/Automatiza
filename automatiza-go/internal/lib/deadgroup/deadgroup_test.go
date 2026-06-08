package deadgroup

import (
	"errors"
	"testing"
)

func TestIsNotParticipatingError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"waha 500 not participating", errors.New(`failed to send image message: WAHA error 500: {"message":"2 UNKNOWN: failed to get group members: you're not participating in that group"}`), true},
		{"group members only", errors.New("2 UNKNOWN: failed to get group members"), true},
		{"uppercase", errors.New("You're NOT PARTICIPATING IN THAT GROUP"), true},
		{"timeout unrelated", errors.New("context deadline exceeded"), false},
		{"404 unrelated", errors.New("instance not found"), false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if got := IsNotParticipatingError(tc.err); got != tc.want {
				t.Fatalf("IsNotParticipatingError(%v) = %v, want %v", tc.err, got, tc.want)
			}
		})
	}
}
