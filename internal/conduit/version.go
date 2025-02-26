package conduit

import (
	"strings"

	"github.com/Masterminds/semver/v3"
)

type Flags struct {
	args *Args
}

type Args struct {
	PipelineFile   string
	ConnectorsPath string
	DBPath         string
	ProcessorsPath string
}

func NewFlags(fns ...func(*Args)) *Flags {
	var args Args
	for _, fn := range fns {
		fn(&args)
	}
	return &Flags{args: &args}
}

func (f *Flags) ForVersion(ver string) []string {
	verConstraint, _ := semver.NewConstraint("< 0.12.0")
	sanitized, _ := strings.CutPrefix(ver, "v")
	v, _ := semver.NewVersion(sanitized)

	if verConstraint.Check(v) {
		return f.version011()
	}
	return f.version012()
}

func (f *Flags) version011() []string {
	return []string{
		"/app/conduit",
		"-pipelines.path", f.args.PipelineFile,
		"-connectors.path", f.args.ConnectorsPath,
		"-db.type", "sqlite",
		"-db.sqlite.path", f.args.DBPath,
		"-pipelines.exit-on-error",
		"-processors.path", f.args.ProcessorsPath,
	}
}

func (f *Flags) version012() []string {
	return []string{
		"/app/conduit",
		"--pipelines.path", f.args.PipelineFile,
		"--connectors.path", f.args.ConnectorsPath,
		"--db.type", "sqlite",
		"--db.sqlite.path", f.args.DBPath,
		"--pipelines.exit-on-degraded",
		"--processors.path", f.args.ProcessorsPath,
	}
}

func WithPipelineFile(file string) func(*Args) {
	return func(a *Args) {
		a.PipelineFile = file
	}
}

func WithConnectorsPath(path string) func(*Args) {
	return func(a *Args) {
		a.ConnectorsPath = path
	}
}

func WithDBPath(path string) func(*Args) {
	return func(a *Args) {
		a.DBPath = path
	}
}

func WithProcessorsPath(path string) func(*Args) {
	return func(a *Args) {
		a.ProcessorsPath = path
	}
}
