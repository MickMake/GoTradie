package goinvoiceninja

import (
	"context"
	"net/url"
)

// BankIntegration is an Invoice Ninja bank-account container.
type BankIntegration struct {
	Entity
	ProviderName      string  `json:"provider_name,omitempty"`
	BankAccountName   string  `json:"bank_account_name,omitempty"`
	BankAccountNumber string  `json:"bank_account_number,omitempty"`
	BankAccountStatus string  `json:"bank_account_status,omitempty"`
	BankAccountType   string  `json:"bank_account_type,omitempty"`
	Currency          string  `json:"currency,omitempty"`
	IntegrationType   string  `json:"integration_type,omitempty"`
	AutoSync          bool    `json:"auto_sync,omitempty"`
	DisabledUpstream  bool    `json:"disabled_upstream,omitempty"`
	Balance           float64 `json:"balance,omitempty"`
}

// BankTransaction is an Invoice Ninja bank withdrawal or deposit.
type BankTransaction struct {
	Entity
	BankIntegrationID string  `json:"bank_integration_id,omitempty"`
	TransactionID     int64   `json:"transaction_id,omitempty"`
	Amount            float64 `json:"amount,omitempty"`
	CurrencyID        string  `json:"currency_id,omitempty"`
	Date              string  `json:"date,omitempty"`
	Description       string  `json:"description,omitempty"`
	Participant       string  `json:"participant,omitempty"`
	ParticipantName   string  `json:"participant_name,omitempty"`
	BaseType          string  `json:"base_type,omitempty"`
	ExpenseID         string  `json:"expense_id,omitempty"`
	VendorID          string  `json:"vendor_id,omitempty"`
	StatusID          string  `json:"status_id,omitempty"`
}

// BankTransactionRule contains the fields needed to prevent Invoice Ninja
// from auto-converting an imported supplier withdrawal into another Expense.
type BankTransactionRule struct {
	Entity
	Name         string `json:"name,omitempty"`
	AutoConvert  bool   `json:"auto_convert,omitempty"`
	MatchesOnAll bool   `json:"matches_on_all,omitempty"`
	AppliesTo    string `json:"applies_to,omitempty"`
}

type CreateBankTransactionRequest struct {
	BankIntegrationID string  `json:"bank_integration_id"`
	CurrencyID        string  `json:"currency_id"`
	Amount            float64 `json:"amount"`
	BaseType          string  `json:"base_type"`
	Date              string  `json:"date,omitempty"`
	Description       string  `json:"description,omitempty"`
	Participant       string  `json:"participant,omitempty"`
	ParticipantName   string  `json:"participant_name,omitempty"`
}

type UpdateBankTransactionRequest CreateBankTransactionRequest

type BankIntegrationService struct{ *Service[BankIntegration] }
type BankTransactionService struct{ *Service[BankTransaction] }
type BankTransactionRuleService struct{ *Service[BankTransactionRule] }

type BankIntegrationQuery struct {
	ListOptions
	WithTrashed bool
	IsDeleted   *bool
}

func (q BankIntegrationQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	return v
}

func (s *BankIntegrationService) ListAll(ctx context.Context, q BankIntegrationQuery) ([]BankIntegration, error) {
	return listAllWithValues[BankIntegration](ctx, s.client, s.path, q.Values())
}

type BankTransactionQuery struct {
	ListOptions
	WithTrashed       bool
	IsDeleted         *bool
	BankIntegrationID string
}

func (q BankTransactionQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	setIf(v, "bank_integration_ids", q.BankIntegrationID)
	return v
}

func (s *BankTransactionService) ListAll(ctx context.Context, q BankTransactionQuery) ([]BankTransaction, error) {
	return listAllWithValues[BankTransaction](ctx, s.client, s.path, q.Values())
}

func (s *BankTransactionService) Create(ctx context.Context, req CreateBankTransactionRequest) (*BankTransaction, error) {
	return s.Service.Create(ctx, req)
}

func (s *BankTransactionService) Update(ctx context.Context, id string, req UpdateBankTransactionRequest) (*BankTransaction, error) {
	return s.Service.Update(ctx, id, req)
}

type BankTransactionRuleQuery struct {
	ListOptions
	WithTrashed bool
	IsDeleted   *bool
}

func (q BankTransactionRuleQuery) Values() url.Values {
	v := q.ListOptions.Values()
	setBoolIf(v, "with_trashed", q.WithTrashed)
	setBoolPtr(v, "is_deleted", q.IsDeleted)
	return v
}

func (s *BankTransactionRuleService) ListAll(ctx context.Context, q BankTransactionRuleQuery) ([]BankTransactionRule, error) {
	return listAllWithValues[BankTransactionRule](ctx, s.client, s.path, q.Values())
}
