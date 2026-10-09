package accounting

import (
	"fmt"
	"math/big"
	"sort"
	"strings"
)

type Severity string

const (
	SeverityInfo    Severity = "INFO"
	SeverityWarning Severity = "WARNING"
	SeverityError   Severity = "ERROR"
)

type Amounts struct {
	GrossCents int64
	NetCents   int64
	GSTCents   int64
}

type Exception struct {
	Severity   Severity
	SourceType string
	SourceID   string
	Date       string
	Message    string
}

type Allocation struct {
	SourceType        string
	SourceID          string
	Date              string
	AmountCents       int64
	CurrencyID        string
	ArchivedOrDeleted bool
}

type Sale struct {
	SourceID          string
	Number            string
	CustomerID        string
	CustomerName      string
	CurrencyID        string
	Date              string
	Amounts           Amounts
	TaxKnown          bool
	ArchivedOrDeleted bool
	Payments          []Allocation
}

type Purchase struct {
	SourceID            string
	Number              string
	VendorID            string
	VendorName          string
	CurrencyID          string
	Date                string
	PaymentDate         string
	Amounts             Amounts
	SettlementBaseCents int64
	BusinessUsePercent  *float64
	TaxKnown            bool
	SupplierAccount     bool
	ArchivedOrDeleted   bool
	Settlements         []Allocation
}

type Dataset struct {
	Source            string
	CompanyCurrencyID string
	Sales             []Sale
	Purchases         []Purchase
	Exceptions        []Exception
}

type GSTEvent struct {
	Kind               string
	SourceType         string
	SourceID           string
	RelatedSourceType  string
	RelatedSourceID    string
	Number             string
	PartyID            string
	PartyName          string
	Date               string
	Amounts            Amounts
	BusinessUsePercent *float64
}

func (d Dataset) GSTEvents(basis string) ([]GSTEvent, []Exception) {
	exceptions := append([]Exception(nil), d.Exceptions...)
	var events []GSTEvent

	for _, sale := range d.Sales {
		if sale.Amounts == (Amounts{}) {
			continue
		}
		if basis == "accrual" {
			event := GSTEvent{
				Kind: "sale", SourceType: "invoice", SourceID: sale.SourceID,
				Number: sale.Number, PartyID: sale.CustomerID, PartyName: sale.CustomerName,
				Date: sale.Date, Amounts: sale.Amounts,
			}
			events = append(events, event)
			exceptions = appendRecognitionExceptions(exceptions, d.CompanyCurrencyID, sale.CurrencyID, sale.TaxKnown, sale.ArchivedOrDeleted, event)
			continue
		}

		allocated, problems := allocateProportionally(sale.Amounts, sale.Amounts.GrossCents, sale.Payments)
		for _, allocation := range allocated {
			event := GSTEvent{
				Kind: "sale", SourceType: "invoice", SourceID: sale.SourceID,
				RelatedSourceType: allocation.allocation.SourceType, RelatedSourceID: allocation.allocation.SourceID,
				Number: sale.Number, PartyID: sale.CustomerID, PartyName: sale.CustomerName,
				Date: allocation.allocation.Date, Amounts: allocation.amounts,
			}
			events = append(events, event)
			exceptions = appendRecognitionExceptions(exceptions, d.CompanyCurrencyID, firstNonEmpty(allocation.allocation.CurrencyID, sale.CurrencyID), sale.TaxKnown, sale.ArchivedOrDeleted || allocation.allocation.ArchivedOrDeleted, event)
		}
		exceptions = appendAllocationProblems(exceptions, "invoice", sale.SourceID, sale.Date, problems)
	}

	for _, purchase := range d.Purchases {
		if purchase.Amounts == (Amounts{}) {
			continue
		}
		if basis == "accrual" {
			event := purchaseEvent(purchase, purchase.Date, Amounts{}, Allocation{})
			event.Amounts = purchase.Amounts
			events = append(events, event)
			exceptions = appendRecognitionExceptions(exceptions, d.CompanyCurrencyID, purchase.CurrencyID, purchase.TaxKnown, purchase.ArchivedOrDeleted, event)
			continue
		}

		if !purchase.SupplierAccount {
			if strings.TrimSpace(purchase.PaymentDate) == "" && purchase.Amounts.GrossCents != 0 {
				exceptions = append(exceptions, Exception{Severity: SeverityError, SourceType: "expense", SourceID: purchase.SourceID, Message: "missing payment date required for cash-basis GST calculation"})
				continue
			}
			event := purchaseEvent(purchase, purchase.PaymentDate, purchase.Amounts, Allocation{})
			events = append(events, event)
			exceptions = appendRecognitionExceptions(exceptions, d.CompanyCurrencyID, purchase.CurrencyID, purchase.TaxKnown, purchase.ArchivedOrDeleted, event)
			continue
		}

		if purchase.Amounts.GrossCents < 0 {
			exceptions = append(exceptions, Exception{Severity: SeverityError, SourceType: "expense", SourceID: purchase.SourceID, Date: purchase.Date, Message: "supplier-account credit has no deterministic cash GST recognition event"})
			continue
		}
		allocated, problems := allocateProportionally(purchase.Amounts, purchase.SettlementBaseCents, purchase.Settlements)
		for _, allocation := range allocated {
			event := purchaseEvent(purchase, allocation.allocation.Date, allocation.amounts, allocation.allocation)
			events = append(events, event)
			exceptions = appendRecognitionExceptions(exceptions, d.CompanyCurrencyID, firstNonEmpty(allocation.allocation.CurrencyID, purchase.CurrencyID), purchase.TaxKnown, purchase.ArchivedOrDeleted || allocation.allocation.ArchivedOrDeleted, event)
		}
		exceptions = appendAllocationProblems(exceptions, "expense", purchase.SourceID, purchase.Date, problems)
	}

	sort.SliceStable(events, func(i, j int) bool {
		if events[i].Date != events[j].Date {
			return events[i].Date < events[j].Date
		}
		if events[i].SourceType != events[j].SourceType {
			return events[i].SourceType < events[j].SourceType
		}
		if events[i].SourceID != events[j].SourceID {
			return events[i].SourceID < events[j].SourceID
		}
		return events[i].RelatedSourceID < events[j].RelatedSourceID
	})
	return events, exceptions
}

