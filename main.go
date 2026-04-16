package main

import "github.com/beobeodev/fscan/cmd"

// version is set via ldflags at build time: -ldflags "-X main.version=v0.1.0"
var version = "dev"

func main() {
	cmd.SetVersion(version)
	cmd.Execute()
}
