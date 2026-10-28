//go:build !windows

package main

import (
	"fmt"
	"os"
)

func attachConsole() bool { return true }

func messageBox(title, msg string) { fmt.Fprintln(os.Stderr, title+": "+msg) }