func purchaseEvent(purchase Purchase, date string, amounts Amounts, allocation Allocation) GSTEvent {
	return GSTEvent{
		Kind: "purchase", SourceType: "expense", SourceID: purchase.SourceID,
		RelatedSourceType: allocation.SourceType, RelatedSourceID: allocation.SourceID,
		Number: purchase.Number, PartyID: purchase.VendorID, PartyName: purchase.VendorName,
		Date: date, Amounts: amounts, BusinessUsePercent: purchase.BusinessUsePercent,
	}
}

func appendRecognitionExceptions(exceptions []Exception, companyCurrency, recordCurrency string, taxKnown, archived bool, event GSTEvent) []Exception {
	if strings.TrimSpace(event.Date) == "" {
		exceptions = append(exceptions, Exception{Severity: SeverityError, SourceType: event.SourceType, SourceID: event.SourceID, Message: "missing accounting date required for GST calculation"})
	}
	if !taxKnown {
		exceptions = append(exceptions, Exception{Severity: SeverityError, SourceType: event.SourceType, SourceID: event.SourceID, Date: event.Date, Message: "missing GST treatment"})
	}
	if companyCurrency != "" && recordCurrency != "" && companyCurrency != recordCurrency {
		exceptions = append(exceptions, Exception{Severity: SeverityError, SourceType: event.SourceType, SourceID: event.SourceID, Date: event.Date, Message: fmt.Sprintf("unsupported foreign currency %q; company currency is %q", recordCurrency, companyCurrency)})
	}
	if archived {
		exceptions = append(exceptions, Exception{Severity: SeverityError, SourceType: event.SourceType, SourceID: event.SourceID, Date: event.Date, Message: "archived or deleted accounting record affects the result"})
	}
	return exceptions
}

type proportionalAllocation struct {
	allocation Allocation
	amounts    Amounts
}

func allocateProportionally(total Amounts, denominator int64, allocations []Allocation) ([]proportionalAllocation, []string) {
	if len(allocations) == 0 {
		return nil, nil
	}
	if denominator <= 0 {
		return nil, []string{"cannot allocate payment proportionally because the source gross amount is not positive"}
	}
	rows := append([]Allocation(nil), allocations...)
	sort.SliceStable(rows, func(i, j int) bool {
		if rows[i].Date != rows[j].Date {
			return rows[i].Date < rows[j].Date
		}
		return rows[i].SourceID < rows[j].SourceID
	})
	var result []proportionalAllocation
	var problems []string
	var cumulative, previousGross, previousNet, previousGST int64
	for _, allocation := range rows {
		if allocation.AmountCents <= 0 {
			problems = append(problems, fmt.Sprintf("allocation %q has non-positive amount", allocation.SourceID))
			continue
		}
		cumulative += allocation.AmountCents
		if cumulative > denominator {
			problems = append(problems, fmt.Sprintf("allocations exceed source gross amount by %.2f", float64(cumulative-denominator)/100))
			cumulative = denominator
		}
		gross := proportionalCents(total.GrossCents, cumulative, denominator)
		net := proportionalCents(total.NetCents, cumulative, denominator)
		gst := proportionalCents(total.GSTCents, cumulative, denominator)
		part := Amounts{GrossCents: gross - previousGross, NetCents: net - previousNet, GSTCents: gst - previousGST}
		previousGross, previousNet, previousGST = gross, net, gst
		if part.GrossCents != 0 || part.NetCents != 0 || part.GSTCents != 0 {
			result = append(result, proportionalAllocation{allocation: allocation, amounts: part})
		}
		if cumulative == denominator {
			break
		}
	}
	return result, problems
}

func appendAllocationProblems(exceptions []Exception, sourceType, sourceID, date string, problems []string) []Exception {
	for _, problem := range problems {
		exceptions = append(exceptions, Exception{Severity: SeverityError, SourceType: sourceType, SourceID: sourceID, Date: date, Message: problem})
	}
	return exceptions
}

func proportionalCents(total, part, whole int64) int64 {
	if total == 0 || part == 0 || whole <= 0 {
		return 0
	}
	product := new(big.Int).Mul(big.NewInt(total), big.NewInt(part))
	quotient, remainder := new(big.Int), new(big.Int)
	quotient.QuoRem(product, big.NewInt(whole), remainder)
	doubleRemainder := new(big.Int).Lsh(new(big.Int).Abs(remainder), 1)
	if doubleRemainder.Cmp(big.NewInt(whole)) >= 0 {
		if product.Sign() < 0 {
			quotient.Sub(quotient, big.NewInt(1))
		} else {
			quotient.Add(quotient, big.NewInt(1))
		}
	}
	return quotient.Int64()
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return value
		}
	}
	return ""
}
