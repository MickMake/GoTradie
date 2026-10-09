package app

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"os"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/MickMake/GoTradie/internal/bunnings"
	"github.com/MickMake/GoTradie/internal/config"
	"github.com/MickMake/GoTradie/internal/ninja"
	"github.com/MickMake/GoTradie/internal/syncer"
)

const version = "v0.5.4"

var errExpenseImportStopped = errors.New("expense import stopped by operator")

type App struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
	Now func() time.Time
}

func (a App) Run(ctx context.Context, args []string) int {
	if a.Out == nil {
		a.Out = os.Stdout
	}
	if a.In == nil {
		a.In = os.Stdin
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
	if hasLegacyConfigFlag(args) {
		fmt.Fprintln(a.Err, "--config was removed in v0.5.3; use ~/.GoTradie/config.yaml")
		return 2
	}
	cfg, err := config.Load()
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
		svc := syncer.Service{Bunnings: bn, Ninja: nj, BunningsCustom: cfg.ProductSync.CustomFields.BunningsIN}
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
		return a.runNinja(ctx, nj, cfg, args[1:])
	case "ninja-products-export", "ninja-products-import", "ninja-clients-export", "ninja-clients-import":
		fmt.Fprintln(a.Err, "this command form has been replaced; use `GoTradie ninja export ...` or `GoTradie ninja import ...`")
		return 2
	default:
		fmt.Fprintln(a.Err, "unknown command:", args[0])
		a.usage()
		return 2
	}
	return 2
}

func hasLegacyConfigFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--config" || arg == "-config" || strings.HasPrefix(arg, "--config=") {
			return true
		}
	}
	return false
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
		fmt.Fprintln(a.Err, "usage: GoTradie sync refresh [--web] [--commit]")
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
		fmt.Fprintln(a.Err, "usage: GoTradie sync import [--web] [--commit] <bunnings-in>")
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
		fmt.Fprintln(a.Err, "usage: GoTradie sync search [--web] [--limit=10] [--create --select=IN1,IN2 --commit] <query>")
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
		fmt.Fprintln(a.Err, "usage: GoTradie bunnings <find|get|lookup> ...")
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
			fmt.Fprintln(a.Err, "usage: GoTradie bunnings find [--web] [--limit=10] <query>")
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
			fmt.Fprintln(a.Err, "usage: GoTradie bunnings get [--web] <IN...>")
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
			fmt.Fprintln(a.Err, "usage: GoTradie bunnings lookup [--web] <IN...>")
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

func (a App) runNinja(ctx context.Context, svc *ninja.Service, cfg config.Config, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(a.Err, "usage: GoTradie ninja <export|import> ...")
		return 2
	}
	switch args[0] {
	case "export":
		return a.runNinjaExport(ctx, svc, cfg, args[1:])
	case "import":
		return a.runNinjaImport(ctx, svc, args[1:])
	default:
		fmt.Fprintln(a.Err, "unknown ninja subcommand:", args[0])
		fmt.Fprintln(a.Err, "usage: GoTradie ninja <export|import> ...")
		return 2
	}
}

