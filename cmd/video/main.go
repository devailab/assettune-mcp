package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/devailab/assettune-mcp/internal/tools/video"
)

func main() {
	dir := flag.String("dir", "media/videos", "directory to scan for videos")
	overwrite := flag.Bool("overwrite", true, "re-encode even if output exists (use -overwrite=false to skip)")
	concurrency := flag.Int("concurrency", 0, "parallel workers (0 = 1)")
	flag.Parse()

	if err := video.CheckFFmpeg(); err != nil {
		fmt.Fprintln(os.Stderr, err.Error())
		os.Exit(1)
	}

	result, err := video.Run(context.Background(), *dir, *concurrency, *overwrite)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tORIG MB\tOPT MB\tSAVINGS\tFILE")
	for _, f := range result.Files {
		status := "ok"
		switch {
		case f.Error != "":
			status = "ERR"
		case f.Skipped:
			status = "skip"
		}
		fmt.Fprintf(w, "%s\t%.1f\t%.1f\t%.1f%%\t%s\n",
			status,
			float64(f.OriginalKB)/1024,
			float64(f.OptimizedKB)/1024,
			f.SavingsPct,
			f.Original,
		)
		if f.Error != "" {
			fmt.Fprintf(w, "\t\t\t\t  error: %s\n", f.Error)
		}
	}
	w.Flush()

	fmt.Printf("\nfound=%d  converted=%d  skipped=%d  failed=%d  savings=%.1f%%\n",
		result.TotalFound, result.Converted, result.Skipped, result.Failed, result.TotalSavings)
}
