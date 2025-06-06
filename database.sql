-- ============================================
-- VN STOCK PREDICTION DATABASE SCHEMA
-- Fixed version based on models_db structs
-- ============================================

-- Drop existing tables if needed (uncomment if you want fresh start)
-- DROP TABLE IF EXISTS predictions;
-- DROP TABLE IF EXISTS stock_prices;
-- DROP TABLE IF EXISTS stocks;
-- DROP TABLE IF EXISTS exchanges;
-- DROP TABLE IF EXISTS sync_logs;

-- ============================================
-- EXCHANGES TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS exchanges (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    code VARCHAR(10) NOT NULL UNIQUE COMMENT 'Exchange code: HOSE, HNX, UPCOM',
    name VARCHAR(100) NOT NULL COMMENT 'Exchange full name',
    timezone VARCHAR(50) DEFAULT 'Asia/Ho_Chi_Minh' COMMENT 'Exchange timezone',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    INDEX idx_exchanges_code (code),
    INDEX idx_exchanges_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Stock exchanges table';

-- ============================================
-- STOCKS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS stocks (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    symbol VARCHAR(10) NOT NULL UNIQUE COMMENT 'Stock symbol: VCB, VIC, FPT',
    company_name VARCHAR(200) NOT NULL COMMENT 'Company full name',
    exchange_id INT UNSIGNED NOT NULL COMMENT 'Foreign key to exchanges',
    is_vn30 BOOLEAN DEFAULT FALSE COMMENT 'Is VN30 stock',
    is_vn100 BOOLEAN DEFAULT FALSE COMMENT 'Is VN100 stock', 
    listing_date DATE NULL COMMENT 'Stock listing date',
    sector VARCHAR(100) NULL COMMENT 'Business sector: Banking, Tech, etc',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    
    -- Indexes
    INDEX idx_stocks_symbol (symbol),
    INDEX idx_stocks_exchange_id (exchange_id),
    INDEX idx_stocks_is_vn30 (is_vn30),
    INDEX idx_stocks_sector (sector),
    INDEX idx_stocks_deleted_at (deleted_at),
    
    -- Foreign key
    FOREIGN KEY (exchange_id) REFERENCES exchanges(id) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Stocks master data';

-- ============================================
-- STOCK_PRICES TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS stock_prices (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    stock_id INT UNSIGNED NOT NULL COMMENT 'Foreign key to stocks',
    trading_date DATE NOT NULL COMMENT 'Trading date',
    open_price DECIMAL(15,2) NOT NULL COMMENT 'Opening price',
    high_price DECIMAL(15,2) NOT NULL COMMENT 'Highest price',
    low_price DECIMAL(15,2) NOT NULL COMMENT 'Lowest price',
    close_price DECIMAL(15,2) NOT NULL COMMENT 'Closing price',
    volume BIGINT NOT NULL COMMENT 'Trading volume',
    value DECIMAL(20,2) DEFAULT 0 COMMENT 'Trading value in VND',
    
    -- THESE ARE THE MISSING COLUMNS IN YOUR DB!
    `change` DECIMAL(15,2) DEFAULT 0 COMMENT 'Price change amount',
    change_percent DECIMAL(5,4) DEFAULT 0 COMMENT 'Price change percentage',
    
    foreign_buy BIGINT DEFAULT 0 COMMENT 'Foreign buy volume',
    foreign_sell BIGINT DEFAULT 0 COMMENT 'Foreign sell volume',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    
    -- Indexes
    UNIQUE KEY uk_stock_prices_stock_date (stock_id, trading_date),
    INDEX idx_stock_prices_trading_date (trading_date),
    INDEX idx_stock_prices_deleted_at (deleted_at),
    
    -- Foreign key
    FOREIGN KEY (stock_id) REFERENCES stocks(id) ON DELETE RESTRICT ON UPDATE CASCADE
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci;

-- ============================================
-- PREDICTIONS TABLE  
-- ============================================
CREATE TABLE IF NOT EXISTS predictions (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    stock_id INT UNSIGNED NOT NULL COMMENT 'Foreign key to stocks',
    predicted_price DECIMAL(15,2) NOT NULL COMMENT 'ML predicted price',
    confidence DECIMAL(5,4) DEFAULT 0.0000 COMMENT 'Prediction confidence 0-1',
    algorithm_name VARCHAR(50) NOT NULL COMMENT 'ML algorithm used',
    
    -- FIX: Use DATETIME instead of TIMESTAMP to avoid timezone issues
    prediction_date DATETIME NOT NULL COMMENT 'When prediction was made',
    target_date DATETIME NOT NULL COMMENT 'Target date for prediction',
    
    actual_price DECIMAL(15,2) NULL COMMENT 'Actual price on target date',
    accuracy DECIMAL(5,4) NULL COMMENT 'Prediction accuracy 0-1',
    
    -- FIX: Only one TIMESTAMP column can have DEFAULT CURRENT_TIMESTAMP
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    
    -- Indexes
    INDEX idx_predictions_stock_id (stock_id),
    INDEX idx_predictions_algorithm (algorithm_name),
    INDEX idx_predictions_prediction_date (prediction_date),
    INDEX idx_predictions_target_date (target_date),
    INDEX idx_predictions_deleted_at (deleted_at),
    
    -- Foreign key constraint
    CONSTRAINT fk_predictions_stock_id 
        FOREIGN KEY (stock_id) REFERENCES stocks(id) 
        ON DELETE RESTRICT ON UPDATE CASCADE
        
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci 
COMMENT='ML predictions data';

-- ============================================
-- SYNC_LOGS TABLE
-- ============================================
CREATE TABLE IF NOT EXISTS sync_logs (
    id INT UNSIGNED AUTO_INCREMENT PRIMARY KEY,
    sync_date TIMESTAMP NOT NULL COMMENT 'When sync happened',
    success_count INT NOT NULL DEFAULT 0 COMMENT 'Number of successful syncs',
    error_count INT NOT NULL DEFAULT 0 COMMENT 'Number of failed syncs',
    duration_ms BIGINT NOT NULL COMMENT 'Sync duration in milliseconds',
    source VARCHAR(50) NULL COMMENT 'Data source: VietStock, CafeF, etc',
    error_message TEXT NULL COMMENT 'Error details if any',
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP ON UPDATE CURRENT_TIMESTAMP,
    deleted_at TIMESTAMP NULL DEFAULT NULL,
    
    -- Indexes
    INDEX idx_sync_logs_sync_date (sync_date),
    INDEX idx_sync_logs_source (source),
    INDEX idx_sync_logs_deleted_at (deleted_at)
) ENGINE=InnoDB DEFAULT CHARSET=utf8mb4 COLLATE=utf8mb4_unicode_ci COMMENT='Data synchronization logs';

-- ============================================
-- SAMPLE DATA INSERTION
-- ============================================

-- Insert sample exchanges
INSERT IGNORE INTO exchanges (code, name, timezone) VALUES
('HOSE', 'Ho Chi Minh Stock Exchange', 'Asia/Ho_Chi_Minh'),
('HNX', 'Hanoi Stock Exchange', 'Asia/Ho_Chi_Minh'),
('UPCOM', 'Unlisted Public Company Market', 'Asia/Ho_Chi_Minh');

-- Insert sample VN30 stocks
INSERT IGNORE INTO stocks (symbol, company_name, exchange_id, is_vn30, is_vn100, sector) VALUES
('VCB', 'Ngân hàng Ngoại thương Việt Nam', 1, TRUE, TRUE, 'Banking'),
('VIC', 'Tập đoàn Vingroup', 1, TRUE, TRUE, 'Real Estate'),
('FPT', 'Tập đoàn FPT', 1, TRUE, TRUE, 'Technology'),
('VNM', 'Công ty Cổ phần Sữa Việt Nam', 1, TRUE, TRUE, 'Consumer Goods'),
('HPG', 'Tập đoàn Hòa Phát', 1, TRUE, TRUE, 'Industrial'),
('GAS', 'Tổng công ty Khí Việt Nam', 1, TRUE, TRUE, 'Energy'),
('MBB', 'Ngân hàng Quân đội', 1, TRUE, TRUE, 'Banking'),
('TCB', 'Ngân hàng Kỹ thương Việt Nam', 1, TRUE, TRUE, 'Banking'),
('BID', 'Ngân hàng Đầu tư và Phát triển Việt Nam', 1, TRUE, TRUE, 'Banking'),
('VRE', 'Vincom Retail', 1, TRUE, TRUE, 'Real Estate');

-- ============================================
-- USEFUL QUERIES FOR DEBUGGING
-- ============================================

-- Check table structures
-- DESCRIBE exchanges;
-- DESCRIBE stocks; 
-- DESCRIBE stock_prices;
-- DESCRIBE predictions;
-- DESCRIBE sync_logs;

-- Check foreign key constraints
-- SELECT * FROM information_schema.TABLE_CONSTRAINTS 
-- WHERE CONSTRAINT_SCHEMA = 'your_database_name' 
-- AND CONSTRAINT_TYPE = 'FOREIGN KEY';

-- Check indexes
-- SHOW INDEX FROM stocks;
-- SHOW INDEX FROM stock_prices;
-- SHOW INDEX FROM predictions;

-- Sample queries to verify data
-- SELECT s.symbol, s.company_name, e.name as exchange 
-- FROM stocks s JOIN exchanges e ON s.exchange_id = e.id 
-- WHERE s.is_vn30 = TRUE;