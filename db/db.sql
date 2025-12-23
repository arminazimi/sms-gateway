CREATE TABLE user_balances (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    balance BIGINT NOT NULL DEFAULT 0,
    last_updated DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_user_balances_user_id (user_id)
) ENGINE=InnoDB;

CREATE TABLE user_transactions (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    amount BIGINT NOT NULL,
    transaction_type VARCHAR(50) NOT NULL,
    description TEXT,
    transaction_id VARCHAR(50) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    UNIQUE KEY uq_user_transactions_transaction_id (transaction_id),
    INDEX idx_user_transactions_user_id (user_id, created_at)
) ENGINE=InnoDB;

CREATE TABLE sms_status (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    user_id BIGINT NOT NULL,
    status VARCHAR(50) NOT NULL,
    type VARCHAR(50) NOT NULL,
    recipient VARCHAR(20) NOT NULL,
    provider VARCHAR(50) NOT NULL DEFAULT '',
    sms_identifier VARCHAR(50) NOT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_sms_status_identifier_recipient (sms_identifier, recipient),
    INDEX idx_sms_status_user_identifier (user_id, sms_identifier),
    INDEX idx_sms_status_user_status_created (user_id, status, created_at)
) ENGINE=InnoDB;

CREATE TABLE outbox_events (
    id BIGINT PRIMARY KEY AUTO_INCREMENT,
    aggregate_type VARCHAR(50) NOT NULL,
    aggregate_id VARCHAR(50) NOT NULL,
    event_type VARCHAR(100) NOT NULL,
    payload JSON NOT NULL,
    priority INT NOT NULL DEFAULT 0,
    status VARCHAR(20) NOT NULL DEFAULT 'pending',
    attempts INT NOT NULL DEFAULT 0,
    next_run_at DATETIME NULL,
    last_error TEXT NULL,
    created_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP,
    updated_at DATETIME NOT NULL DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    UNIQUE KEY uq_outbox_aggregate_event (aggregate_id, event_type),
    INDEX idx_outbox_pending (status, priority, next_run_at, created_at)
) ENGINE=InnoDB;

# truncate table sms_status
# truncate table user_transactions
# truncate table user_balances

drop table sms_status
drop table user_transactions
drop table user_balances
drop table outbox_events

