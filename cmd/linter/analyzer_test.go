package main

import (
	"testing"

	"golang.org/x/tools/go/analysis/analysistest"
)

func TestAnalyzer_Panic(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer, "withpanic")
}

func TestAnalyzer_NoPanic(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer, "nopanic")
}

func TestAnalyzer_ExitOutsideMain(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer, "exitoutsidemain")
}

func TestAnalyzer_MainPkg(t *testing.T) {
	analysistest.Run(t, analysistest.TestData(), Analyzer, "mainpkg")
}
