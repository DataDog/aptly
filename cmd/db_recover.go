package cmd

import (
	"github.com/smira/commander"

	"github.com/DataDog/aptly/database/goleveldb"
)

// aptly db recover
func aptlyDbRecover(cmd *commander.Command, args []string) error {
	var err error

	if len(args) != 0 {
		cmd.Usage()
		return commander.ErrCommandError
	}

	context.Progress().Printf("Recovering database...\n")
	err = goleveldb.RecoverDB(context.DBPath())

	return err
}

func makeCmdDbRecover() *commander.Command {
	cmd := &commander.Command{
		Run:       aptlyDbRecover,
		UsageLine: "recover",
		Short:     "recover DB after crash",
		Long: `
Database recover does its' best to recover the database after a crash.
It is recommended to backup the DB before running recover.

Example:

  $ aptly db recover
`,
	}

	return cmd
}
