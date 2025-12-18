CREATE TABLE user_balances (
    user_id BIGINT PRIMARY KEY,
    balance BIGINT NOT NULL DEFAULT 0,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP,
    version BIGINT NOT NULL DEFAULT 0
);

-- seed
insert into user_balances (user_id, balance)
values (1,100);


CREATE TABLE user_transactions (
                                   transaction_id BIGINT AUTO_INCREMENT PRIMARY KEY,
                                   user_id BIGINT NOT NULL,
                                   amount BIGINT NOT NULL,  -- Positive for deposit, negative for withdrawal
                                   new_balance BIGINT NOT NULL,  -- Balance after this transaction
                                   transaction_type VARCHAR(50) NOT NULL,  -- e.g., 'deposit', 'withdrawal', 'transfer'
                                   description TEXT,
                                   created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),

                                   INDEX idx_user_id_created_at (user_id, created_at DESC),
                                   INDEX idx_transaction_type (transaction_type),
                                   FOREIGN KEY (user_id) REFERENCES user_balances(user_id) ON DELETE CASCADE
) ENGINE=InnoDB;