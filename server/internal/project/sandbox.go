package project

import (
	"archive/tar"
	"bytes"
	"embed"
	"io"

	_ "embed"
)

//go:embed sandbox/Dockerfile
var sandboxFS embed.FS

// sandboxContext returns a tar stream of the embedded sandbox build
// context, so the image definition ships inside the binary and no path
// configuration is needed.
func sandboxContext() io.Reader {
	raw, err := sandboxFS.ReadFile("sandbox/Dockerfile")
	if err != nil {
		panic("embedded sandbox Dockerfile missing: " + err.Error())
	}
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	_ = tw.WriteHeader(&tar.Header{Name: "Dockerfile", Mode: 0o644, Size: int64(len(raw))})
	_, _ = tw.Write(raw)
	_ = tw.Close()
	return &buf
}
