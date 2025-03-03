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
	constraints := map[string]string{
		"v011": "< 0.12.0",
		"v012": ">= 0.12.0, < 0.13.0",
	}

	sanitized, _ := strings.CutPrefix(ver, "v")
	v, _ := semver.NewVersion(sanitized)

	for key, rule := range constraints {
		c, _ := semver.NewConstraint(rule)
		if c.Check(v) {
			switch key {
			case "v011":
				return f.v011()
			case "v012":
				return f.v012()
			}
		}
	}
	return f.v013()
}

func (f *Flags) v011() []string {
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

func (f *Flags) v012() []string {
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

func (f *Flags) v013() []string {
	return []string{
		"/app/conduit run",
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
