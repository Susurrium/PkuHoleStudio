package main

import "testing"

func TestCrawlerCommandUsesExplicitStartPageOnly(t *testing.T) {
	cmd := newCrawlerCmd()

	if flag := cmd.Flags().Lookup("resume"); flag != nil {
		t.Fatal("crawler command must not expose the unsupported --resume flag")
	}
	if flag := cmd.Flags().Lookup("start-page"); flag == nil {
		t.Fatal("crawler command must keep the --start-page flag")
	}
}
