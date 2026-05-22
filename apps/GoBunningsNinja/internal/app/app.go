package app

import (
	"context"
	"errors"
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

const version = "v0.5"

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
	if args[0] == "commands" || args[0] == "extended-help" || args[0] == "manual" {
		a.extendedUsage()
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
	if args[0] == "commands" || args[0] == "extended-help" || args[0] == "manual" {
		a.extendedUsage()
		return 0
	}
	cfg, err := config.FromEnvAndFile(globals.ConfigPath)
	if err != nil {
		fmt.Fprintln(a.Err, "config error:", err)
		return 2
	}
	switch args[0] {
	case "sync", "add-in":
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
			return a.runSyncNamespace(ctx, svc, args[1:])
		case "add-in":
			fmt.Fprintln(a.Err, "deprecated: `add-in` moved to `sync import <IN>`")
			return a.runAddIN(ctx, svc, args[1:])
		case "search":
			return a.runSearch(ctx, svc, args[1:])
		}
	case "bunnings":
		if err := cfg.ValidateBunnings(); err != nil {
			fmt.Fprintln(a.Err, "config error:", err)
			return 2
		}
		bn, err := bunnings.New(cfg)
		if err != nil {
			fmt.Fprintln(a.Err, "bunnings client error:", err)
			return 2
		}
		return a.runBunnings(ctx, bn, args[1:])
	case "search":
		fmt.Fprintln(a.Err, "deprecated: top-level search moved to `sync search` (guarded import workflow) or `bunnings find` (product discovery)")
		return 2
	case "ninja":
		if err := cfg.ValidateInvoiceNinja(); err != nil {
			fmt.Fprintln(a.Err, "config error:", err)
			return 2
		}
		nj, err := ninja.New(cfg)
		if err != nil {
			fmt.Fprintln(a.Err, "invoice ninja client error:", err)
			return 2
		}
		return a.runNinja(ctx, nj, args[1:])
	case "ninja-products-export", "ninja-products-import", "ninja-clients-export", "ninja-clients-import":
		fmt.Fprintln(a.Err, "this command form has been replaced; use `bunnings-ninja ninja export ...` or `bunnings-ninja ninja import ...`")
		return 2
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

func (a App) runSyncNamespace(ctx context.Context, svc syncer.Service, args []string) int {
	if len(args) == 0 {
		return a.runSync(ctx, svc, args)
	}
	switch args[0] {
	case "refresh":
		return a.runSync(ctx, svc, args[1:])
	case "import":
		return a.runAddIN(ctx, svc, args[1:])
	case "search":
		return a.runSearch(ctx, svc, args[1:])
	default:
		return a.runSync(ctx, svc, args)
	}
}

func (a App) runSync(ctx context.Context, svc syncer.Service, args []string) int {
	fs := flag.NewFlagSet("sync", flag.ContinueOnError)
	fs.SetOutput(a.Err)
	commit := fs.Bool("commit", false, "commit changes to Invoice Ninja")
	web := fs.Bool("web", false, "use website-derived Bunnings data instead of the API")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 0 {
		fmt.Fprintln(a.Err, "usage: bunnings-ninja sync refresh [--web] [--commit]")
		return 2
	}
	svc.Bunnings.WithWeb(*web)
	svc.DryRun = !*commit
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
	commit := fs.Bool("commit", false, "commit changes to Invoice Ninja")
	web := fs.Bool("web", false, "use website-derived Bunnings data instead of the API")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() != 1 {
		fmt.Fprintln(a.Err, "usage: bunnings-ninja sync import [--web] [--commit] <bunnings-in>")
		return 2
	}
	svc.Bunnings.WithWeb(*web)
	svc.DryRun = !*commit
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
	commit := fs.Bool("commit", false, "commit selected product changes to Invoice Ninja")
	web := fs.Bool("web", false, "use website-derived Bunnings data instead of the API")
	if err := fs.Parse(args); err != nil {
		return 2
	}
	if fs.NArg() < 1 {
		fmt.Fprintln(a.Err, "usage: bunnings-ninja sync search [--web] [--limit=10] [--create --select=IN1,IN2 --commit] <query>")
		return 2
	}
	query := strings.Join(fs.Args(), " ")
	svc.Bunnings.WithWeb(*web)
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
	svc.DryRun = !*commit
	results := svc.AddProducts(ctx, selected)
	printResults(a.Out, results)
	return exitCode(results)
}

func (a App) runBunnings(ctx context.Context, svc *bunnings.Service, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(a.Err, "usage: bunnings-ninja bunnings <find|get|lookup> ...")
		return 2
	}
	switch args[0] {
	case "find":
		fs := flag.NewFlagSet("bunnings find", flag.ContinueOnError)
		fs.SetOutput(a.Err)
		limit := fs.Int("limit", 10, "maximum results (default 10, max 25)")
		web := fs.Bool("web", false, "use website-derived Bunnings data instead of the API")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if fs.NArg() < 1 {
			fmt.Fprintln(a.Err, "usage: bunnings-ninja bunnings find [--web] [--limit=10] <query>")
			return 2
		}
		svc.WithWeb(*web)
		products, err := svc.Search(ctx, strings.Join(fs.Args(), " "), *limit)
		if err != nil {
			fmt.Fprintln(a.Err, "find error:", err)
			return 1
		}
		hydrated := make([]bunnings.Product, 0, len(products))
		for _, p := range products {
			hp, err := svc.Hydrate(ctx, p)
			if err == nil {
				hydrated = append(hydrated, hp)
			} else {
				hydrated = append(hydrated, p)
			}
		}
		printProductsCSV(a.Out, hydrated)
		return 0
	case "get":
		fs := flag.NewFlagSet("bunnings get", flag.ContinueOnError)
		fs.SetOutput(a.Err)
		web := fs.Bool("web", false, "use website-derived Bunnings data instead of the API")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if fs.NArg() < 1 {
			fmt.Fprintln(a.Err, "usage: bunnings-ninja bunnings get [--web] <IN...>")
			return 2
		}
		svc.WithWeb(*web)
		rows := make([]bunnings.Product, 0, fs.NArg())
		for _, item := range fs.Args() {
			p, err := svc.GetProduct(ctx, item)
			if err != nil {
				rows = append(rows, bunnings.Product{ItemNumber: item, Description: err.Error()})
				continue
			}
			rows = append(rows, p)
		}
		printProductsCSV(a.Out, rows)
		return 0
	case "lookup":
		fs := flag.NewFlagSet("bunnings lookup", flag.ContinueOnError)
		fs.SetOutput(a.Err)
		web := fs.Bool("web", false, "use website-derived Bunnings data instead of the API")
		if err := fs.Parse(args[1:]); err != nil {
			return 2
		}
		if fs.NArg() < 1 {
			fmt.Fprintln(a.Err, "usage: bunnings-ninja bunnings lookup [--web] <IN...>")
			return 2
		}
		svc.WithWeb(*web)
		for _, item := range fs.Args() {
			p, err := svc.GetProduct(ctx, item)
			if err != nil {
				fmt.Fprintf(a.Out, "IN: %s\nError: %v\n\n", item, err)
				continue
			}
			printProductDetail(a.Out, p)
		}
		return 0
	default:
		fmt.Fprintln(a.Err, "unknown bunnings subcommand:", args[0])
		return 2
	}
}

func (a App) runNinja(ctx context.Context, svc *ninja.Service, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(a.Err, "usage: bunnings-ninja ninja <export|import> ...")
		return 2
	}
	switch args[0] {
	case "export":
		return a.runNinjaExport(ctx, svc, args[1:])
	case "import":
		return a.runNinjaImport(ctx, svc, args[1:])
	default:
		fmt.Fprintln(a.Err, "unknown ninja subcommand:", args[0])
		fmt.Fprintln(a.Err, "usage: bunnings-ninja ninja <export|import> ...")
		return 2
	}
}

