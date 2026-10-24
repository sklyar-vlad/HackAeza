package calculation

import (
	"context"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/sklyar-vlad/HackAeza/backend/internal/entity"
	"github.com/sklyar-vlad/HackAeza/backend/internal/risk"
)

func (r *PostgresRepository) Customers(ctx context.Context, run Run, afterID entity.ID, limit int) ([]risk.Input, error) {
	if limit < 1 || limit > 5000 || r.maxBatchRecords < limit {
		return nil, ErrBatchRecordLimit
	}
	inputs := make([]risk.Input, 0, limit)
	err := pgx.BeginTxFunc(ctx, r.pool, pgx.TxOptions{IsoLevel: pgx.RepeatableRead, AccessMode: pgx.ReadOnly}, func(tx pgx.Tx) error {
		rows, err := tx.Query(ctx, `SELECT id, email, balance, withdraw_balance, bonus_balance, bonus_state, total_replenished
			FROM accounts WHERE snapshot_version = $1 AND id > $2 ORDER BY id LIMIT $3`, run.Target.SourceVersion, afterID, limit)
		if err != nil {
			return err
		}
		inputs, err = pgx.CollectRows(rows, func(row pgx.CollectableRow) (risk.Input, error) {
			input := risk.Input{Services: []entity.Service{}, Transactions: []entity.Transaction{}}
			a := &input.Account
			err := row.Scan(&a.ID, &a.Email, &a.Balance, &a.WithdrawBalance, &a.BonusBalance, &a.BonusState, &a.TotalReplenished)
			return input, err
		})
		if err != nil || len(inputs) == 0 {
			return err
		}
		ids := make([]int64, len(inputs))
		owners := make(map[entity.ID]int, len(inputs))
		for i, input := range inputs {
			ids[i], owners[input.Account.ID] = int64(input.Account.ID), i
		}
		records := len(inputs)
		rows, err = tx.Query(ctx, `SELECT id, owner_id, name, payment_term, auto_prolong, created_at, expires_at, status, price
			FROM services WHERE snapshot_version = $1 AND owner_id = ANY($2::bigint[])
			ORDER BY owner_id, id LIMIT $3`, run.Target.SourceVersion, ids, r.maxBatchRecords-records+1)
		if err != nil {
			return err
		}
		for rows.Next() {
			records++
			if records > r.maxBatchRecords {
				rows.Close()
				return ErrBatchRecordLimit
			}
			var s entity.Service
			if err := rows.Scan(&s.ID, &s.OwnerID, &s.Name, &s.PaymentTerm, &s.AutoProlong,
				&s.CreatedAt.Time, &s.ExpiresAt.Time, &s.Status, &s.Price); err != nil {
				rows.Close()
				return err
			}
			i := owners[s.OwnerID]
			inputs[i].Services = append(inputs[i].Services, s)
		}
		if err := rows.Err(); err != nil {
			return err
		}
		rows, err = tx.Query(ctx, `SELECT s.owner_id, t.id, t.service_id, t.amount, t.status, t.type, t.created_at, t.performed_at
			FROM transactions t JOIN services s ON s.snapshot_version = t.snapshot_version AND s.id = t.service_id
			WHERE t.snapshot_version = $1 AND s.owner_id = ANY($2::bigint[])
			ORDER BY s.owner_id, t.id LIMIT $3`, run.Target.SourceVersion, ids, r.maxBatchRecords-records+1)
		if err != nil {
			return err
		}
		defer rows.Close()
		for rows.Next() {
			records++
			if records > r.maxBatchRecords {
				return ErrBatchRecordLimit
			}
			var owner entity.ID
			var t entity.Transaction
			var performed *time.Time
			if err := rows.Scan(&owner, &t.ID, &t.ServiceID, &t.Amount, &t.Status, &t.Type, &t.CreatedAt.Time, &performed); err != nil {
				return err
			}
			if performed != nil {
				t.PerformedAt = &entity.SourceTimestamp{Time: *performed}
			}
			i := owners[owner]
			inputs[i].Transactions = append(inputs[i].Transactions, t)
		}
		return rows.Err()
	})
	if err != nil {
		return nil, err
	}
	return inputs, nil
}
