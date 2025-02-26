package conduit

// ArgsByVersion returns the args needed for creating the
// runtime container depending on the conduit version.
func ArgsByVersion(version string, pipelineFile string, connectorsPath string, dbPath string, processorsPath string) []string {
	var args []string
	if version < "v0.12.0" {
		args = []string{
			"/app/conduit",
			"-pipelines.path", pipelineFile,
			"-connectors.path", connectorsPath,
			"-db.type", "sqlite",
			"-db.sqlite.path", dbPath,
			"-pipelines.exit-on-error",
			"-processors.path", processorsPath,
		}
	} else {
		args = []string{
			"/app/conduit",
			"--pipelines.path", pipelineFile,
			"--connectors.path", connectorsPath,
			"--db.type", "sqlite",
			"--db.sqlite.path", dbPath,
			"--pipelines.exit-on-degraded",
			"--processors.path", processorsPath,
		}
	}

	return args
}