func (a App) runNinjaExport(ctx context.Context, svc *ninja.Service, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(a.Err, "usage: bunnings-ninja ninja export <products|clients|quotes|invoices|payments> <file|-> [--commit]")
		return 2
	}
	kind := args[0]
	outPath, commit, err := parseExportArgs(args[1:])
	if err != nil {
		fmt.Fprintln(a.Err, err)
		fmt.Fprintf(a.Err, "usage: bunnings-ninja ninja export %s <file|-> [--commit]\n", kind)
		return 2
	}
	w, closeFn, err := writerFor(outPath, a.Out, commit)
	if err != nil {
		fmt.Fprintln(a.Err, "output error:", err)
		return 1
	}
	defer closeFn()
	var exportErr error
	switch kind {
	case "products":
		exportErr = svc.ExportProductsCSV(ctx, w)
	case "clients":
		exportErr = svc.ExportClientsCSV(ctx, w)
	case "quotes":
		exportErr = svc.ExportQuotesCSV(ctx, w)
	case "invoices":
		exportErr = svc.ExportInvoicesCSV(ctx, w)
	case "payments":
		exportErr = svc.ExportPaymentsCSV(ctx, w)
	default:
		fmt.Fprintln(a.Err, "unknown export target:", kind)
		return 2
	}
	if exportErr != nil {
		fmt.Fprintln(a.Err, "export error:", exportErr)
		return 1
	}
	return 0
}

