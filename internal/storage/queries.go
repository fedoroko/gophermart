package storage

const (
	schema = `	
-- 				DROP TABLE users;
-- 				DROP TABLE sessions;
-- 				DROP TABLE orders;


				CREATE TABLE users (
					id serial PRIMARY KEY,
					login VARCHAR (50) UNIQUE NOT NULL,
					password VARCHAR (60) NOT NULL,
					balance REAL,
					withdraw REAL,
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
				    withdrawal BOOLEAN default FALSE,
				    status int NOT NULL default 1,
				    sum REAL,
				    uploaded_at TIMESTAMP default current_timestamp
				);
				`

	userExistsQuery = `UPDATE users SET last_login_at = current_timestamp
					   WHERE login = $1
					   RETURNING id, login, password, balance, withdraw, last_login_at;`

	userCreateQuery = `INSERT INTO users (login, password)
					   VALUES($1, $2)
					   RETURNING id, login, password, balance, withdraw, last_login_at;`

	userUpdateQuery = `UPDATE users SET balance = $1, withdraw = $2
					   WHERE login = $3;`

	userOrdersQuery = `		SELECT number, status, sum, uploaded_at FROM orders 
							WHERE user_id = $1 and withdrawal = 0
							ORDER BY uploaded_at`

	userWithdrawalsQuery = `SELECT * FROM orders 
							WHERE user_id = $1 and withdrawal = 1
							ORDER BY uploaded_at`

	sessionCreateQuery = `	INSERT INTO sessions (token, user_id, expire_at)
						  	VALUES($1, $2, current_timestamp + '30 minutes')
						  	ON CONFLICT(user_id)	DO UPDATE
 						  	SET token = $1, expire_at = current_timestamp + '30 minutes'
							RETURNING token, expire_at;`

	sessionCheckQuery = `	SELECT id, login, balance, withdraw, last_login_at, expire_at FROM sessions 
	    					LEFT JOIN users u on u.id = sessions.user_id 
							WHERE token = $1 and expire_at > current_timestamp;`

	sessionKillQuery = `DELETE FROM sessions WHERE token = $1;`

	orderExistsQuery = `SELECT user_id FROM orders
						WHERE number = $1
						;`

	orderCreateQuery = `	INSERT INTO orders (number, user_id, withdrawal, status, sum, uploaded_at)
							VALUES ($1, $2, $3, $4, $5, current_timestamp)
			;`
)
