package version

// Version is the CLI version string. Override at link time:
//
//	go build -ldflags "-X github.com/yunxiao-cli/yunxiao/internal/version.Version=0.16.23"
//
// Default matches the release when built without ldflags.
var Version = "0.16.23"