func (a App) runNinjaImport(ctx context.Context, svc *ninja.Service, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(a.Err, "usage: bunnings-ninja ninja import <products|clients> <file|-> [--commit]")
		return 2
	}
	kind := args[0]
	inPath, dryRun, err := parseImportArgs(args[1:])
	if err != nil {
		fmt.Fprintln(a.Err, err)
		fmt.Fprintf(a.Err, "usage: bunnings-ninja ninja import %s <file|-> [--commit]\n", kind)
		return 2
	}
	r, closeFn, err := readerFor(inPath, os.Stdin)
	if err != nil {
		fmt.Fprintln(a.Err, "input error:", err)
		return 1
	}
	defer closeFn()
	var results []ninja.CSVImportResult
	switch kind {
	case "products":
		results, err = svc.ImportProductsCSV(ctx, r, dryRun)
	case "clients":
		results, err = svc.ImportClientsCSV(ctx, r, dryRun)
	case "quotes", "invoices", "payments":
		fmt.Fprintf(a.Err, "ninja import %s is not supported; exports only for this target\n", kind)
		return 2
	default:
		fmt.Fprintln(a.Err, "unknown import target:", kind)
		return 2
	}
	if err != nil {
		fmt.Fprintln(a.Err, "import error:", err)
		return 1
	}
	printCSVImportResults(a.Out, results)
	return csvImportExitCode(results)
}

