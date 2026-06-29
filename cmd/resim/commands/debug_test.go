package commands

import (
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestIsTransientExecDialError(t *testing.T) {
	cases := []struct {
		name string
		err  error
		want bool
	}{
		{"nil", nil, false},
		{"kubelet serving cert not ready", errors.New("error dialing backend: remote error: tls: internal error"), true},
		{"dialing backend", errors.New("error dialing backend: connection refused"), true},
		{"tls internal error", errors.New("remote error: tls: internal error"), true},
		{"unrelated session error", errors.New("command terminated with exit code 1"), false},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			assert.Equal(t, c.want, isTransientExecDialError(c.err))
		})
	}
}
