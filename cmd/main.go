package main

import (
	"fmt"
	"os"

	tracevisualizer "github.com/bhusalashish/Trace-Visualizer"
	"github.com/spf13/cobra"
)

func main() {
	fmt.Println("Let's Parse!")

	var filename, regex string

	var rootCmd = &cobra.Command{
		Use:   "parse",
		Short: "Tool to parse the log file and generate a json compatible to otel",
	}

	rootCmd.Flags().StringVarP(&filename, "file", "f", "", "File to search")
	rootCmd.Flags().StringVarP(&regex, "regex", "r", "", "Tracehandle regex to search for")
	rootCmd.MarkFlagRequired("file")
	rootCmd.MarkFlagRequired("regex")

	rootCmd.Run = func(cmd *cobra.Command, args []string) {
		tracevisualizer.Parse(filename, regex)
	}

	if err := rootCmd.Execute(); err != nil {
		fmt.Println(err)
		os.Exit(1)
	}
}
