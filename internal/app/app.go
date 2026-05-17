package app

import (
	"context"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strings"

	"github.com/MickMake/GoBunningsNinja/internal/bunnings"
	"github.com/MickMake/GoBunningsNinja/internal/config"
	"github.com/MickMake/GoBunningsNinja/internal/ninja"
	"github.com/MickMake/GoBunningsNinja/internal/syncer"
)

const version = "0.1"

type App struct {
	Out io.Writer
	Err io.Writer
}

type globalOptions struct {
	ConfigPath string
}

func (a App) Run(ctx context.Context, args []string) int {
	if a.Out == nil {
		a.Out = os.Stdout
	}
	if a.Err == nil {
		a.Err = os.Stderr
	}
	if len(args) == 0 || args[0] == "help" || args[0] == "--help" || args[0] == "-h" {
		a.usage()
		return 0
	}
	if args[0] == "version" || args[0] == "--version" {
		fmt.Fprintln(a.Out, version)
		return 0
	}
	globals, args, err := parseGlobals(args)
	if err != nil {
		fmt.Fprintln(a.Err, err)
		return 2
	}
	if len(args) == 0 {
		a.usage()
		return 0
	}
	cfg, err := config.FromEnvAndFile(globals.ConfigPath)
	if err != nil {
		fmt.Fprintln(a.Err, "config error:", err)
		return 2
	}
	switch args[0] {
	case "sync", "add-in", "search":
		if err := cfg.Validate(); err != nil {
			fmt.Fprintln(a.Err, "config error:", err)
			return 2
		}
		bn, err := bunnings.New(cfg)
		if err != nil {
			fmt.Fprintln(a.Err, "bunnings client error:", err)
			return 2
		}
		nj, err := ninja.New(cfg)
		if err != nil {
			fmt.Fprintln(a.Err, "invoice ninja client error:", err)
			return 2
		}
		svc := syncer.Service{Bunnings: bn, Ninja: nj, BunningsCustom: cfg.BunningsCustom}
		switch args[0] {
		case "sync":
			return a.runSync(ctx, svc, args[1:])
		case "add-in":
			return a.runAddIN(ctx, svc, args[1:])
		case "search":
			return a.runSearch(ctx, svc, args[1:])
		}
	case "ninja-products-export", "ninja-products-import", "ninja-clients-export", "ninja-clients-import":
		if err := cfg.ValidateInvoiceNinja(); err != nil {
			fmt.Fprintln(a.Err, "config error:", err)
			return 2
		}
		nj, err := ninja.New(cfg)
		if err != nil {
			fmt.Fprintln(a.Err, "invoice ninja client error:", err)
			return 2
		}
		switch args[0] {
		case "ninja-products-export":
			return a.runNinjaProductsExport(ctx, nj, args[1:])
		case "ninja-products-import":
			return a.runNinjaProductsImport(ctx, nj, args[1:])
		case "ninja-clients-export":
			return a.runNinjaClientsExport(ctx, nj, args[1:])
		case "ninja-clients-import":
			return a.runNinjaClientsImport(ctx, nj, args[1:])
		}
	default:
		fmt.Fprintln(a.Err, "unknown command:", args[0])
		a.usage()
		return 2
	}
	return 2
}

func parseGlobals(args []string) (globalOptions, []string, error) {
	var opts globalOptions
	var rest []string
	for i := 0; i < len(args); i++ {
		a := args[i]
		switch {
		case a == "--config" || a == "-config":
			if i+1 >= len(args) {
				return opts, nil, fmt.Errorf("--config requires a path")
			}
			i++
			opts.ConfigPath = args[i]
		case strings.HasPrefix(a, "--config="):
			opts.ConfigPath = strings.TrimPrefix(a, "--config=")
		default:
			rest = append(rest, a)
		}
	}
	return opts, rest, nil
}

func (a App) runSync(ctx context.Context, svc syncer.Service, args []string) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	dryRun := fs.Bool("dry-run", true, "preview changes without updating Invoice Ninja")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	svc.DryRun = *dryRun
	results, err := svc.SyncExisting(ctx)
	if err != nil {
		fmt.Fprintln(a.Err, "sync error:", err)
		return 1
	}
	printResults(a.Out, results)
	return exitCode(results)
}

func (a App) runAddIN(ctx context.Context, svc syncer.Service, args []string) int {
	fs := flag.NewFlagSet("add-in", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	dryRun := fs.Bool("dry-run", true, "preview changes without updating Invoice Ninja")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(a.Err, "usage: bunnings-ninja add-in [--dry-run=false] <bunnings-in>")
		return 2
	}
	svc.DryRun = *dryRun
	res := svc.AddByIN(ctx, fs.Arg(0))
	printResults(a.Out, []syncer.Result{res})
	return exitCode([]syncer.Result{res})
}

