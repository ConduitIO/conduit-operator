package conduit

import (
	"fmt"
	"path/filepath"
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
	LogFormat      string
}

func NewFlags(fns ...func(*Args)) *Flags {
	var args Args
	for _, fn := range fns {
		fn(&args)
	}
	return &Flags{args: &args}
}

func (f *Flags) ForVersion(ver string) ([]string, error) {
	constraints := map[string]string{
		"v011":    "~0.11.x",
		"v012":    "~0.12.x",
		"v013.4":  "<= 0.13.4, >= 0.13.0",
		"v013.5+": "^0.13.5",
	}

	sanitized, _ := strings.CutPrefix(ver, "v")
	v, _ := semver.NewVersion(sanitized)

	for key, rule := range constraints {
		c, err := semver.NewConstraint(rule)
		if err != nil {
			return nil, fmt.Errorf("parse error occured while creating constraint: %w", err)
		}
		if c.Check(v) {
			switch key {
			case "v011":
				return f.v011(), nil
			case "v012":
				return f.v012(), nil
			case "v013.4":
				return f.v013upto4(), nil
			case "v013.5+":
				return f.v0134plus(), nil
			}
		}
	}
	return nil, fmt.Errorf("version %s not supported", ver)
}

func (f *Flags) v011() []string {
	return []string{
		"-pipelines.path", f.args.PipelineFile,
		"-connectors.path", f.args.ConnectorsPath,
		"-db.type", "sqlite",
		"-db.sqlite.path", f.args.DBPath,
		"-pipelines.exit-on-error",
		"-processors.path", f.args.ProcessorsPath,
		"-log.format", f.args.LogFormat,
	}
}

func (f *Flags) v012() []string {
	return []string{
		"--pipelines.path", f.args.PipelineFile,
		"--connectors.path", f.args.ConnectorsPath,
		"--db.type", "sqlite",
		"--db.sqlite.path", f.args.DBPath,
		"--pipelines.exit-on-degraded",
		"--pipelines.error-recovery.max-retries", "0",
		"--processors.path", f.args.ProcessorsPath,
		"--log.format", f.args.LogFormat,
	}
}

func (f *Flags) v013upto4() []string {
	return []string{
		"run",
		"--pipelines.path", f.args.PipelineFile,
		"--connectors.path", f.args.ConnectorsPath,
		"--db.type", "sqlite",
		"--db.sqlite.path", f.args.DBPath,
		"--pipelines.exit-on-degraded",
		"--pipelines.error-recovery.max-retries", "0",
		"--processors.path", f.args.ProcessorsPath,
		"--log.format", f.args.LogFormat,
	}
}

func (f *Flags) v0134plus() []string {
	return []string{
		"run",
		"--pipelines.path", filepath.Dir(f.args.PipelineFile),
		"--connectors.path", f.args.ConnectorsPath,
		"--db.type", "sqlite",
		"--db.sqlite.path", f.args.DBPath,
		"--pipelines.exit-on-degraded",
		"--pipelines.error-recovery.max-retries", "0",
		"--processors.path", f.args.ProcessorsPath,
		"--log.format", f.args.LogFormat,
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

func WithLogFormat(format string) func(*Args) {
	return func(a *Args) {
		a.LogFormat = format
	}
}
