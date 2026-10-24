package main

import (
	"evolution.local/companion/internal/companion"
	"os"
)

func main() { os.Exit(companion.RunTerminal(os.Args[1:], os.Args[0], os.Stdout, os.Stderr)) }
