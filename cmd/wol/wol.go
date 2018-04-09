package main

////////////////////////////////////////////////////////////////////////////////

import (
	"errors"
	"fmt"
	"os"
	"os/user"
	"path"
	"strings"

	flags "github.com/jessevdk/go-flags"

	wol "github.com/sabhiram/go-wol"
)

////////////////////////////////////////////////////////////////////////////////

const DBPath = "/.config/go-wol/bolt.db"

var (
	// Define holders for the cli arguments we wish to parse.
	Options struct {
		Version            bool   `short:"v" long:"version"`
		Help               bool   `short:"h" long:"help"`
		BroadcastInterface string `short:"i" long:"interface" default:""`
		BroadcastIP        string `short:"b" long:"bcast" default:"255.255.255.255"`
		UDPPort            string `short:"p" long:"port" default:"9"`
	}
)

////////////////////////////////////////////////////////////////////////////////

// Run the alias command.
func aliasCmd(args []string, aliases *Aliases) error {
	if len(args) >= 2 {
		var eth string
		if len(args) > 2 {
			eth = args[2]
		}
		// TODO: Validate mac address
		alias, mac := args[0], args[1]
		return aliases.Add(alias, mac, eth)
	}
	return errors.New("alias command requires a <name> and a <mac>")
}

// Run the list command.
func listCmd(args []string, aliases *Aliases) error {
	mp, err := aliases.List()
	if err != nil {
		fmt.Fprintf(os.Stderr, "Failed to get list of aliases: %v\n", err)
		return err
	}
	if len(mp) == 0 {
		fmt.Printf("No aliases found! Add one with \"wol alias <name> <mac>\"\n")
	} else {
		for alias, mi := range mp {
			fmt.Printf("    %s - %s %s\n", alias, mi.Mac, mi.Iface)
		}
	}
	return nil
}

// Run the remove command.
func removeCmd(args []string, aliases *Aliases) error {
	if len(args) > 0 {
		alias := args[0]
		return aliases.Del(alias)
	}
	return errors.New("remove command requires a <name> of an alias")
}

// Run the wake command.
func wakeCmd(args []string, aliases *Aliases) error {
	if len(args) <= 0 {
		return errors.New("No mac address specified to wake command")
	}

	// bcastInterface can be "eth0", "eth1", etc.. An empty string implies
	// that we use the default interface when sending the UDP packet (nil).
	bcastInterface := ""
	macAddr := args[0]

	// First we need to see if this macAddr is actually an alias, if it is:
	// we set the eth interface based on the stored item, and set the macAddr
	// based on the alias entry's mac for this alias.
	mi, err := aliases.Get(macAddr)
	if err == nil {
		macAddr = mi.Mac
		bcastInterface = mi.Iface
	}

	// Always use the interface specified in the command line, if it exists.
	if Options.BroadcastInterface != "" {
		bcastInterface = Options.BroadcastInterface
	}

	err = wol.SendMagicPacket(macAddr, Options.BroadcastIP+":"+Options.UDPPort, bcastInterface)
	if err != nil {
		return err
	}

	fmt.Printf("Magic packet sent successfully to %s\n", macAddr)
	return nil
}

////////////////////////////////////////////////////////////////////////////////

type cmdFnType func([]string, *Aliases) error

var cmdMap = map[string]cmdFnType{
	"alias":  aliasCmd,
	"list":   listCmd,
	"remove": removeCmd,
	"wake":   wakeCmd,
}

////////////////////////////////////////////////////////////////////////////////

// Helper function to dump the usage and print an error if specified,
// it also returns the exit code requested to the function (saves me a line).
func printUsageGetExitCode(s string, e int) int {
	if len(s) > 0 {
		fmt.Printf(s)
	}
	fmt.Printf(getAppUsageString())
	return e
}

func fatalOnError(err error) {
	if err != nil {
		fmt.Printf("Fatal error: %s\n", err.Error())
		os.Exit(1)
	}
}

// Main entry point for binary.
func main() {
	var args []string

	// Detect the current user to figure out what their ~ is.
	usr, err := user.Current()
	fatalOnError(err)

	// Load the list of aliases from the file at DBPath.
	aliases, err := LoadAliases(path.Join(usr.HomeDir, DBPath))
	fatalOnError(err)
	defer aliases.Close()

	// Parse arguments which might get passed to "wol".
	parser := flags.NewParser(&Options, flags.Default & ^flags.HelpFlag)
	args, err = parser.Parse()

	ec := 0
	switch {

	// Parse Error, print usage.
	case err != nil:
		ec = printUsageGetExitCode("", 1)

	// No arguments, or help requested, print usage.
	case len(os.Args) == 1 || Options.Help:
		ec = printUsageGetExitCode("", 0)

	// "--version" requested.
	case Options.Version:
		fmt.Printf("%s\n", wol.Version)

	// Make sure we are being asked to run a something.
	case len(args) == 0:
		ec = printUsageGetExitCode("No command specified, see usage:\n", 1)

	// All other cases go here.
	case true:
		cmd, cmdArgs := strings.ToLower(args[0]), args[1:]
		if fn, ok := cmdMap[cmd]; ok {
			err = fn(cmdArgs, aliases)
		} else {
			err = wakeCmd(args, aliases)
		}
		fatalOnError(err)
	}
	os.Exit(ec)
}
