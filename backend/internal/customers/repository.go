package customers

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
)

var sortColumns = map[string]string{
	"id":               "a.id",
	"email":            "a.email",
	"balance":          "a.balance",
	"totalReplenished": "a.total_replenished",
	"score":            "r.risk_score",
	"potentialLoss":    "r.loss_amount_cents",
}

const customerColumns = `a.id, a.email, a.balance, a.withdraw_balance, a.bonus_balance, a.bonus_state, a.total_replenished,
	r.services_count, r.assessment_status, r.risk_level, r.risk_score, r.reasons,
	r.loss_amount_cents, r.renewal_revenue_cents, r.unassessed_revenue_cents, r.is_partial, r.loss_explanation`

type Repository struct {
	pool *pgxpool.Pool
}

func NewRepository(pool *pgxpool.Pool) *Repository {
	return &Repository{pool: pool}
}

func (r *Repository) Published(ctx context.Context) (entity.Calculation, int64, error) {
	var calculation entity.Calculation
	var horizonDays int64
	err := r.pool.QueryRow(ctx, `
		SELECT coalesce(c.version, 0), coalesce(c.source_version, 0), p.active_model_version, c.calculated_at, m.horizon_days
		FROM published_state p
		JOIN model_versions m ON m.version = p.active_model_version
		LEFT JOIN calculation_versions c ON c.version = p.active_calculation_version`,
	).Scan(&calculation.Version, &calculation.SourceVersion, &calculation.ModelVersion, &calculation.CalculatedAt, &horizonDays)
	if err != nil {
		return entity.Calculation{}, 0, fmt.Errorf("read published calculation: %w", err)
	}
	if calculation.CalculatedAt != nil {
		*calculation.CalculatedAt = calculation.CalculatedAt.UTC()
	}
	return calculation, horizonDays, nil
}

func (r *Repository) List(ctx context.Context, calculation entity.Calculation, query Query) ([]entity.Customer, int64, error) {
	where, args := customerFilter(calculation, query)
	var total int64
	if err := r.pool.QueryRow(ctx, `SELECT count(*) FROM customer_results r JOIN accounts a ON a.snapshot_version = r.source_version AND a.id = r.customer_id WHERE `+where, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count customers: %w", err)
	}
	direction := "ASC"
	if query.SortOrder == "desc" {
		direction = "DESC"
	}
	args = append(args, query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.pool.Query(ctx, fmt.Sprintf(`
		SELECT %s
		FROM customer_results r
		JOIN accounts a ON a.snapshot_version = r.source_version AND a.id = r.customer_id
		WHERE %s
		ORDER BY %s %s NULLS LAST, a.id
		LIMIT $%d OFFSET $%d`, customerColumns, where, sortColumns[query.SortBy], direction, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list customers: %w", err)
	}
	customers, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (entity.Customer, error) {
		var customer entity.Customer
		return customer, scanCustomer(row, &customer)
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list customers: %w", err)
	}
	return customers, total, nil
}

func (r *Repository) Get(ctx context.Context, calculation entity.Calculation, id entity.ID) (entity.CustomerDetail, error) {
	var detail entity.CustomerDetail
	err := scanCustomer(r.pool.QueryRow(ctx, `
		SELECT `+customerColumns+`, r.features
		FROM customer_results r
		JOIN accounts a ON a.snapshot_version = r.source_version AND a.id = r.customer_id
		WHERE r.calculation_version = $1 AND r.customer_id = $2`, calculation.Version, id), &detail.Customer, &detail.Features)
	if errors.Is(err, pgx.ErrNoRows) {
		return entity.CustomerDetail{}, entity.ErrNotFound
	}
	if err != nil {
		return entity.CustomerDetail{}, fmt.Errorf("read customer: %w", err)
	}
	rows, err := r.pool.Query(ctx, `
		SELECT s.id, s.owner_id, s.name, s.payment_term, s.auto_prolong, s.created_at, s.expires_at, s.status, s.price,
			coalesce(sr.assessment_status, 'insufficient_data'), coalesce(sr.risk_level, 'unknown'), sr.risk_score,
			coalesce(sr.reasons, '[]'), coalesce(sr.features, '[]'), sr.loss_amount_cents,
			coalesce(sr.renewal_revenue_cents, 0), coalesce(sr.unassessed_revenue_cents, 0),
			coalesce(sr.is_partial, false), coalesce(sr.loss_explanation, '')
		FROM services s
		LEFT JOIN service_results sr ON sr.calculation_version = $1 AND sr.service_id = s.id
		WHERE s.snapshot_version = $2 AND s.owner_id = $3
		ORDER BY s.id`, calculation.Version, calculation.SourceVersion, id)
	if err != nil {
		return entity.CustomerDetail{}, fmt.Errorf("read customer services: %w", err)
	}
	detail.Services, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (entity.ServiceAssessment, error) {
		var s entity.ServiceAssessment
		err := row.Scan(&s.ID, &s.OwnerID, &s.Name, &s.PaymentTerm, &s.AutoProlong, &s.CreatedAt.Time, &s.ExpiresAt.Time, &s.Status, &s.Price,
			&s.Risk.Status, &s.Risk.Level, &s.Risk.Score, &s.Risk.Reasons, &s.Features, &s.Loss.AmountCents,
			&s.Loss.RenewalRevenueCents, &s.Loss.UnassessedRevenueCents, &s.Loss.IsPartial, &s.Loss.Explanation)
		return s, err
	})
	if err != nil {
		return entity.CustomerDetail{}, fmt.Errorf("read customer services: %w", err)
	}
	return detail, nil
}

func (r *Repository) Transactions(ctx context.Context, calculation entity.Calculation, id entity.ID, query TransactionQuery) ([]entity.Transaction, int64, error) {
	var exists bool
	if err := r.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM accounts WHERE snapshot_version = $1 AND id = $2)`, calculation.SourceVersion, id).Scan(&exists); err != nil {
		return nil, 0, fmt.Errorf("check customer: %w", err)
	}
	if !exists {
		return nil, 0, entity.ErrNotFound
	}
	where := "t.snapshot_version = $1 AND s.owner_id = $2"
	args := []any{calculation.SourceVersion, id}
	if query.Status != "" {
		args = append(args, query.Status)
		where += " AND t.status = $" + strconv.Itoa(len(args))
	}
	if query.Type != "" {
		args = append(args, query.Type)
		where += " AND t.type = $" + strconv.Itoa(len(args))
	}
	from := " FROM transactions t JOIN services s ON s.snapshot_version = t.snapshot_version AND s.id = t.service_id WHERE " + where
	var total int64
	if err := r.pool.QueryRow(ctx, "SELECT count(*)"+from, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count transactions: %w", err)
	}
	direction := "DESC"
	if query.SortOrder == "asc" {
		direction = "ASC"
	}
	args = append(args, query.PageSize, (query.Page-1)*query.PageSize)
	rows, err := r.pool.Query(ctx, fmt.Sprintf("SELECT t.id, t.service_id, t.amount, t.status, t.type, t.created_at, t.performed_at%s ORDER BY t.created_at %s, t.id LIMIT $%d OFFSET $%d",
		from, direction, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list transactions: %w", err)
	}
	transactions, err := pgx.CollectRows(rows, func(row pgx.CollectableRow) (entity.Transaction, error) {
		var t entity.Transaction
		var performedAt *time.Time
		err := row.Scan(&t.ID, &t.ServiceID, &t.Amount, &t.Status, &t.Type, &t.CreatedAt.Time, &performedAt)
		if performedAt != nil {
			t.PerformedAt = &entity.SourceTimestamp{Time: *performedAt}
		}
		return t, err
	})
	if err != nil {
		return nil, 0, fmt.Errorf("list transactions: %w", err)
	}
	return transactions, total, nil
}

func customerFilter(calculation entity.Calculation, query Query) (string, []any) {
	conditions := []string{"r.calculation_version = $1"}
	args := []any{calculation.Version}
	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, strings.ReplaceAll(condition, "?", "$"+strconv.Itoa(len(args))))
	}
	if query.Search != "" {
		pattern := "%" + strings.NewReplacer(`\`, `\\`, "%", `\%`, "_", `\_`).Replace(query.Search) + "%"
		if id, err := strconv.ParseInt(query.Search, 10, 64); err == nil {
			args = append(args, pattern, id)
			conditions = append(conditions, fmt.Sprintf("(a.email ILIKE $%d OR a.id = $%d)", len(args)-1, len(args)))
		} else {
			add("a.email ILIKE ?", pattern)
		}
	}
	if len(query.RiskLevels) > 0 {
		add("r.risk_level = ANY(?)", query.RiskLevels)
	}
	if query.MinScore != nil {
		add("r.risk_score >= ?", *query.MinScore)
	}
	if query.MaxScore != nil {
		add("r.risk_score <= ?", *query.MaxScore)
	}
	return strings.Join(conditions, " AND "), args
}

func scanCustomer(row pgx.Row, c *entity.Customer, extra ...any) error {
	return row.Scan(append([]any{&c.ID, &c.Email, &c.Balance, &c.WithdrawBalance, &c.BonusBalance, &c.BonusState, &c.TotalReplenished,
		&c.ServicesCount, &c.Risk.Status, &c.Risk.Level, &c.Risk.Score, &c.Risk.Reasons,
		&c.Loss.AmountCents, &c.Loss.RenewalRevenueCents, &c.Loss.UnassessedRevenueCents, &c.Loss.IsPartial, &c.Loss.Explanation}, extra...)...)
}
