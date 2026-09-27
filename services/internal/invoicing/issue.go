package invoicing

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/google/uuid"
	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"

	"github.com/its-aryansingh/qrit/services/internal/jobs"
)

// IST is the timezone of GST dates (issue date, financial year).
var IST = func() *time.Location {
	l, err := time.LoadLocation("Asia/Kolkata")
	if err != nil {
		return time.FixedZone("IST", 5*3600+1800)
	}
	return l
}()

// Invoice is an issued invoice.
type Invoice struct {
	ID          uuid.UUID  `json:"id"`
	OrgID       uuid.UUID  `json:"org_id"`
	ContractID  *uuid.UUID `json:"contract_id"`
	Number      string     `json:"number"`
	IssueDate   time.Time  `json:"issue_date"`
	DueDate     time.Time  `json:"due_date"`
	PeriodStart *time.Time `json:"period_start"`
	PeriodEnd   *time.Time `json:"period_end"`
	Currency    string     `json:"currency"`
	Seller      Seller     `json:"seller"`
	Buyer       Buyer      `json:"buyer"`
	Lines       []Line     `json:"lines"`
	Totals
	Status         string     `json:"status"`
	PaymentLinkURL *string    `json:"payment_link_url"`
	PaidAt         *time.Time `json:"paid_at"`
	PaidReference  *string    `json:"paid_reference,omitempty"`
	VoidReason     *string    `json:"void_reason,omitempty"`
	RemindersSent  int        `json:"reminders_sent"`
	CreatedAt      time.Time  `json:"created_at"`
	paymentLinkID  *string
}

const invoiceCols = `id, org_id, contract_id, number, issue_date, due_date, period_start, period_end, currency, seller, buyer,
	lines, tax_mode, place_of_supply, subtotal_minor, cgst_minor, sgst_minor, igst_minor, total_minor, endorsement, status,
	payment_link_url, payment_link_id, paid_at, paid_reference, void_reason, reminders_sent, created_at`

// Querier is satisfied by pgxpool.Pool and pgx.Tx.
type Querier interface {
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
}

func scanInvoice(row pgx.Row) (Invoice, error) {
	var inv Invoice
	var seller, buyer, lines []byte
	err := row.Scan(&inv.ID, &inv.OrgID, &inv.ContractID, &inv.Number, &inv.IssueDate, &inv.DueDate, &inv.PeriodStart, &inv.PeriodEnd,
		&inv.Currency, &seller, &buyer, &lines, &inv.TaxMode, &inv.PlaceOfSupply, &inv.SubtotalMinor, &inv.CGSTMinor, &inv.SGSTMinor,
		&inv.IGSTMinor, &inv.TotalMinor, &inv.Endorsement, &inv.Status, &inv.PaymentLinkURL, &inv.paymentLinkID, &inv.PaidAt,
		&inv.PaidReference, &inv.VoidReason, &inv.RemindersSent, &inv.CreatedAt)
	_ = json.Unmarshal(seller, &inv.Seller)
	_ = json.Unmarshal(buyer, &inv.Buyer)
	_ = json.Unmarshal(lines, &inv.Lines)
	return inv, err
}

// Get loads one invoice (scoped to an organisation when orgID is set).
func Get(ctx context.Context, db Querier, id uuid.UUID, orgID *uuid.UUID) (Invoice, error) {
	sql := `SELECT ` + invoiceCols + ` FROM invoices WHERE id = $1`
	args := []any{id}
	if orgID != nil {
		sql += ` AND org_id = $2`
		args = append(args, *orgID)
	}
	return scanInvoice(db.QueryRow(ctx, sql, args...))
}

// List returns an organisation's invoices, newest first.
func List(ctx context.Context, db Querier, orgID uuid.UUID) ([]Invoice, error) {
	rows, err := db.Query(ctx, `SELECT `+invoiceCols+` FROM invoices WHERE org_id = $1 ORDER BY issue_date DESC, number DESC LIMIT 500`, orgID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Invoice{}
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			return nil, err
		}
		out = append(out, inv)
	}
	return out, rows.Err()
}

// ErrAlreadyIssued means the contract period already has an invoice.
var ErrAlreadyIssued = errors.New("invoicing: period already invoiced")

// Issuer creates invoices for contract periods.
type Issuer struct {
	Pool   *pgxpool.Pool
	Seller Seller
	Now    func() time.Time
}

func (i *Issuer) today() time.Time {
	now := time.Now()
	if i.Now != nil {
		now = i.Now()
	}
	y, m, d := now.In(IST).Date()
	return time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
}

// Actor attributes the issuance in the audit chain.
type Actor struct {
	Type string // system | staff
	ID   *uuid.UUID
}

type contractRow struct {
	id, orgID        uuid.UUID
	name, interval   string
	currency         string
	startsOn, endsOn time.Time
	amount           int64
	seats, termsDays int
	po               *string
	status           string
}

func buyerOf(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, po *string) (Buyer, error) {
	var (
		name, country       string
		legal, gstin, email *string
		addr                []byte
	)
	if err := tx.QueryRow(ctx, `SELECT name, legal_name, gstin, billing_email::text, tax_country, billing_address FROM organizations WHERE id = $1`,
		orgID).Scan(&name, &legal, &gstin, &email, &country, &addr); err != nil {
		return Buyer{}, err
	}
	b := Buyer{LegalName: name, Country: strings.TrimSpace(country)}
	if legal != nil && *legal != "" {
		b.LegalName = *legal
	}
	if gstin != nil {
		b.GSTIN = *gstin
	}
	if email != nil {
		b.Email = *email
	}
	if po != nil {
		b.PONumber = *po
	}
	var a map[string]any
	_ = json.Unmarshal(addr, &a)
	var parts []string
	for _, k := range []string{"line1", "line2", "city", "postal_code"} {
		if v, _ := a[k].(string); strings.TrimSpace(v) != "" {
			parts = append(parts, strings.TrimSpace(v))
		}
	}
	if sc, _ := a["state_code"].(string); sc != "" {
		b.StateCode = sc
		if n, ok := States[sc]; ok {
			parts = append(parts, n)
		}
	}
	if b.Country != "" && b.Country != "IN" {
		parts = append(parts, b.Country)
	}
	b.Address = strings.Join(parts, ", ")
	return b, nil
}

// IssueForPeriod issues the invoice for one contract period: base fee (prorated for a short
// final period) plus a seat true-up line when active members exceed contracted seats.
func (i *Issuer) IssueForPeriod(ctx context.Context, contractID uuid.UUID, p Period, actor Actor) (Invoice, error) {
	tx, err := i.Pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var c contractRow
	err = tx.QueryRow(ctx, `SELECT id, org_id, name, billing_interval, currency, starts_on, ends_on, amount_minor, seats,
		payment_terms_days, po_number, status FROM contracts WHERE id = $1 FOR UPDATE`, contractID).Scan(&c.id, &c.orgID, &c.name,
		&c.interval, &c.currency, &c.startsOn, &c.endsOn, &c.amount, &c.seats, &c.termsDays, &c.po, &c.status)
	if err != nil {
		return Invoice{}, err
	}
	if c.status != "active" {
		return Invoice{}, fmt.Errorf("invoicing: contract is %s", c.status)
	}
	var exists bool
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM invoices WHERE contract_id = $1 AND period_start = $2 AND status <> 'void')`,
		c.id, p.Start).Scan(&exists); err != nil {
		return Invoice{}, err
	}
	if exists {
		return Invoice{}, ErrAlreadyIssued
	}
	buyer, err := buyerOf(ctx, tx, c.orgID, c.po)
	if err != nil {
		return Invoice{}, err
	}
	period := p.Start.Format("2 Jan 2006") + " – " + p.End.Format("2 Jan 2006")
	base := Prorate(c.amount, p, c.interval)
	lines := []Line{{Description: c.name + " (" + period + ")", SAC: i.Seller.SAC, Qty: 1, UnitMinor: base, AmountMinor: base}}
	var active int
	if err := tx.QueryRow(ctx, `SELECT count(*) FROM org_members WHERE org_id = $1 AND status = 'active'`, c.orgID).Scan(&active); err != nil {
		return Invoice{}, err
	}
	if over := active - c.seats; over > 0 && c.seats > 0 {
		unit := Prorate(c.amount/int64(c.seats), p, c.interval)
		lines = append(lines, Line{Description: fmt.Sprintf("Additional seats: %d active members above %d contracted (%s)", over, c.seats, period),
			SAC: i.Seller.SAC, Qty: int64(over), UnitMinor: unit, AmountMinor: unit * int64(over)})
	}
	totals, err := Compute(i.Seller, buyer, c.currency, lines)
	if err != nil {
		return Invoice{}, err
	}
	issue := i.today()
	due := issue.AddDate(0, 0, c.termsDays)
	var number string
	if err := tx.QueryRow(ctx, `SELECT next_invoice_number($1)`, issue).Scan(&number); err != nil {
		return Invoice{}, err
	}
	sb, _ := json.Marshal(i.Seller)
	bb, _ := json.Marshal(buyer)
	lb, _ := json.Marshal(lines)
	id := uuid.Must(uuid.NewV7())
	inv, err := scanInvoice(tx.QueryRow(ctx, `INSERT INTO invoices (id, org_id, contract_id, number, issue_date, due_date, period_start,
		period_end, currency, tax_mode, place_of_supply, seller, buyer, lines, subtotal_minor, cgst_minor, sgst_minor, igst_minor,
		total_minor, endorsement) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20)
		RETURNING `+invoiceCols, id, c.orgID, c.id, number, issue, due, p.Start, p.End, c.currency, totals.TaxMode, totals.PlaceOfSupply,
		sb, bb, lb, totals.SubtotalMinor, totals.CGSTMinor, totals.SGSTMinor, totals.IGSTMinor, totals.TotalMinor, totals.Endorsement))
	if err != nil {
		return Invoice{}, err
	}
	actorType := actor.Type
	if actorType == "" {
		actorType = "system"
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (org_id, actor_type, actor_id, action, target_type, target_id, changes)
		VALUES ($1, $2, $3, 'invoice.issued', 'invoice', $4, jsonb_build_object('number', $5::text, 'total_minor', $6::bigint,
		'currency', $7::text, 'tax_mode', $8::text))`, c.orgID, actorType, actor.ID, id, number, totals.TotalMinor, c.currency, totals.TaxMode); err != nil {
		return Invoice{}, err
	}
	// Payment link and email go out after commit (the job is durable).
	if _, err := jobs.Enqueue(ctx, tx, "invoice.deliver", map[string]any{"invoice_id": id}, jobs.Options{UniqueKey: "invoice:" + id.String()}); err != nil {
		return Invoice{}, err
	}
	return inv, tx.Commit(ctx)
}

// IssueDue is the invoice run: every started, un-invoiced period of every active contract.
// Contracts past their end date are marked expired.
func (i *Issuer) IssueDue(ctx context.Context) (int, error) {
	today := i.today()
	if _, err := i.Pool.Exec(ctx, `UPDATE contracts SET status = 'expired' WHERE status = 'active' AND ends_on <= $1`, today); err != nil {
		return 0, err
	}
	rows, err := i.Pool.Query(ctx, `SELECT id, starts_on, ends_on, billing_interval FROM contracts WHERE status = 'active'`)
	if err != nil {
		return 0, err
	}
	type ct struct {
		id           uuid.UUID
		starts, ends time.Time
		interval     string
	}
	var cs []ct
	for rows.Next() {
		var c ct
		if rows.Scan(&c.id, &c.starts, &c.ends, &c.interval) == nil {
			cs = append(cs, c)
		}
	}
	rows.Close()
	n := 0
	var firstErr error
	for _, c := range cs {
		for _, p := range Periods(c.starts, c.ends, today, c.interval) {
			_, err := i.IssueForPeriod(ctx, c.id, p, Actor{Type: "system"})
			switch {
			case err == nil:
				n++
			case errors.Is(err, ErrAlreadyIssued):
			default:
				if firstErr == nil {
					firstErr = fmt.Errorf("contract %s: %w", c.id, err)
				}
			}
		}
	}
	return n, firstErr
}

// MarkPaid records payment and lifts a billing hold once nothing is overdue.
func MarkPaid(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, reference string, actor Actor) (Invoice, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	inv, err := scanInvoice(tx.QueryRow(ctx, `UPDATE invoices SET status = 'paid', paid_at = now(), paid_reference = NULLIF($2, '')
		WHERE id = $1 AND status = 'issued' RETURNING `+invoiceCols, id, reference))
	if errors.Is(err, pgx.ErrNoRows) {
		cur, gerr := Get(ctx, tx, id, nil)
		if gerr == nil && cur.Status == "paid" {
			return cur, nil // idempotent (webhook retries)
		}
		return Invoice{}, fmt.Errorf("invoicing: invoice is not payable")
	}
	if err != nil {
		return Invoice{}, err
	}
	actorType := actor.Type
	if actorType == "" {
		actorType = "system"
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (org_id, actor_type, actor_id, action, target_type, target_id, changes)
		VALUES ($1, $2, $3, 'invoice.paid', 'invoice', $4, jsonb_build_object('number', $5::text, 'reference', $6::text))`,
		inv.OrgID, actorType, actor.ID, inv.ID, inv.Number, reference); err != nil {
		return Invoice{}, err
	}
	if err := updateHold(ctx, tx, inv.OrgID, time.Now().In(IST)); err != nil {
		return Invoice{}, err
	}
	return inv, tx.Commit(ctx)
}

// Void cancels an unpaid invoice (a credit note is the GST-correct way to reverse a paid one).
func Void(ctx context.Context, pool *pgxpool.Pool, id uuid.UUID, reason string, actor Actor) (Invoice, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return Invoice{}, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	inv, err := scanInvoice(tx.QueryRow(ctx, `UPDATE invoices SET status = 'void', void_reason = $2 WHERE id = $1 AND status = 'issued'
		RETURNING `+invoiceCols, id, reason))
	if errors.Is(err, pgx.ErrNoRows) {
		return Invoice{}, fmt.Errorf("invoicing: only unpaid invoices can be voided; issue a credit note for paid ones")
	}
	if err != nil {
		return Invoice{}, err
	}
	if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (org_id, actor_type, actor_id, action, target_type, target_id, changes)
		VALUES ($1, $2, $3, 'invoice.voided', 'invoice', $4, jsonb_build_object('number', $5::text, 'reason', $6::text))`,
		inv.OrgID, actor.Type, actor.ID, inv.ID, inv.Number, reason); err != nil {
		return Invoice{}, err
	}
	if err := updateHold(ctx, tx, inv.OrgID, time.Now().In(IST)); err != nil {
		return Invoice{}, err
	}
	return inv, tx.Commit(ctx)
}

// HoldAfterDays is how long past due an unpaid invoice may be before dashboard edits freeze.
const HoldAfterDays = 30

