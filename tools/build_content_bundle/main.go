package main

import (
	"bytes"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"

	"view_panel/internal/content"
)

func main() {
	os.Exit(run(os.Args[1:]))
}

func run(args []string) int {
	return runWithIO(args, os.Stdout, os.Stderr)
}

func runWithIO(args []string, stdout, stderr io.Writer) int {
	flagSet := flag.NewFlagSet("build_content_bundle", flag.ContinueOnError)
	flagSet.SetOutput(io.Discard)

	source := flagSet.String("source", "content/src", "content source root containing personas/ and materials/")
	out := flagSet.String("out", "content/build/default", "bundle output root to replace with compiled runtime artifacts")
	bundleID := flagSet.String("bundle-id", "default", "bundle identifier recorded in bundle-manifest.json")

	if err := flagSet.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			_, _ = io.WriteString(stdout, usageText(flagSet))
			return 0
		}
		fmt.Fprintf(stderr, "%v\n\n%s", err, usageText(flagSet))
		return 2
	}
	if extras := flagSet.Args(); len(extras) > 0 {
		fmt.Fprintf(stderr, "unexpected positional arguments: %v\n\n%s", extras, usageText(flagSet))
		return 2
	}

	if err := runBuildContentBundle(*source, *out, *bundleID); err != nil {
		fmt.Fprintln(stderr, err)
		return 1
	}

	return 0
}

func usageText(flagSet *flag.FlagSet) string {
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "Usage: %s [flags]\n\n", flagSet.Name())
	fmt.Fprintf(&buf, "Build a runtime-ready content bundle from an authored content source tree.\n")
	fmt.Fprintf(&buf, "The output root is replaced as a single snapshot and contains bundle-manifest.json plus runtime/ artifacts.\n\n")
	fmt.Fprintf(&buf, "Flags:\n")
	flagSet.SetOutput(&buf)
	flagSet.PrintDefaults()
	return buf.String()
}

func runBuildContentBundle(sourceRoot, outRoot, bundleID string) error {
	catalog, err := content.LoadCatalog(sourceRoot)
	if err != nil {
		return fmt.Errorf("load content catalog: %w", err)
	}

	bundle, err := content.CompileBundle(catalog, content.CompileOptions{BundleID: bundleID})
	if err != nil {
		return fmt.Errorf("compile content bundle: %w", err)
	}

	if err := content.WriteBundle(outRoot, bundle); err != nil {
		return fmt.Errorf("write content bundle: %w", err)
	}

	return nil
}
