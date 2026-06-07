package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"text/tabwriter"

	"github.com/davidbyttow/govips/v2/vips"
	"github.com/devailab/assettune-mcp/internal/tools/image"
)

func main() {
	dir := flag.String("dir", "media/images", "directory to scan")
	quality := flag.Int("quality", 80, "webp quality (1-100)")
	maxDim := flag.Int("max-dimension", 0, "max width/height in px (0 = no limit)")
	overwrite := flag.Bool("overwrite", true, "overwrite existing .webp files (default true; use -overwrite=false to skip already-converted images)")
	concurrency := flag.Int("concurrency", 0, "parallel workers (0 = NumCPU)")
	flag.Parse()

	vips.Startup(nil)
	defer vips.Shutdown()

	result, err := image.Run(context.Background(), *dir, *quality, *maxDim, *concurrency, *overwrite)
	if err != nil {
		fmt.Fprintf(os.Stderr, "error: %v\n", err)
		os.Exit(1)
	}

	w := tabwriter.NewWriter(os.Stdout, 0, 0, 2, ' ', 0)
	fmt.Fprintln(w, "STATUS\tORIG KB\tOPT KB\tSAVINGS\tFILE")
	for _, f := range result.Files {
		status := "ok"
		switch {
		case f.Error != "":
			status = "ERR"
		case f.Skipped:
			status = "skip"
		}
		fmt.Fprintf(w, "%s\t%d\t%d\t%.1f%%\t%s\n",
			status, f.OriginalKB, f.OptimizedKB, f.SavingsPct, f.Original)
	}
	w.Flush()

	fmt.Printf("\nfound=%d  converted=%d  skipped=%d  failed=%d  savings=%.1f%%\n",
		result.TotalFound, result.Converted, result.Skipped, result.Failed, result.TotalSavings)
}