func (a App) runSearch(ctx context.Context, svc syncer.Service, args []string) int {
	fs := flag.NewFlagSet("search", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	limit := fs.Int("limit", 10, "maximum search results to preview or import; hard-capped at 25")
	create := fs.Bool("create", false, "create/update selected products in Invoice Ninja")
	selectCSV := fs.String("select", "", "comma-separated Bunnings item numbers to import from the search results")
	all := fs.Bool("all", false, "import all returned results up to --limit; requires --yes")
	yes := fs.Bool("yes", false, "confirm a guarded bulk import")
	dryRun := fs.Bool("dry-run", true, "preview changes without updating Invoice Ninja")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(a.Err, "usage: bunnings-ninja search [--limit=10] [--create --select=IN1,IN2 --dry-run=false] <query>")
		return 2
	}
	query := strings.Join(fs.Args(), " ")
	products, err := svc.Search(ctx, query, *limit)
	if err != nil {
		fmt.Fprintln(a.Err, "search error:", err)
		return 1
	}
	if !*create {
		printProducts(a.Out, products)
		return 0
	}
	selected := selectProducts(products, *selectCSV, *all)
	if len(selected) == 0 {
		fmt.Fprintln(a.Err, "no products selected; use --select=IN1,IN2 or --all --yes")
		return 2
	}
	if *all && !*yes {
		fmt.Fprintln(a.Err, "refusing --all without --yes; the goblin at the gate is doing its job")
		return 2
	}
	svc.DryRun = *dryRun
	results := svc.AddProducts(ctx, selected)
	printResults(a.Out, results)
	return exitCode(results)
}

