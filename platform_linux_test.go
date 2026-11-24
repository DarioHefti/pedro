//go:build linux

package main

import (
	"testing"

	"github.com/wailsapp/wails/v2/pkg/options"
)

type fakeLinuxEnv struct {
	vars   map[string]string
	nvidia bool
}

func (f *fakeLinuxEnv) Getenv(k string) string { return f.vars[k] }
func (f *fakeLinuxEnv) LookupEnv(k string) (string, bool) {
	v, ok := f.vars[k]
	return v, ok
}
func (f *fakeLinuxEnv) Setenv(k, v string) error {
	f.vars[k] = v
	return nil
}
func (f *fakeLinuxEnv) HasNvidiaDriver() bool { return f.nvidia }

func newOpts() *options.App { return &options.App{MinWidth: 808, MinHeight: 608} }

func TestConfigureLinuxSetsAppID(t *testing.T) {
	opts := newOpts()
	configureLinux(opts, &fakeLinuxEnv{vars: map[string]string{}})
	if opts.Linux == nil || opts.Linux.ProgramName != "pedro" {
		t.Fatalf("ProgramName = %+v, want pedro", opts.Linux)
	}
	if len(opts.Linux.Icon) == 0 {
		t.Fatal("icon not embedded")
	}
}

func TestConfigureLinuxMinSize(t *testing.T) {
	cases := []struct {
		name    string
		vars    map[string]string
		wantMin bool
	}{
		{"x11 session keeps min size", map[string]string{"XDG_SESSION_TYPE": "x11", "DISPLAY": ":0"}, true},
		{"wayland drops min size", map[string]string{"WAYLAND_DISPLAY": "wayland-1"}, false},
		{"wayland backend list", map[string]string{"WAYLAND_DISPLAY": "wayland-1", "GDK_BACKEND": "wayland,x11"}, false},
		{"forced xwayland keeps min size", map[string]string{"WAYLAND_DISPLAY": "wayland-1", "GDK_BACKEND": "x11"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			opts := newOpts()
			configureLinux(opts, &fakeLinuxEnv{vars: tc.vars})
			if got := opts.MinWidth == 808 && opts.MinHeight == 608; got != tc.wantMin {
				t.Fatalf("min size kept = %v, want %v (got %dx%d)", got, tc.wantMin, opts.MinWidth, opts.MinHeight)
			}
		})
	}
}

func TestConfigureLinuxRendererWorkarounds(t *testing.T) {
	const dmabuf, compositing = "WEBKIT_DISABLE_DMABUF_RENDERER", "WEBKIT_DISABLE_COMPOSITING_MODE"
	cases := []struct {
		name   string
		vars   map[string]string
		nvidia bool
		want   map[string]string // "" = must stay unset
	}{
		{"wayland disables compositing only", map[string]string{"WAYLAND_DISPLAY": "wayland-1"}, false,
			map[string]string{dmabuf: "", compositing: "1"}},
		{"x11 nvidia disables dmabuf only", map[string]string{"XDG_SESSION_TYPE": "x11"}, true,
			map[string]string{dmabuf: "1", compositing: ""}},
		{"x11 without nvidia untouched", map[string]string{"XDG_SESSION_TYPE": "x11"}, false,
			map[string]string{dmabuf: "", compositing: ""}},
		{"user opt-out wins", map[string]string{"WAYLAND_DISPLAY": "wayland-1", compositing: "0", dmabuf: "0"}, true,
			map[string]string{dmabuf: "0", compositing: "0"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			env := &fakeLinuxEnv{vars: tc.vars, nvidia: tc.nvidia}
			configureLinux(newOpts(), env)
			for k, want := range tc.want {
				got, set := env.vars[k]
				if want == "" && set {
					t.Fatalf("%s = %q, want unset", k, got)
				}
				if want != "" && got != want {
					t.Fatalf("%s = %q, want %q", k, got, want)
				}
			}
		})
	}
}
