package storage

import (
	"context"
	"database/sql"
	"strings"
	"time"

	_ "github.com/jackc/pgx/v4/stdlib"

	"github.com/fedoroko/gophermart/internal/config"
	"github.com/fedoroko/gophermart/internal/orders"
	"github.com/fedoroko/gophermart/internal/users"
	"github.com/fedoroko/gophermart/internal/withdrawals"
)

//go:generate mockgen -destination=../mocks/mock_doer.go -package=mocks github.com/fedoroko/gophermart/internal/storage Repo
type Repo interface {
	UserExists(context.Context, *users.TempUser) (*users.Session, error)
	UserCreate(context.Context, *users.TempUser) (*users.Session, error)
	UserOrders(context.Context, *users.User) ([]*orders.Order, error)
	UserWithdrawals(context.Context, *users.User) ([]*withdrawals.Withdrawal, error)

	SessionCheck(context.Context, string) (*users.Session, error)
	SessionBalanceCheck(context.Context, string) (*users.Session, error)
	SessionKill(context.Context, *users.Session) error

	OrderCreate(context.Context, *orders.Order) error
	OrdersUpdate(context.Context, []*orders.Order) error
	OrdersRestore(context.Context) ([]*orders.Order, error)
	WithdrawalCreate(context.Context, *withdrawals.Withdrawal) error

	Close() error
}

type stmtQueries struct {
	userOrders       *sql.Stmt
	userWithdrawals  *sql.Stmt
	sessionCheck     *sql.Stmt
	withdrawalCreate *sql.Stmt
}

type postgres struct {
	*sql.DB
	logger *config.Logger
	cfg    *config.ServerConfig
	stmt   *stmtQueries
}

func (p *postgres) UserExists(ctx context.Context, user *users.TempUser) (*users.Session, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	var u users.TempUser
	err = tx.QueryRowContext(ctx, userExistsQuery, user.Login).
		Scan(&u.ID, &u.Login, &u.Password, &u.LastLogin)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			err = users.ThrowWrongPairErr()
		}
		if rollErr := tx.Rollback(); rollErr != nil {
			p.logger.Error().Stack().Err(rollErr).Send()
		}

		return nil, err
	}

	if !u.ConfirmPassword(user) {
		if rollErr := tx.Rollback(); rollErr != nil {
			p.logger.Error().Stack().Err(rollErr).Send()
		}

		return nil, users.ThrowWrongPairErr()
	}

	var s users.TempSession
	err = tx.QueryRowContext(ctx, sessionCreateQuery, user.GenerateToken(), u.ID).
		Scan(&s.Token, &s.Expire)
	if err != nil {
		if rollErr := tx.Rollback(); rollErr != nil {
			p.logger.Error().Stack().Err(rollErr).Send()
		}

		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	s.User = u.Commit()
	return s.Commit(), nil
}

func (p *postgres) UserCreate(ctx context.Context, user *users.TempUser) (*users.Session, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	var u users.TempUser
	bcrypt, _ := user.HashPassword()
	err = tx.QueryRowContext(ctx, userCreateQuery, user.Login, string(*bcrypt)).
		Scan(&u.ID, &u.Login, &u.Password, &u.LastLogin)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			err = users.ThrowAlreadyExistsErr()
		}
		if rollErr := tx.Rollback(); rollErr != nil {
			p.logger.Error().Stack().Err(rollErr).Send()
		}
		return nil, err
	}

	var s users.TempSession
	err = tx.QueryRowContext(ctx, sessionCreateQuery, user.GenerateToken(), u.ID).
		Scan(&s.Token, &s.Expire)
	if err != nil {
		if rollErr := tx.Rollback(); rollErr != nil {
			p.logger.Error().Stack().Err(rollErr).Send()
		}

		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	s.User = u.Commit()

	return s.Commit(), nil
}

func (p *postgres) UserOrders(ctx context.Context, user *users.User) ([]*orders.Order, error) {
	var ors []*orders.Order
	rows, err := p.stmt.userOrders.QueryContext(ctx, user.ID())

	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			err = orders.ThrowNoItemsErr()
		}
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		o := orders.Order{
			User: user,
		}

		err = rows.Scan(&o.Number, &o.Status, &o.Accrual, &o.UploadedAt)
		if err != nil {
			return ors, nil
		}

		ors = append(ors, &o)
	}

	if err = rows.Err(); err != nil {
		p.logger.Error().Caller().Err(err).Send()
		return ors, err
	}
	return ors, nil
}

func (p *postgres) UserWithdrawals(ctx context.Context, user *users.User) ([]*withdrawals.Withdrawal, error) {
	rows, err := p.stmt.userWithdrawals.QueryContext(ctx, user.ID())
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			err = orders.ThrowNoItemsErr()
		}
		return nil, err
	}

	defer rows.Close()

	var wrs []*withdrawals.Withdrawal
	for rows.Next() {
		w := withdrawals.Withdrawal{
			User: user,
		}

		err = rows.Scan(&w.Order, &w.Sum, &w.UploadedAt)
		if err != nil {
			return nil, err
		}

		wrs = append(wrs, &w)
	}

	if err = rows.Err(); err != nil {
		p.logger.Error().Caller().Err(err).Send()
		return nil, err
	}

	return wrs, nil
}

func (p *postgres) SessionCheck(ctx context.Context, token string) (*users.Session, error) {
	var u users.TempUser
	s := users.TempSession{
		Token: token,
	}

	err := p.stmt.sessionCheck.QueryRowContext(ctx, token).
		Scan(&u.ID, &u.Login, &u.LastLogin, &s.Expire)

	if err != nil {
		return nil, err
	}

	s.User = u.Commit()
	return s.Commit(), nil
}

func (p *postgres) SessionBalanceCheck(ctx context.Context, token string) (*users.Session, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	var zeroValue float64 = 0
	var u users.TempUser
	s := users.TempSession{
		Token: token,
	}
	err = tx.QueryRowContext(ctx, sessionCheckQuery, token).
		Scan(&u.ID, &u.Login, &u.LastLogin, &s.Expire)
	if err != nil {
		if rollErr := tx.Rollback(); rollErr != nil {
			p.logger.Error().Stack().Err(rollErr).Send()
		}
		return nil, err
	}

	err = tx.QueryRowContext(ctx, ordersAmountQuery, u.ID).
		Scan(&u.Balance)
	if err != nil {
		if !strings.Contains(err.Error(), "no rows") {
			if rollErr := tx.Rollback(); rollErr != nil {
				p.logger.Error().Stack().Err(rollErr).Send()
			}
			return nil, err
		}
		u.Balance = &zeroValue
	}

	err = tx.QueryRowContext(ctx, withdrawalsAmountQuery, u.ID).
		Scan(&u.Withdrawn)
	if err != nil {
		if !strings.Contains(err.Error(), "no rows") {
			if rollErr := tx.Rollback(); rollErr != nil {
				p.logger.Error().Stack().Err(rollErr).Send()
			}
			return nil, err
		}
		u.Withdrawn = &zeroValue
	}

	if u.Balance != nil && u.Withdrawn != nil {
		diff := *u.Balance - *u.Withdrawn
		u.Balance = &diff
	}
	s.User = u.Commit()
	return s.Commit(), nil
}

func (p *postgres) SessionKill(ctx context.Context, s *users.Session) error {
	return nil
}

func (p *postgres) OrderCreate(ctx context.Context, order *orders.Order) error {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	var existID *int64
	err = tx.QueryRowContext(ctx, orderExistsQuery, order.Number).
		Scan(&existID)
	if err != nil {
		if !strings.Contains(err.Error(), "no rows") {
			if rollErr := tx.Rollback(); rollErr != nil {
				p.logger.Error().Stack().Err(rollErr).Send()
			}
			return err
		}
	}

	if existID != nil {
		if *existID != order.User.ID() {
			return orders.ThrowBelongToAnotherErr()
		}

		return orders.ThrowAlreadyExistsErr()
	}

	_, err = tx.ExecContext(
		ctx, orderCreateQuery, order.Number, order.User.ID(), order.Status, order.Accrual, order.UploadedAt,
	)
	if err != nil {
		if rollErr := tx.Rollback(); rollErr != nil {
			p.logger.Error().Stack().Err(rollErr).Send()
		}
		return err
	}

	if err = tx.Commit(); err != nil {
		return err
	}

	return nil
}

func (p *postgres) OrdersUpdate(ctx context.Context, ors []*orders.Order) error {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}

	stmt, err := tx.Prepare(ordersUpdateQuery)
	if err != nil {
		return err
	}
	p.logger.Debug().Msg("orders_update transaction prepared")

	for _, o := range ors {
		p.logger.Debug().Caller().Interface("O R D E R", o).Send()
		if _, err = stmt.ExecContext(ctx, o.Status, o.Accrual); err != nil {
			if rollErr := tx.Rollback(); rollErr != nil {
				p.logger.Error().Stack().Err(rollErr).Send()
			}
			return err
		}
	}

	if err = tx.Commit(); err != nil {
		return err
	}
	p.logger.Debug().Msg("orders_update committed")
	return nil
}

func (p *postgres) OrdersRestore(ctx context.Context) ([]*orders.Order, error) {
	rows, err := p.DB.QueryContext(ctx, ordersRestoreQuery)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			err = orders.ThrowNoItemsErr()
		}
		return nil, err
	}

	var ors []*orders.Order
	for rows.Next() {
		o := &orders.Order{}
		if err = rows.Scan(&o.Number, &o.Status, &o.UploadedAt); err != nil {
			return ors, nil
		}

		ors = append(ors, o)
	}

	if err = rows.Err(); err != nil {
		return ors, err
	}

	return ors, nil
}

func (p *postgres) WithdrawalCreate(ctx context.Context, w *withdrawals.Withdrawal) error {
	_, err := p.stmt.withdrawalCreate.ExecContext(ctx, w.Order, w.User.ID(), w.Sum, w.UploadedAt)
	if err != nil {
		if strings.Contains(err.Error(), "duplicate key") {
			err = withdrawals.ThrowInvalidNumberErr()
		}
		return err
	}

	return nil
}

func (p *postgres) Close() error {
	p.logger.Info().Msg("DB: closed")
	return p.DB.Close()
}

func prepare(db *sql.DB) *stmtQueries {
	userOrders, err := db.Prepare(userOrdersQuery)
	if err != nil {
		panic(err)
	}

	userWithdrawals, err := db.Prepare(userWithdrawalsQuery)
	if err != nil {
		panic(err)
	}

	sessionCheck, err := db.Prepare(sessionCheckQuery)
	if err != nil {
		panic(err)
	}

	withdrawalCreate, err := db.Prepare(withdrawalCreateQuery)
	if err != nil {
		panic(err)
	}

	return &stmtQueries{
		userOrders:       userOrders,
		userWithdrawals:  userWithdrawals,
		sessionCheck:     sessionCheck,
		withdrawalCreate: withdrawalCreate,
	}
}

func Postgres(cfg *config.ServerConfig, logger *config.Logger) (Repo, error) {
	subLogger := logger.With().Str("Component", "POSTGRES-DB").Logger()
	logger = config.NewLogger(&subLogger)
	db, err := sql.Open("pgx", cfg.Database)
	if err != nil {
		panic(err)
	}

	db.SetMaxOpenConns(30)
	db.SetMaxIdleConns(30)
	db.SetConnMaxIdleTime(time.Second * 30)
	db.SetConnMaxLifetime(time.Minute * 2)

	if cfg.DBRefresh {
		logger.Debug().Msg("refreshing db")
		_, err = db.Exec(dropTables)
		if err != nil {
			logger.Debug().Err(err).Msg("refreshing failed")
		}
	}

	_, err = db.Exec(schema)
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			logger.Error().Caller().Err(err).Send()
			panic(err)
		}
	}

	stmt := prepare(db)
	logger.Info().Msg("DB: PREPARED")
	return &postgres{
		DB:     db,
		logger: logger,
		cfg:    cfg,
		stmt:   stmt,
	}, nil
}
