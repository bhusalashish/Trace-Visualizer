package main

import (
	"fmt"
	"os"

	tracevisualizer "github.com/bhusalashish/Trace-Visualizer"
	"github.com/spf13/cobra"
)

func main() {
	var filename string
	var patterns []string

	var rootCmd = &cobra.Command{
		Use:   "parse",
		Short: "Tool to parse the log file and generate a json compatible to otel",
	}

	rootCmd.Flags().StringVarP(&filename, "file", "f", "", "File to search")
	rootCmd.PersistentFlags().StringSliceVarP(&patterns, "regex", "r", []string{}, "Tracehandle regex pattern to match (can specify multiple patterns)")

	rootCmd.MarkFlagRequired("file")
	rootCmd.MarkFlagRequired("regex")

	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		tracevisualizer.Parse(filename, patterns)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
