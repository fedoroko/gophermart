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
)

//go:generate mockgen -destination=../mocks/mock_doer.go -package=mocks github.com/fedoroko/gophermart/internal/storage Repo
type Repo interface {
	UserExists(context.Context, *users.TempUser) (*users.Session, error)
	UserCreate(context.Context, *users.TempUser) (*users.Session, error)
	UserUpdate(context.Context, *users.User) error
	UserOrders(context.Context, *users.User) ([]*orders.Order, error)
	UserWithdrawals(context.Context, *users.User) ([]*orders.Order, error)

	SessionCheck(context.Context, string) (*users.Session, error)
	SessionKill(context.Context, *users.Session) error

	OrderCreate(context.Context, *orders.TempOrder) (*orders.Order, error)

	Close() error
}

type stmtQueries struct {
	userUpdate      *sql.Stmt
	userOrders      *sql.Stmt
	userWithdrawals *sql.Stmt
	sessionCheck    *sql.Stmt
	orderCreate     *sql.Stmt
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
		Scan(&u.ID, &u.Login, &u.Password, &u.Balance, &u.Withdrawn, &u.LastLogin)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			err = users.ThrowWrongPairErr()
		}
		if rollErr := tx.Rollback(); rollErr != nil {
			p.logger.Error().Stack().Err(rollErr).Send()
		}

		return nil, err
	}

	if u.ConfirmPassword(user) == false {
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
		Scan(&u.ID, &u.Login, &u.Password, &u.Balance, &u.Withdrawn, &u.LastLogin)
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

func (p *postgres) UserUpdate(ctx context.Context, user *users.User) error {
	return nil
}

func (p *postgres) UserOrders(ctx context.Context, user *users.User) ([]*orders.Order, error) {
	var ors []*orders.Order
	rows, err := p.stmt.userOrders.QueryContext(ctx, user.Id())
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			err = orders.ThrowNoItemsErr()
		}
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		o := orders.TempOrder{
			User:         user,
			IsWithdrawal: false,
		}

		err = rows.Scan(&o.Number, &o.Status, &o.Sum, &o.UploadedAt)
		if err != nil {
			return ors, nil
		}

		ors = append(ors, o.Commit())
	}

	return ors, nil
}

func (p *postgres) UserWithdrawals(ctx context.Context, user *users.User) ([]*orders.Order, error) {
	var ors []*orders.Order
	rows, err := p.stmt.userWithdrawals.QueryContext(ctx, user.Id())
	if err != nil {
		if strings.Contains(err.Error(), "no rows") {
			err = orders.ThrowNoItemsErr()
		}
		return nil, err
	}

	defer rows.Close()

	for rows.Next() {
		o := orders.TempOrder{
			User:         user,
			IsWithdrawal: true,
		}

		err = rows.Scan(&o.Number, &o.Sum, &o.UploadedAt)
		if err != nil {
			return ors, nil
		}

		ors = append(ors, o.Commit())
	}

	return ors, nil
}

func (p *postgres) SessionCreate(ctx context.Context, user *users.User) (*users.Session, error) {

	return nil, nil
}

func (p *postgres) SessionCheck(ctx context.Context, token string) (*users.Session, error) {
	var u users.TempUser
	s := users.TempSession{
		Token: token,
	}
	err := p.stmt.sessionCheck.QueryRowContext(ctx, token).
		Scan(&u.ID, &u.Login, &u.Balance, &u.Withdrawn, &u.LastLogin, &s.Expire)
	if err != nil {
		return nil, err
	}

	s.User = u.Commit()
	return s.Commit(), nil
}

func (p *postgres) SessionKill(ctx context.Context, s *users.Session) error {
	return nil
}

func (p *postgres) OrderCreate(ctx context.Context, order *orders.TempOrder) (*orders.Order, error) {
	tx, err := p.DB.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}

	var existId *int64
	err = tx.QueryRowContext(ctx, orderExistsQuery, order.Number).
		Scan(&existId)
	if err != nil {
		if strings.Contains(err.Error(), "no rows") == false {
			if rollErr := tx.Rollback(); rollErr != nil {
				p.logger.Error().Stack().Err(rollErr).Send()
			}
			return nil, err
		}

	}

	if existId != nil {
		if *existId != order.User.Id() {
			return nil, orders.ThrowBelongToAnotherErr()
		}

		return nil, orders.ThrowAlreadyExistsErr()
	}

	_, err = tx.ExecContext(ctx, orderCreateQuery, order.Number, order.User.Id(), order.IsWithdrawal, order.Status, order.Sum)
	if err != nil {
		if rollErr := tx.Rollback(); rollErr != nil {
			p.logger.Error().Stack().Err(rollErr).Send()
		}
		return nil, err
	}

	if err = tx.Commit(); err != nil {
		return nil, err
	}

	return order.Commit(), nil
}

func (p *postgres) Close() error {
	p.logger.Info().Msg("DB: closed")
	return p.DB.Close()
}

func prepare(db *sql.DB) *stmtQueries {
	userUpdate, err := db.Prepare(userUpdateQuery)
	if err != nil {
		panic(err)
	}

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

	orderCreate, err := db.Prepare(orderCreateQuery)
	if err != nil {
		panic(err)
	}

	return &stmtQueries{
		userUpdate:      userUpdate,
		userOrders:      userOrders,
		userWithdrawals: userWithdrawals,
		sessionCheck:    sessionCheck,
		orderCreate:     orderCreate,
	}
}

func Postgres(cfg *config.ServerConfig, logger *config.Logger) (Repo, error) {
	db, err := sql.Open("pgx", cfg.Database)
	if err != nil {
		panic(err)
	}

	db.SetMaxOpenConns(30)
	db.SetMaxIdleConns(30)
	db.SetConnMaxIdleTime(time.Second * 30)
	db.SetConnMaxLifetime(time.Minute * 2)

	_, err = db.Exec(schema)
	if err != nil {
		if !strings.Contains(err.Error(), "already exists") {
			panic(err)
		}
	}

	stmt := prepare(db)

	subLogger := logger.With().Str("Component", "POSTGRES-DB").Logger()

	return &postgres{
		DB:     db,
		logger: config.NewLogger(&subLogger),
		cfg:    cfg,
		stmt:   stmt,
	}, nil
}
