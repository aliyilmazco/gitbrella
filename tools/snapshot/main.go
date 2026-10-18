// Command snapshot records file metadata under a directory and compares two
// recordings. It is a development tool for proving that gitbrella
// changes nothing; it is not shipped.
//
//	go run ./tools/snapshot save <file.json> <dir>
//	go run ./tools/snapshot same <file.json> <dir>
package main

import (
	"fmt"
	"os"

	"github.com/aliyilmazco/gitbrella/internal/fixtures"
	"github.com/aliyilmazco/gitbrella/internal/scan"
)

func main() {
	if len(os.Args) != 4 || (os.Args[1] != "save" && os.Args[1] != "same") {
		fmt.Fprintln(os.Stderr, "usage: snapshot save|same <file.json> <dir>")
		os.Exit(2)
	}
	if err := scan.PreventMaterialization(); err != nil {
		fmt.Fprintln(os.Stderr, "snapshot: cannot disable cloud downloads:", err)
		os.Exit(2)
	}
	flags := scan.DefaultFlagReader()
	skip := func(p string) bool { d, _ := flags.IsDataless(p); return d }
	snap, err := fixtures.TakeSnapshot(os.Args[3], skip)
	if err != nil {
		fmt.Fprintln(os.Stderr, "snapshot:", err)
		os.Exit(2)
	}
	if os.Args[1] == "save" {
		if err := snap.Save(os.Args[2]); err != nil {
			fmt.Fprintln(os.Stderr, "snapshot:", err)
			os.Exit(2)
		}
		fmt.Printf("saved %d paths\n", len(snap))
		return
	}
	before, err := fixtures.LoadSnapshot(os.Args[2])
	if err != nil {
		fmt.Fprintln(os.Stderr, "snapshot:", err)
		os.Exit(2)
	}
	if d := before.Diff(snap); len(d) > 0 {
		for _, line := range d {
			fmt.Println(line)
		}
		os.Exit(1)
	}
	fmt.Printf("unchanged: %d paths\n", len(snap))
}
