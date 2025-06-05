CREATE TABLE exchanges (
    id SERIAL PRIMARY KEY,
    code VARCHAR(10) UNIQUE NOT NULL, 
    name VARCHAR(100) NOT NULL,
    timezone VARCHAR(50) DEFAULT 'Asia/Ho_Chi_Minh',
    trading_hours JSONB 
);

CREATE TABLE stocks (
    id SERIAL PRIMARY KEY,
    symbol VARCHAR(10) UNIQUE NOT NULL, 
    company_name VARCHAR(200) NOT NULL,
    exchange_id INTEGER REFERENCES exchanges(id),
    is_vn30 BOOLEAN DEFAULT FALSE,
    is_vn100 BOOLEAN DEFAULT FALSE,
    listing_date DATE,
    sector VARCHAR(100),
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE TABLE stock_prices (
    id SERIAL PRIMARY KEY,
    stock_id INTEGER REFERENCES stocks(id),
    trading_date DATE NOT NULL,
    open_price DECIMAL(15,2) NOT NULL,
    high_price DECIMAL(15,2) NOT NULL,
    low_price DECIMAL(15,2) NOT NULL,
    close_price DECIMAL(15,2) NOT NULL,
    volume BIGINT NOT NULL,
    value DECIMAL(20,2),
    foreign_buy BIGINT DEFAULT 0, 
    foreign_sell BIGINT DEFAULT 0, 
    created_at TIMESTAMP DEFAULT NOW(),
    UNIQUE(stock_id, trading_date)
);

CREATE TABLE predictions (
    id SERIAL PRIMARY KEY,
    stock_id INTEGER REFERENCES stocks(id),
    predicted_price DECIMAL(15,2) NOT NULL,
    confidence DECIMAL(5,4), 
    algorithm_name VARCHAR(50) NOT NULL,
    prediction_date DATE NOT NULL,
    target_date DATE NOT NULL, 
    actual_price DECIMAL(15,2), 
    accuracy DECIMAL(5,4), 
    created_at TIMESTAMP DEFAULT NOW()
);

CREATE INDEX idx_stock_prices_symbol_date ON stock_prices(stock_id, trading_date DESC);
CREATE INDEX idx_stocks_vn30 ON stocks(is_vn30) WHERE is_vn30 = TRUE;
CREATE INDEX idx_predictions_date ON predictions(prediction_date DESC);
CREATE INDEX idx_predictions_stock ON predictions(stock_id, target_date);