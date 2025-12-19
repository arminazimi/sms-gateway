CREATE TABLE user_balances (
    user_id BIGINT PRIMARY KEY,
    balance BIGINT NOT NULL DEFAULT 0,
    last_updated TIMESTAMPTZ NOT NULL DEFAULT CURRENT_TIMESTAMP
)ENGINE=InnoDB;

CREATE TABLE user_transactions (
        user_id BIGINT NOT NULL,
        amount BIGINT NOT NULL,  -- Positive for deposit, negative for withdrawal
        transaction_type VARCHAR(50) NOT NULL,  -- e.g., 'deposit', 'withdrawal', 'transfer'
        description TEXT,
        created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
) ENGINE=InnoDB;

CREATE TABLE sms_status (
        user_id BIGINT NOT NULL,
        status VARCHAR(50) NOT NULL,
        type  VARCHAR(50) NOT NULL,
        recipient VARCHAR(20) NOT NULL,
        Provider VARCHAR(50)  NOT NULL default '',
        created_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6),
        updated_at DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB;