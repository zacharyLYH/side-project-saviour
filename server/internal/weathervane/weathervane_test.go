package weathervane

import (
	"errors"
	"fmt"
	"testing"

	"github.com/stretchr/testify/mock"

	"sps/internal/docker"
	dockermocks "sps/mocks/docker"
)

func TestContainerStatusMapping(t *testing.T) {
	cases := []struct {
		name    string
		inspect docker.Container
		want    string
	}{
		{"running", docker.Container{Running: true, Status: "running"}, StateRunning},
		{"exited", docker.Container{Running: false, Status: "exited"}, "exited"},
		{"created", docker.Container{Running: false, Status: "created"}, "created"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			d := dockermocks.NewMockClient(t)
			d.EXPECT().Inspect(mock.Anything, "sps-x").Return(tc.inspect, nil)
			got, err := Container(t.Context(), d, "sps-x")
			if err != nil || got.State != tc.want {
				t.Fatalf("got %+v err=%v, want %q", got, err, tc.want)
			}
		})
	}
}

func TestContainerMissingIsAStatus(t *testing.T) {
	d := dockermocks.NewMockClient(t)
	wrapped := fmt.Errorf("inspect: %w", docker.ErrNotFound)
	d.EXPECT().Inspect(mock.Anything, "sps-x").Return(docker.Container{}, wrapped)
	got, err := Container(t.Context(), d, "sps-x")
	if err != nil || got.State != StateMissing {
		t.Fatalf("got %+v err=%v, want missing", got, err)
	}
}

func TestContainerEngineErrorPropagates(t *testing.T) {
	d := dockermocks.NewMockClient(t)
	d.EXPECT().Inspect(mock.Anything, "sps-x").Return(docker.Container{}, errors.New("engine on fire"))
	_, err := Container(t.Context(), d, "sps-x")
	if err == nil || err.Error() != "engine on fire" {
		t.Fatalf("err = %v, want the engine error verbatim", err)
	}
}
