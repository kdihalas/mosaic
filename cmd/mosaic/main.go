package main

import (
	"context"
	"github.com/kdihalas/mosaic/internal/cli"
	"os"
)

func main() { os.Exit(cli.Execute(context.Background(), os.Args[1:], os.Stdin, os.Stdout, os.Stderr)) }