func (a App) runNinjaProductsExport(ctx context.Context, svc *ninja.Service, args []string) int {
	fs := flag.NewFlagSet("ninja-products-export", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	outPath := fs.String("out", "-", "output CSV path; use - for stdout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	w, closeFn, err := writerFor(*outPath, a.Out)
	if err != nil {
		fmt.Fprintln(a.Err, "output error:", err)
		return 1
	}
	defer closeFn()
	if err := svc.ExportProductsCSV(ctx, w); err != nil {
		fmt.Fprintln(a.Err, "export error:", err)
		return 1
	}
	return 0
}

func (a App) runNinjaProductsImport(ctx context.Context, svc *ninja.Service, args []string) int {
	fs := flag.NewFlagSet("ninja-products-import", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	dryRun := fs.Bool("dry-run", true, "preview changes without updating Invoice Ninja")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(a.Err, "usage: bunnings-ninja ninja-products-import [--dry-run=false] <products.csv|->")
		return 2
	}
	r, closeFn, err := readerFor(fs.Arg(0), os.Stdin)
	if err != nil {
		fmt.Fprintln(a.Err, "input error:", err)
		return 1
	}
	defer closeFn()
	results, err := svc.ImportProductsCSV(ctx, r, *dryRun)
	if err != nil {
		fmt.Fprintln(a.Err, "import error:", err)
		return 1
	}
	printCSVImportResults(a.Out, results)
	return csvImportExitCode(results)
}

func (a App) runNinjaClientsExport(ctx context.Context, svc *ninja.Service, args []string) int {
	fs := flag.NewFlagSet("ninja-clients-export", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	outPath := fs.String("out", "-", "output CSV path; use - for stdout")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	w, closeFn, err := writerFor(*outPath, a.Out)
	if err != nil {
		fmt.Fprintln(a.Err, "output error:", err)
		return 1
	}
	defer closeFn()
	if err := svc.ExportClientsCSV(ctx, w); err != nil {
		fmt.Fprintln(a.Err, "export error:", err)
		return 1
	}
	return 0
}

func (a App) runNinjaClientsImport(ctx context.Context, svc *ninja.Service, args []string) int {
	fs := flag.NewFlagSet("ninja-clients-import", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	dryRun := fs.Bool("dry-run", true, "preview changes without updating Invoice Ninja")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(a.Err, "usage: bunnings-ninja ninja-clients-import [--dry-run=false] <clients.csv|->")
		return 2
	}
	r, closeFn, err := readerFor(fs.Arg(0), os.Stdin)
	if err != nil {
		fmt.Fprintln(a.Err, "input error:", err)
		return 1
	}
	defer closeFn()
	results, err := svc.ImportClientsCSV(ctx, r, *dryRun)
	if err != nil {
		fmt.Fprintln(a.Err, "import error:", err)
		return 1
	}
	printCSVImportResults(a.Out, results)
	return csvImportExitCode(results)
}

func readerFor(path string, stdin io.Reader) (io.Reader, func(), error) {
	if path == "-" {
		return stdin, func() {}, nil
	}
	f, err := os.Open(path)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { _ = f.Close() }, nil
}

func writerFor(path string, stdout io.Writer) (io.Writer, func(), error) {
	if path == "-" {
		return stdout, func() {}, nil
	}
	f, err := os.Create(path)
	if err != nil {
		return nil, func() {}, err
	}
	return f, func() { _ = f.Close() }, nil
}

func printCSVImportResults(w io.Writer, results []ninja.CSVImportResult) {
	fmt.Fprintln(w, "ID\tName\tAction\tChanges/Error")
	for _, r := range results {
		detail := strings.Join(r.Changes, ",")
		if r.Error != nil {
			detail = r.Error.Error()
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.ID, r.Name, r.Action, detail)
	}
}

func csvImportExitCode(results []ninja.CSVImportResult) int {
	for _, r := range results {
		if r.Error != nil {
			return 1
		}
	}
	return 0
}

func selectProducts(products []bunnings.Product, csv string, all bool) []bunnings.Product {
	if all {
		return products
	}
	wanted := map[string]bool{}
	for _, v := range strings.Split(csv, ",") {
		v = strings.TrimSpace(v)
		if v != "" {
			wanted[v] = true
		}
	}
	var selected []bunnings.Product
	for _, p := range products {
		if wanted[p.ItemNumber] {
			selected = append(selected, p)
		}
	}
	return selected
}

func printProducts(w io.Writer, products []bunnings.Product) {
	fmt.Fprintln(w, "Bunnings search results")
	fmt.Fprintln(w, "IN\tTitle")
	for _, p := range products {
		fmt.Fprintf(w, "%s\t%s\n", p.ItemNumber, p.Title)
	}
	fmt.Fprintln(w, "\nPreview only. To import, re-run with --create --select=IN1,IN2 --dry-run=false")
}

func printResults(w io.Writer, results []syncer.Result) {
	sort.SliceStable(results, func(i, j int) bool { return results[i].ItemNumber < results[j].ItemNumber })
	fmt.Fprintln(w, "IN\tProductKey\tAction\tChanges/Error")
	for _, r := range results {
		detail := strings.Join(r.Changes, ",")
		if r.Error != nil {
			detail = r.Error.Error()
		}
		fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.ItemNumber, r.ProductKey, r.Action, detail)
	}
}

func exitCode(results []syncer.Result) int {
	for _, r := range results {
		if r.Error != nil {
			return 1
		}
	}
	return 0
}

func (a App) usage() {
	fmt.Fprintln(a.Out, `bunnings-ninja syncs Bunnings products into Invoice Ninja.

Version: 0.1

Global options:
  --config <path>       Optional key=value config file. File values override environment variables.
                        If omitted, GOBUNNINGSNINJA_CONFIG is used, then ./gobunningsninja.conf if present.

Commands:
  sync                         Refresh existing Invoice Ninja products linked to Bunnings INs.
  add-in <IN>                  Add or refresh one product by Bunnings item number.
  search <query>               Preview Bunnings search results; optionally import selected results.
  ninja-products-export        Export Invoice Ninja products as CSV.
  ninja-products-import <CSV>   Import product CSV changes; dry-run by default.
  ninja-clients-export         Export Invoice Ninja clients as CSV.
  ninja-clients-import <CSV>    Import client CSV changes; dry-run by default.
  version                      Print version.

Examples:
  bunnings-ninja --config ./gobunningsninja.conf sync
  bunnings-ninja sync --dry-run=false
  bunnings-ninja add-in --dry-run=false 0123456
  bunnings-ninja search "merbau decking" --limit=10
  bunnings-ninja search "merbau decking" --create --select=0123456,0987654 --dry-run=false
  bunnings-ninja ninja-products-export --out products.csv
  bunnings-ninja ninja-products-import products.csv
  bunnings-ninja ninja-products-import --dry-run=false products.csv
  bunnings-ninja ninja-clients-export --out clients.csv
  bunnings-ninja ninja-clients-import --dry-run=false clients.csv

Required configuration for ninja-* commands:
  INVOICE_NINJA_TOKEN

Additional required configuration for Bunnings sync/search commands:
  BUNNINGS_CLIENT_ID
  BUNNINGS_CLIENT_SECRET

Useful optional configuration:
  INVOICE_NINJA_URL              default empty; uses GoInvoiceNinja default
  BUNNINGS_ENV                   live, test, sandbox; default live
  BUNNINGS_COUNTRY               AU or NZ; default AU
  BUNNINGS_LOCATION              location code; required for price refresh
  BUNNINGS_SCOPES                optional scopes, space or comma separated
  PRODUCT_PREFIX                 default BUNNINGS-
  BUNNINGS_IN_CUSTOM_FIELD       default 1
  BUNNINGS_IMAGE_CUSTOM_FIELD    default 2
  TAX_NAME                       default GST
  TAX_RATE                       default 10`)
}
