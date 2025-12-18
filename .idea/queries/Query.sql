CREATE TABLE user_balances (
    user_id BIGINT PRIMARY KEY,
    balance BIGINT NOT NULL DEFAULT 0,  )
    last_updated TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version BIGINT NOT NULL DEFAULT 0
);

insert into user_balances (user_id, balance)
values (1,100);