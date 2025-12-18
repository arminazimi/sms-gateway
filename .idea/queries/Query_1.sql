CREATE TABLE user_balances (
                               user_id BIGINT PRIMARY KEY,
                               balance BIGINT NOT NULL DEFAULT 0,  -- In smallest currency unit (e.g., cents)
                               last_updated DATETIME(6) NOT NULL DEFAULT CURRENT_TIMESTAMP(6) ON UPDATE CURRENT_TIMESTAMP(6)
) ENGINE=InnoDB;