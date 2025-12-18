CREATE TABLE user_transactions (
                                   user_id BIGINT NOT NULL,
                                   amount BIGINT NOT NULL,  -- Positive for deposit, negative for withdrawal
                                   transaction_type VARCHAR(50) NOT NULL,  -- e.g., 'deposit', 'withdrawal', 'transfer'
                                   description TEXT,
                                   created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB;