package storage

const (
	schema = `	
				DROP TABLE users;
				DROP TABLE sessions;
				DROP TABLE orders;
				DROP TABLE withdrawals;


				CREATE TABLE users (
					id serial PRIMARY KEY,
					login VARCHAR (50) UNIQUE NOT NULL,
					password VARCHAR (60) NOT NULL,
					created_at TIMESTAMP default current_timestamp,
					last_login_at TIMESTAMP default current_timestamp
				);
				CREATE TABLE sessions (
				    token VARCHAR (64) UNIQUE NOT NULL,
				    user_id INTEGER UNIQUE NOT NULL,
				    expire_at TIMESTAMP default current_timestamp + '30 minutes'
				);
				CREATE TABLE orders (
				    number BIGINT UNIQUE NOT NULL,
				    user_id INTEGER NOT NULL,
				    status int NOT NULL default 1,
				    accrual REAL,
				    uploaded_at TIMESTAMP default current_timestamp
				);
				CREATE TABLE withdrawals (
				    "order" BIGINT UNIQUE NOT NULL,
				    user_id INTEGER NOT NULL,
				    sum REAL,
				    uploaded_at TIMESTAMP default current_timestamp
				);
				`

	userExistsQuery = `UPDATE users SET last_login_at = current_timestamp
					   WHERE login = $1
					   RETURNING id, login, password, last_login_at;`

	userCreateQuery = `INSERT INTO users (login, password)
					   VALUES($1, $2)
					   RETURNING id, login, password, last_login_at;`

	userOrdersQuery = `		SELECT number, status, accrual, uploaded_at FROM orders 
							WHERE user_id = $1
							ORDER BY uploaded_at`

	userWithdrawalsQuery = `SELECT "order", sum, uploaded_at FROM withdrawals 
							WHERE user_id = $1
							ORDER BY uploaded_at`

	sessionCreateQuery = `	INSERT INTO sessions (token, user_id, expire_at)
						  	VALUES($1, $2, current_timestamp + '30 minutes')
						  	ON CONFLICT(user_id)	DO UPDATE
 						  	SET token = $1, expire_at = current_timestamp + '30 minutes'
							RETURNING token, expire_at;`

	sessionCheckQuery = `	SELECT id, login, last_login_at, expire_at FROM sessions 
	    					LEFT JOIN users u on u.id = sessions.user_id 
							WHERE token = $1 and expire_at > current_timestamp;`

	ordersAmountQuery = `	SELECT sum(accrual) FROM orders 
							WHERE user_id = $1
							GROUP BY user_id`

	withdrawalsAmountQuery = `	SELECT sum(sum) FROM withdrawals 
								WHERE user_id = $1
								GROUP BY user_id`

	sessionKillQuery = `DELETE FROM sessions WHERE token = $1;`

	orderExistsQuery = `SELECT user_id FROM orders
						WHERE number = $1
						;`

	orderCreateQuery = `	INSERT INTO orders (number, user_id, status, accrual, uploaded_at)
							VALUES ($1, $2, $3, $4, $5)
			;`

	ordersUpdateQuery = `	UPDATE orders SET status = $1, accrual = $2;`

	withdrawalCreateQuery = `	INSERT INTO withdrawals ("order", user_id, sum, uploaded_at)
								VALUES ($1, $2, $3, $4)
			;`
)
