package ninja

import (
	"context"
	"fmt"

	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/financial"
)

func (s *Service) LoadFinancialSource(ctx context.Context) (financial.SourceData, error) {
	source, err := s.loadAccountingSource(ctx)
	if err != nil {
		return financial.SourceData{}, err
	}
	customers, err := s.client.Clients.ListAll(ctx, invoiceninja.ClientQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Include: []string{"contacts"}, Status: "active,archived,deleted"},
		WithTrashed: true,
	})
	if err != nil {
		return financial.SourceData{}, fmt.Errorf("list Invoice Ninja customers: %w", err)
	}
	vendors, err := s.client.Vendors.ListAll(ctx, invoiceninja.VendorQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Status: "active,archived,deleted"},
		WithTrashed: true,
	})
	if err != nil {
		return financial.SourceData{}, fmt.Errorf("list Invoice Ninja vendors: %w", err)
	}
	products, err := s.client.Products.ListAll(ctx, invoiceninja.ProductQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Status: "active,archived,deleted"},
		WithTrashed: true,
	})
	if err != nil {
		return financial.SourceData{}, fmt.Errorf("list Invoice Ninja products: %w", err)
	}
	projects, err := s.client.Projects.ListAll(ctx, invoiceninja.ProjectQuery{
		ListOptions: invoiceninja.ListOptions{PerPage: 100, Status: "active,archived,deleted"},
		WithTrashed: true,
	})
	if err != nil {
		return financial.SourceData{}, fmt.Errorf("list Invoice Ninja projects: %w", err)
	}
	return financial.SourceData{
		Source:               "Invoice Ninja",
		Invoices:             source.invoices,
		Payments:             source.payments,
		Expenses:             source.expenses,
		SupplierTransactions: source.transactions,
		Customers:            customers,
		Vendors:              vendors,
		Products:             products,
		Projects:             projects,
		Accounting:           buildAccountingDataset(source),
	}, nil
}