func (a App) runNinjaExport(ctx context.Context, svc *ninja.Service, cfg config.Config, args []string) int {
	if len(args) < 1 {
		fmt.Fprintln(a.Err, "usage: GoTradie ninja export <products|clients|quotes|invoices|payments|bas> ...")
		return 2
	}
	kind := args[0]
	if kind == "tax" {
		return a.runNinjaTaxExport(ctx, svc, args[1:])
	}
	if kind == "bas" {
		return a.runNinjaBASExport(ctx, svc, cfg, args[1:])
	}

	outPath, force, err := parseExportArgs(args[1:])
	if err != nil {
		fmt.Fprintln(a.Err, err)
		fmt.Fprintf(a.Err, "usage: GoTradie ninja export %s <file|-> [--force]\n", kind)
		return 2
	}
	w, closeFn, err := writerFor(outPath, a.Out, force)
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
		fmt.Fprintln(a.Err, "usage: GoTradie ninja import <products|clients|expenses> <file|-> [--commit]")
		return 2
	}
	kind := args[0]
	var (
		inPath       string
		dryRun       bool
		receiptsRoot string
		batchSize    int
		pause        bool
		err          error
	)
	if kind == "expenses" {
		inPath, receiptsRoot, batchSize, pause, dryRun, err = parseExpenseImportArgs(args[1:])
	} else {
		inPath, dryRun, err = parseImportArgs(args[1:])
	}
	if err != nil {
		fmt.Fprintln(a.Err, err)
		if kind == "expenses" {
			fmt.Fprintln(a.Err, "usage: GoTradie ninja import expenses <file> [--receipts-root <dir>] [--batch-size <n>] [--pause] [--commit]")
		} else {
			fmt.Fprintf(a.Err, "usage: GoTradie ninja import %s <file|-> [--commit]\n", kind)
		}
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
	case "expenses":
		pauseReader := bufio.NewReader(a.In)
		results, err = svc.ImportExpensesCSVWithOptions(ctx, r, ninja.ExpenseImportOptions{
			DryRun:       dryRun,
			ReceiptsRoot: receiptsRoot,
			BatchSize:    batchSize,
			OnPreflight: func(report ninja.ExpenseImportPreflight) {
				printExpensePreflight(a.Out, a.Err, report)
			},
			OnProgress: func(progress ninja.ExpenseImportProgress) {
				printExpenseProgress(a.Out, a.Err, progress)
			},
			OnBatchComplete: func(summary ninja.ExpenseImportBatchSummary) error {
				if dryRun {
					fmt.Fprintf(a.Out, "Batch %d previewed: %d would create, %d existing, %d would update, %d deferred, %d errors\n", summary.Number, summary.New, summary.Existing, summary.Updated, summary.Deferred, summary.Errors)
				} else {
					fmt.Fprintf(a.Out, "Batch %d complete: %d new, %d existing, %d updated, %d deferred, %d errors\n", summary.Number, summary.New, summary.Existing, summary.Updated, summary.Deferred, summary.Errors)
				}
				if !pause || summary.Last {
					return nil
				}
				fmt.Fprint(a.Out, "Continue? [Y/n] ")
				keepGoing, readErr := readExpenseBatchDecision(pauseReader)
				if readErr != nil {
					return readErr
				}
				if keepGoing {
					return nil
				}
				return errExpenseImportStopped
			},
		})
	case "quotes", "invoices", "payments":
		fmt.Fprintf(a.Err, "ninja import %s is not supported; exports only for this target\n", kind)
		return 2
	default:
		fmt.Fprintln(a.Err, "unknown import target:", kind)
		return 2
	}
	if err != nil {
		if kind == "expenses" && errors.Is(err, errExpenseImportStopped) {
			fmt.Fprintln(a.Out, "Import stopped by operator")
			printExpenseImportSummary(a.Out, results, dryRun, false)
			return csvImportExitCode(results)
		}
		fmt.Fprintln(a.Err, "import error:", err)
		return 1
	}
	if kind == "expenses" {
		printExpenseImportSummary(a.Out, results, dryRun, true)
		return csvImportExitCode(results)
	}
	printCSVImportResults(a.Out, results)
	return csvImportExitCode(results)
}