// updateHold sets or clears organizations.billing_hold_since from the org's overdue invoices.
func updateHold(ctx context.Context, tx pgx.Tx, orgID uuid.UUID, now time.Time) error {
	var overdue bool
	y, m, d := now.Date()
	cutoff := time.Date(y, m, d, 0, 0, 0, 0, time.UTC).AddDate(0, 0, -HoldAfterDays)
	if err := tx.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM invoices WHERE org_id = $1 AND status = 'issued' AND due_date < $2)`,
		orgID, cutoff).Scan(&overdue); err != nil {
		return err
	}
	var action string
	if overdue {
		tag, err := tx.Exec(ctx, `UPDATE organizations SET billing_hold_since = now() WHERE id = $1 AND billing_hold_since IS NULL`, orgID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 1 {
			action = "billing.hold_started"
		}
	} else {
		tag, err := tx.Exec(ctx, `UPDATE organizations SET billing_hold_since = NULL WHERE id = $1 AND billing_hold_since IS NOT NULL`, orgID)
		if err != nil {
			return err
		}
		if tag.RowsAffected() == 1 {
			action = "billing.hold_cleared"
		}
	}
	if action != "" {
		if _, err := tx.Exec(ctx, `INSERT INTO audit_logs (org_id, actor_type, action, target_type, target_id)
			VALUES ($1, 'system', $2, 'organization', $1)`, orgID, action); err != nil {
			return err
		}
	}
	return nil
}

// ReminderOffsets are the dunning reminders relative to the due date (days).
var ReminderOffsets = []int{-7, 0, 7, 14}

// Reminder is one dunning email to send.
type Reminder struct {
	Invoice Invoice
	To      string
	Offset  int
}

// Dunning returns reminders now due (marking them sent) and updates billing holds.
func Dunning(ctx context.Context, pool *pgxpool.Pool, now time.Time) ([]Reminder, error) {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	y, m, d := now.In(IST).Date()
	today := time.Date(y, m, d, 0, 0, 0, 0, time.UTC)
	rows, err := tx.Query(ctx, `SELECT `+invoiceCols+` FROM invoices WHERE status = 'issued' AND reminders_sent < $1
		ORDER BY due_date FOR UPDATE SKIP LOCKED`, len(ReminderOffsets))
	if err != nil {
		return nil, err
	}
	var due []Invoice
	for rows.Next() {
		inv, err := scanInvoice(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		if !today.Before(inv.DueDate.AddDate(0, 0, ReminderOffsets[inv.RemindersSent])) {
			due = append(due, inv)
		}
	}
	rows.Close()
	var out []Reminder
	orgs := map[uuid.UUID]bool{}
	for _, inv := range due {
		// Skip reminders whose moment has passed (e.g. an invoice issued after its first slot).
		k := inv.RemindersSent
		for k+1 < len(ReminderOffsets) && !today.Before(inv.DueDate.AddDate(0, 0, ReminderOffsets[k+1])) {
			k++
		}
		if _, err := tx.Exec(ctx, `UPDATE invoices SET reminders_sent = $2, last_reminder_at = now() WHERE id = $1`, inv.ID, k+1); err != nil {
			return nil, err
		}
		if inv.Buyer.Email != "" {
			out = append(out, Reminder{Invoice: inv, To: inv.Buyer.Email, Offset: ReminderOffsets[k]})
		}
		orgs[inv.OrgID] = true
	}
	hr, err := tx.Query(ctx, `SELECT DISTINCT org_id FROM invoices WHERE status = 'issued' AND due_date < $1
		UNION SELECT id FROM organizations WHERE billing_hold_since IS NOT NULL`, today.AddDate(0, 0, -HoldAfterDays))
	if err != nil {
		return nil, err
	}
	for hr.Next() {
		var id uuid.UUID
		if hr.Scan(&id) == nil {
			orgs[id] = true
		}
	}
	hr.Close()
	for id := range orgs {
		if err := updateHold(ctx, tx, id, now.In(IST)); err != nil {
			return nil, err
		}
	}
	return out, tx.Commit(ctx)
}

// PaymentLinker creates an online payment link for an invoice (Razorpay for INR).
type PaymentLinker interface {
	Configured() bool
	CreateLink(ctx context.Context, inv Invoice) (id, url string, err error)
}

// AttachPaymentLink stores a payment link on an unpaid invoice (once).
func AttachPaymentLink(ctx context.Context, pool *pgxpool.Pool, inv Invoice, l PaymentLinker) (Invoice, error) {
	if l == nil || !l.Configured() || inv.Currency != "INR" || inv.Status != "issued" || inv.paymentLinkID != nil {
		return inv, nil
	}
	id, url, err := l.CreateLink(ctx, inv)
	if err != nil {
		return inv, err
	}
	return scanInvoice(pool.QueryRow(ctx, `UPDATE invoices SET payment_link_id = $2, payment_link_url = $3 WHERE id = $1
		RETURNING `+invoiceCols, inv.ID, id, url))
}
