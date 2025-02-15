package main

import (
	"math/rand"
	"os"
	"time"

	"github.com/DataDog/aptly/aptly"
	"github.com/DataDog/aptly/cmd"

	_ "embed"
)

//go:generate sh -c "make -s version | tr -d '\n' > VERSION"
//go:embed VERSION
var Version string

func main() {
	if Version == "" {
		Version = "unknown"
	}

	aptly.Version = Version

	rand.Seed(time.Now().UnixNano())

	os.Exit(cmd.Run(cmd.RootCommand(), os.Args[1:], true))
}
