package main

import (
	"fmt"
	"os"
	"path/filepath"
)

func main() {
	log("main", "debug", "Starting program...")

	recursive, pattern, paths, err := parseArgs(os.Args)
	if err != nil {
		log("main", "error", fmt.Sprintf("Argument parsing failed: %v", err))
		os.Exit(2)
	}

	foundAny := false
	multi := recursive || len(paths) > 1

	if len(paths) == 0 {
		// No paths: read stdin
		if scanAndPrint("stdin", os.Stdin, pattern, false) {
			foundAny = true
		}
	} else if recursive {
		// Recursive: walk each root
		for _, root := range paths {
			err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
				if err != nil || info.IsDir() {
					return nil
				}
				f, err := os.Open(path)
				if err != nil {
					return nil
				}
				defer f.Close()
				if scanAndPrint(path, f, pattern, true) {
					foundAny = true
				}
				return nil
			})
			if err != nil {
				log("main", "error", fmt.Sprintf("Error walking %s: %v", root, err))
				os.Exit(2)
			}
		}
	} else {
		// Non-recursive, specific files
		for _, filename := range paths {
			f, err := os.Open(filename)
			if err != nil {
				log("main", "error", fmt.Sprintf("Failed to open file %q: %v", filename, err))
				os.Exit(2)
			}
			defer f.Close()
			if scanAndPrint(filename, f, pattern, multi) {
				foundAny = true
			}
		}
	}

	if foundAny {
		os.Exit(0)
	}
	os.Exit(1)
}