func parseExportArgs(args []string) (string, bool, error) {
	force := false
	var paths []string
	for _, arg := range args {
		switch {
		case arg == "--force":
			force = true
		case strings.HasPrefix(arg, "-"):
			return "", false, fmt.Errorf("unknown export flag %q", arg)
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) != 1 {
		return "", false, fmt.Errorf("expected exactly one export path, got %d", len(paths))
	}
	return paths[0], force, nil
}

func parseImportArgs(args []string) (string, bool, error) {
	dryRun := true
	var paths []string
	for _, arg := range args {
		switch {
		case arg == "--commit":
			dryRun = false
		case arg == "-":
			paths = append(paths, arg)
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

func parseExpenseImportArgs(args []string) (string, string, int, bool, bool, error) {
	dryRun := true
	receiptsRoot := ""
	batchSize := 0
	pause := false
	var paths []string
	for i := 0; i < len(args); i++ {
		arg := args[i]
		switch {
		case arg == "--commit":
			dryRun = false
		case arg == "--pause":
			pause = true
		case arg == "--batch-size":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return "", "", 0, false, true, fmt.Errorf("--batch-size requires a positive integer")
			}
			i++
			var parseErr error
			batchSize, parseErr = strconv.Atoi(args[i])
			if parseErr != nil || batchSize <= 0 {
				return "", "", 0, false, true, fmt.Errorf("--batch-size requires a positive integer")
			}
		case strings.HasPrefix(arg, "--batch-size="):
			var parseErr error
			batchSize, parseErr = strconv.Atoi(strings.TrimPrefix(arg, "--batch-size="))
			if parseErr != nil || batchSize <= 0 {
				return "", "", 0, false, true, fmt.Errorf("--batch-size requires a positive integer")
			}
		case arg == "--receipts-root":
			if i+1 >= len(args) || strings.HasPrefix(args[i+1], "-") {
				return "", "", 0, false, true, fmt.Errorf("--receipts-root requires a directory")
			}
			i++
			receiptsRoot = args[i]
		case strings.HasPrefix(arg, "--receipts-root="):
			receiptsRoot = strings.TrimPrefix(arg, "--receipts-root=")
		case arg == "-":
			return "", "", 0, false, true, fmt.Errorf("Expense import requires a named file so the complete source can be preflighted before processing; stdin is not supported")
		case strings.HasPrefix(arg, "-"):
			return "", "", 0, false, true, fmt.Errorf("unknown import flag %q", arg)
		default:
			paths = append(paths, arg)
		}
	}
	if len(paths) != 1 {
		return "", "", 0, false, true, fmt.Errorf("expected exactly one import path, got %d", len(paths))
	}
	if strings.TrimSpace(receiptsRoot) == "" && containsExpenseReceiptFlag(args) {
		return "", "", 0, false, true, fmt.Errorf("receipts root must not be blank")
	}
	if pause && batchSize == 0 {
		return "", "", 0, false, true, fmt.Errorf("--pause requires --batch-size")
	}
	return paths[0], receiptsRoot, batchSize, pause, dryRun, nil
}

func containsExpenseReceiptFlag(args []string) bool {
	for _, arg := range args {
		if arg == "--receipts-root" || strings.HasPrefix(arg, "--receipts-root=") {
			return true
		}
	}
	return false
}

func readExpenseBatchDecision(reader *bufio.Reader) (bool, error) {
	answer, err := reader.ReadString('\n')
	if err != nil {
		if !errors.Is(err, io.EOF) {
			return false, err
		}
		if answer == "" {
			return false, nil
		}
	}
	answer = strings.ToLower(strings.TrimSpace(answer))
	return answer == "" || answer == "y" || answer == "yes", nil
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

func writerFor(path string, stdout io.Writer, force bool) (io.Writer, func(), error) {
	if path == "-" {
		return stdout, func() {}, nil
	}
	if strings.TrimSpace(path) == "" {
		return nil, func() {}, fmt.Errorf("export path is required")
	}
	flags := os.O_WRONLY | os.O_CREATE
	if force {
		flags |= os.O_TRUNC
	} else {
		flags |= os.O_EXCL
	}
	f, err := os.OpenFile(path, flags, 0644)
	if err != nil {
		if errors.Is(err, os.ErrExist) {
			return nil, func() {}, fmt.Errorf("refusing to overwrite existing file %s; use --force", path)
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

func printExpensePreflight(out, errOut io.Writer, report ninja.ExpenseImportPreflight) {
	for _, warning := range report.Warnings {
		fmt.Fprintln(out, "WARNING:", warning.String())
	}
	for _, issue := range report.Errors {
		fmt.Fprintln(errOut, "ERROR:", issue.String())
	}
	fmt.Fprintln(out, "Preflight complete")
	fmt.Fprintf(out, "Rows: %d\nErrors: %d\nWarnings: %d\nMode: %s\n", report.Rows, len(report.Errors), len(report.Warnings), report.Mode)
	if report.Mode == ninja.ExpenseImportModeSettlementSafe {
		fmt.Fprintln(out, "Account Payment rows detected. Using settlement-safe whole-file mode.")
	}
}

func printExpenseProgress(out, errOut io.Writer, progress ninja.ExpenseImportProgress) {
	result := progress.Result
	state := expenseResultState(result)
	line := fmt.Sprintf("%-12s %d/%d  %-10s  %-28s  $%9.2f  %s", state, progress.Index, progress.Total, result.Date, result.Supplier, result.BusinessAmount, result.ImportID)
	if result.Error == nil {
		fmt.Fprintln(out, strings.TrimRight(line, " "))
		return
	}
	fmt.Fprintln(errOut, strings.TrimRight(line, " "))
	if result.ReceiptName != "" {
		fmt.Fprintln(errOut, "             File:", result.ReceiptName)
	}
	if result.RowNo > 0 {
		fmt.Fprintln(errOut, "             CSV row:", result.RowNo)
	}
	fmt.Fprintln(errOut, "             Error:", result.Error)
}

func expenseResultState(result ninja.CSVImportResult) string {
	if result.Error != nil || result.Action == "error" {
		return "ERROR"
	}
	switch result.Action {
	case "created", "created-transaction":
		return "NEW"
	case "would-create", "would-create-transaction":
		return "WOULD-CREATE"
	case "updated":
		return "UPDATED"
	case "would-update", "would-update-transaction":
		return "WOULD-UPDATE"
	case "deferred":
		return "DEFERRED"
	default:
		return "EXISTING"
	}
}

func printExpenseImportSummary(w io.Writer, results []ninja.CSVImportResult, dryRun, complete bool) {
	counts := map[string]int{}
	processed := 0
	for _, result := range results {
		if result.RowNo == 0 {
			continue
		}
		processed++
		counts[expenseResultState(result)]++
	}
	if complete {
		fmt.Fprintln(w, "Import complete")
	} else {
		fmt.Fprintln(w, "Import summary")
	}
	if dryRun {
		fmt.Fprintf(w, "Processed: %d\nWould create: %d\nExisting: %d\nWould update: %d\nDeferred: %d\nErrors: %d\n", processed, counts["NEW"]+counts["WOULD-CREATE"], counts["EXISTING"], counts["UPDATED"]+counts["WOULD-UPDATE"], counts["DEFERRED"], counts["ERROR"])
		return
	}
	fmt.Fprintf(w, "Processed: %d\nNew: %d\nExisting: %d\nUpdated: %d\nDeferred: %d\nErrors: %d\n", processed, counts["NEW"]+counts["WOULD-CREATE"], counts["EXISTING"], counts["UPDATED"]+counts["WOULD-UPDATE"], counts["DEFERRED"], counts["ERROR"])
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
	fmt.Fprint(a.Out, `GoTradie syncs Bunnings products into Invoice Ninja.

Version: v0.5.4

Configuration:
  ~/.GoTradie/config.yaml is mandatory for operational commands.
  INVOICE_NINJA_TOKEN and BUNNINGS_CLIENT_SECRET may override YAML secrets.

Commands:
  bunnings find <query>                 Fuzzy Bunnings discovery (CSV output).
  bunnings get <IN...>                  Exact Bunnings lookup (CSV output).
  bunnings lookup <IN...>               Exact Bunnings lookup (human-readable output).
  sync refresh                          Preview linked product refresh; use --commit to update.
  sync import <IN>                      Preview one product import; use --commit to update.
  sync search <query>                   Guarded Bunnings search/import workflow for Invoice Ninja.
  ninja export products <file|->        Export Invoice Ninja products as CSV; use --force to overwrite.
  ninja import products <file|->        Preview product CSV changes; use --commit to update.
  ninja export clients <file|->         Export Invoice Ninja clients as CSV; use --force to overwrite.
  ninja import clients <file|->         Preview client CSV changes; use --commit to update.
  ninja import expenses <file> [--receipts-root <dir>] [--batch-size <n>] [--pause]
                                         Preflight and preview expenses; use --commit to write/upload.
  ninja export quotes <file|->          Export Invoice Ninja quotes as CSV; use --force to overwrite.
  ninja export invoices <file|->        Export Invoice Ninja invoices as CSV; use --force to overwrite.
  ninja export payments <file|->        Export Invoice Ninja payments as CSV; use --force to overwrite.
  ninja export bas [--fy YYYY] [--period VALUE]
                                         Generate a BAS XLSX workbook; use --force to overwrite.
  commands                              Show extended command help with output examples.
  version                               Print version.

Examples:
  GoTradie commands
  GoTradie bunnings find "merbau decking" --limit=10
  GoTradie bunnings get 0123456 0987654
  GoTradie bunnings lookup 0123456
  GoTradie sync refresh --commit
  GoTradie sync import --commit 0123456
  GoTradie sync search "merbau decking" --limit=10
  GoTradie sync search "merbau decking" --create --select=0123456,0987654 --commit
  GoTradie ninja export products products.csv
  GoTradie ninja export products -
  GoTradie ninja export products products.csv --force
  GoTradie ninja import products products.csv
  GoTradie ninja import products --commit products.csv
  GoTradie ninja export clients clients.csv
  GoTradie ninja import clients --commit clients.csv
  GoTradie ninja import expenses purchases.csv --receipts-root receipts
  GoTradie ninja import expenses --commit purchases.csv --receipts-root receipts
  GoTradie ninja import expenses purchases.csv --batch-size 100 --pause --commit
  GoTradie ninja export quotes quotes.csv
  GoTradie ninja export invoices invoices.csv
  GoTradie ninja export payments payments.csv
  GoTradie ninja export bas
  GoTradie ninja export bas --fy 2025
`)
}
