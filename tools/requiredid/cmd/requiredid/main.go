package main

import (
	"github.com/go-gui-org/go-gui/tools/requiredid"

	"golang.org/x/tools/go/analysis/singlechecker"
)

func main() { singlechecker.Main(requiredid.Analyzer) }
