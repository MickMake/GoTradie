package financial

import (
	invoiceninja "github.com/MickMake/GoInvoiceNinja"
	"github.com/MickMake/GoTradie/internal/accounting"
)

type SourceData struct {
	Source               string
	Invoices             []invoiceninja.Invoice
	Payments             []invoiceninja.Payment
	Expenses             []invoiceninja.Expense
	SupplierTransactions []invoiceninja.BankTransaction
	Customers            []invoiceninja.ClientEntity
	Vendors              []invoiceninja.Vendor
	Products             []invoiceninja.Product
	Projects             []invoiceninja.Project
	Accounting           accounting.Dataset
}