func parseExportArgs(args []string) (string, bool, error) {
	commit := false
	var paths []string
	for _, arg := range args {
		switch {
		case arg == "--commit":
			commit = true
		case strings.HasPrefix(arg, "-"):
			return "", false, fmt.Errorf("unknown export flag %q", arg)
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) != 1 {
		return "", false, fmt.Errorf("expected exactly one export path, got %d", len(paths))
	}
	return paths[0], commit, nil
}

func parseImportArgs(args []string) (string, bool, error) {
	dryRun := true
	var paths []string
	for _, arg := range args {
		switch {
		case arg == "--commit":
			dryRun = false
		case strings.HasPrefix(arg, "-"):
			return "", true, fmt.Errorf("unknown import flag %q", arg)
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) != 1 {
		return "", true, fmt.Errorf("expected exactly one import path, got %d", len(paths))
	}
	return paths[0], dryRun, nil
}

func readerFor(path string, stdin io.Reader) (io.Reader, func(), error) {
	if path == "-" {
		return stdin, func() {}, nil
	}
	if strings.TrimSpace(path) == "" {
		return nil, func() {}, fmt.Errorf("import path is required")
	}
	f, err := os.Open(path)
	if err != nil {
		if errors.Is(err, os.ErrNotExist) {
			return nil, func() {}, fmt.Errorf("import file does not exist: %s", path)
		}
		return nil, func() {}, fmt.Errorf("open import file %s: %w", path, err)
	}
	return f, func() { _ = f.Close() }, nil
}

func writerFor(path string, stdout io.Writer, commit bool) (io.Writer, func(), error) {
	if path == "-" {
		return stdout, func() {}, nil
	}
	if strings.TrimSpace(path) == "" {
		return nil, func() {}, fmt.Errorf("export path is required")
	}
	flags := os.O_WRONLY | os.O_CREATE
	if commit {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, func() {}, fmt.Errorf("refusing to overwrite existing file %s; use --commit", path)
		}
		return nil, func() {}, fmt.Errorf("open export file %s: %w", path, err)
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

func printProductsCSV(w io.Writer, products []bunnings.Product) {
	fmt.Fprintln(w, "IN,Description,Unit,PricePerUnit,ImageURL")
	for _, p := range products {
		fmt.Fprintf(w, "%s,%s,%s,%.2f,%s\n", p.ItemNumber, sanitizeCSV(p.Description), sanitizeCSV(p.Unit), p.Price, sanitizeCSV(p.ImageURL))
	}
}

func printProductDetail(w io.Writer, p bunnings.Product) {
	fmt.Fprintf(w, "IN: %s\nTitle: %s\nDescription: %s\nUnit: %s\nPricePerUnit: %.2f\nImageURL: %s\n\n", p.ItemNumber, p.Title, p.Description, p.Unit, p.Price, p.ImageURL)
}

func sanitizeCSV(v string) string {
	v = strings.TrimSpace(v)
	v = strings.ReplaceAll(v, "\"", "\"\"")
	return strings.ReplaceAll(v, ",", " ")
}

func printProducts(w io.Writer, products []bunnings.Product) {
	fmt.Fprintln(w, "Bunnings search results")
	fmt.Fprintln(w, "IN\tTitle")
	for _, p := range products {
		fmt.Fprintf(w, "%s\t%s\n", p.ItemNumber, p.Title)
	}
	fmt.Fprintln(w, "\nPreview only. To import, re-run with --create --select=IN1,IN2 --commit")
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

Version: v0.5

Global options:
  --config <path>       Optional key=value config file. File values override environment variables.
                        If omitted, GOBUNNINGSNINJA_CONFIG is used, then ./gobunningsninja.conf if present.

Commands:
  bunnings find <query>                 Fuzzy Bunnings discovery (CSV output).
  bunnings get <IN...>                  Exact Bunnings lookup (CSV output).
  bunnings lookup <IN...>               Exact Bunnings lookup (human-readable output).
  sync refresh                          Preview linked product refresh; use --commit to update.
  sync import <IN>                      Preview one product import; use --commit to update.
  sync search <query>                   Guarded Bunnings search/import workflow for Invoice Ninja.
  ninja export products <file|->        Export Invoice Ninja products as CSV; use --commit to overwrite.
  ninja import products <file|->        Preview product CSV changes; use --commit to update.
  ninja export clients <file|->         Export Invoice Ninja clients as CSV; use --commit to overwrite.
  ninja import clients <file|->         Preview client CSV changes; use --commit to update.
  ninja export quotes <file|->          Export Invoice Ninja quotes as CSV; use --commit to overwrite.
  ninja export invoices <file|->        Export Invoice Ninja invoices as CSV; use --commit to overwrite.
  ninja export payments <file|->        Export Invoice Ninja payments as CSV; use --commit to overwrite.
  commands                              Show extended command help with output examples.
  version                               Print version.

Examples:
  bunnings-ninja commands
  bunnings-ninja bunnings find "merbau decking" --limit=10
  bunnings-ninja bunnings get 0123456 0987654
  bunnings-ninja bunnings lookup 0123456
  bunnings-ninja sync refresh --commit
  bunnings-ninja sync import --commit 0123456
  bunnings-ninja sync search "merbau decking" --limit=10
  bunnings-ninja sync search "merbau decking" --create --select=0123456,0987654 --commit
  bunnings-ninja ninja export products products.csv
  bunnings-ninja ninja export products -
  bunnings-ninja ninja export products products.csv --commit
  bunnings-ninja ninja import products products.csv
  bunnings-ninja ninja import products --commit products.csv
  bunnings-ninja ninja export clients clients.csv
  bunnings-ninja ninja import clients --commit clients.csv
  bunnings-ninja ninja export quotes quotes.csv
  bunnings-ninja ninja export invoices invoices.csv
  bunnings-ninja ninja export payments payments.csv

Required configuration for ninja commands:
  INVOICE_NINJA_TOKEN

Additional required configuration for Bunnings and sync commands:
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
