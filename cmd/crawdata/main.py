#!/usr/bin/env python3
# -*- coding: utf-8 -*-
"""
VNStock Historical Data Crawler - Siêu xịn sò cho VN30! 🚀
Lấy data từ vnstock rồi ném vào database MySQL của mày!
VERSION: Fixed for VNStock 3.x API
"""

import os
# Fix Unicode encoding trên Windows
os.environ['PYTHONIOENCODING'] = 'utf-8'

from vnstock import Vnstock  # New syntax!
import mysql.connector
from datetime import datetime, timedelta
import pandas as pd
import time
import sys
import logging
from decimal import Decimal
import traceback

# Setup logging cho đỡ bị mù tịt - fixed encoding
logging.basicConfig(
    level=logging.INFO,
    format='%(asctime)s - %(levelname)s - %(message)s',
    handlers=[
        logging.FileHandler('vnstock_crawler.log', encoding='utf-8'),
        logging.StreamHandler()
    ]
)
logger = logging.getLogger(__name__)

class VNStockCrawler:
    """
    Class siêu xịn để crawl data VN30 từ vnstock! 
    WTF, professional vl luôn! 😎
    """
    
    def __init__(self, db_config):
        """
        Initialize crawler với database config
        """
        self.db_config = db_config
        self.connection = None
        
        # VN30 symbols - list chuẩn chỉnh nhất VN! 💪
        self.vn30_symbols = [
            'ACB', 'BCM', 'BID', 'BVH', 'CTG', 'FPT', 'GAS', 'GVR', 
            'HDB', 'HPG', 'MBB', 'MSN', 'MWG', 'PLX', 'POW', 'SAB', 
            'SHB', 'SSB', 'SSI', 'STB', 'TCB', 'TPB', 'VCB', 'VHM', 
            'VIC', 'VJC', 'VNM', 'VPB', 'VRE', 'VTI'
        ]
        
        # Initialize VNStock instance - NEW WAY!
        self.vnstock = Vnstock()
        
        logger.info("🚀 VNStock Crawler initialized successfully!")
        logger.info(f"📊 Will crawl {len(self.vn30_symbols)} VN30 stocks")

    def connect_to_database(self):
        """
        Kết nối database - đơn giản vl!
        """
        try:
            self.connection = mysql.connector.connect(**self.db_config)
            logger.info("✅ Connected to MySQL database successfully!")
            return True
        except Exception as e:
            logger.error(f"❌ Database connection failed: {e}")
            return False

    def close_database_connection(self):
        """
        Đóng connection - clean up như người lành thiện!
        """
        if self.connection and self.connection.is_connected():
            self.connection.close()
            logger.info("🔒 Database connection closed")

    def get_stock_id_by_symbol(self, symbol):
        """
        Lấy stock_id từ symbol - chuẩn bị insert data
        """
        try:
            cursor = self.connection.cursor()
            query = "SELECT id FROM stocks WHERE symbol = %s"
            cursor.execute(query, (symbol,))
            result = cursor.fetchone()
            cursor.close()
            
            if result:
                return result[0]
            else:
                logger.warning(f"⚠️ Stock {symbol} not found in database")
                return None
                
        except Exception as e:
            logger.error(f"❌ Error getting stock ID for {symbol}: {e}")
            return None

    def fetch_historical_data(self, symbol, days_back=120):
        """
        Fetch data từ vnstock - cái này là heart của toàn bộ system! 💝
        UPDATED FOR VNSTOCK 3.x API!
        """
        try:
            # Tính ngày bắt đầu và kết thúc
            end_date = datetime.now().strftime('%Y-%m-%d')
            start_date = (datetime.now() - timedelta(days=days_back)).strftime('%Y-%m-%d')
            
            logger.info(f"📈 Fetching data for {symbol} from {start_date} to {end_date}")
            
            # NEW VNSTOCK 3.x SYNTAX - magic happens here! ✨
            stock_obj = self.vnstock.stock(symbol=symbol, source='VCI')
            df = stock_obj.quote.history(
                start=start_date,
                end=end_date,
                interval='1D'
            )
            
            if df is None or df.empty:
                logger.warning(f"⚠️ No data returned for {symbol}")
                return None
            
            # DEBUG: Log DataFrame structure để hiểu nó trả về gì!
            logger.info(f"🔍 DataFrame info for {symbol}:")
            logger.info(f"   - Shape: {df.shape}")
            logger.info(f"   - Index type: {type(df.index)}")
            logger.info(f"   - Index: {df.index}")
            logger.info(f"   - Columns: {list(df.columns)}")
            logger.info(f"   - First row: {df.iloc[0].to_dict()}")
                
            logger.info(f"✅ Successfully fetched {len(df)} records for {symbol}")
            return df
            
        except Exception as e:
            logger.error(f"❌ Error fetching data for {symbol}: {e}")
            logger.error(f"Stack trace: {traceback.format_exc()}")
            return None

    def process_and_save_data(self, symbol, df):
        """
        Process dataframe và save vào database - cái này quan trọng vl! 🔥
        """
        try:
            # Get stock ID
            stock_id = self.get_stock_id_by_symbol(symbol)
            if not stock_id:
                logger.error(f"❌ Cannot find stock ID for {symbol}")
                return False

            cursor = self.connection.cursor()
            insert_count = 0
            update_count = 0
            
            # Process từng row
            for index, row in df.iterrows():
                try:
                    # Prepare data - vnstock 3.x trả về theo format mới
                    # DATE NẰM Ở COLUMN 'time' CHỨ KHÔNG PHẢI INDEX!
                    trading_date = pd.to_datetime(row['time']).date()
                    
                    # Extract OHLC data - check multiple column name formats
                    open_price = Decimal(str(row.get('open', row.get('Open', 0))))
                    high_price = Decimal(str(row.get('high', row.get('High', 0))))
                    low_price = Decimal(str(row.get('low', row.get('Low', 0))))
                    close_price = Decimal(str(row.get('close', row.get('Close', 0))))
                    volume = int(row.get('volume', row.get('Volume', 0)))
                    
                    # Tính change và change_percent
                    change = Decimal('0')
                    change_percent = Decimal('0')
                    if 'change' in row and row['change'] is not None:
                        change = Decimal(str(row['change']))
                    if 'change_percent' in row and row['change_percent'] is not None:
                        change_percent = Decimal(str(row['change_percent']))
                    
                    # Value = close_price * volume (estimate)
                    value = close_price * volume if close_price and volume else Decimal('0')
                    
                    # Check if record exists
                    check_query = """
                        SELECT id FROM stock_prices 
                        WHERE stock_id = %s AND trading_date = %s
                    """
                    cursor.execute(check_query, (stock_id, trading_date))
                    existing = cursor.fetchone()
                    
                    if existing:
                        # Update existing record
                        update_query = """
                            UPDATE stock_prices SET
                                open_price = %s, high_price = %s, low_price = %s, close_price = %s,
                                volume = %s, value = %s, `change` = %s, change_percent = %s,
                                updated_at = NOW()
                            WHERE stock_id = %s AND trading_date = %s
                        """
                        cursor.execute(update_query, (
                            open_price, high_price, low_price, close_price,
                            volume, value, change, change_percent,
                            stock_id, trading_date
                        ))
                        update_count += 1
                    else:
                        # Insert new record
                        insert_query = """
                            INSERT INTO stock_prices (
                                stock_id, trading_date, open_price, high_price, low_price, close_price,
                                volume, value, `change`, change_percent, created_at, updated_at
                            ) VALUES (
                                %s, %s, %s, %s, %s, %s, %s, %s, %s, %s, NOW(), NOW()
                            )
                        """
                        cursor.execute(insert_query, (
                            stock_id, trading_date, open_price, high_price, low_price, close_price,
                            volume, value, change, change_percent
                        ))
                        insert_count += 1
                except Exception as row_error:
                    logger.error(f"❌ Error processing row for {symbol}: {row_error}")
                    continue
                    
            # Commit all changes
            try:
                self.connection.commit()
                logger.info(f"✅ {symbol}: Inserted {insert_count} new records, Updated {update_count} records")
                return True
            except Exception as commit_error:
                logger.error(f"❌ Error committing data for {symbol}: {commit_error}")
                if self.connection:
                    self.connection.rollback()
                return False
            finally:
                cursor.close()
                
        except Exception as e:
            logger.error(f"❌ Error saving data for {symbol}: {e}")
            if self.connection:
                self.connection.rollback()
            return False

    def crawl_all_vn30(self, days_back=90):
        """
        Crawl toàn bộ VN30 - main function siêu xịn! 🎯
        """
        logger.info("🚀 Starting VN30 historical data crawling...")
        
        if not self.connect_to_database():
            logger.error("❌ Cannot connect to database. Aborting!")
            return False
        
        success_count = 0
        error_count = 0
        
        try:
            for i, symbol in enumerate(self.vn30_symbols, 1):
                logger.info(f"📊 Processing {symbol} ({i}/{len(self.vn30_symbols)})")
                
                try:
                    # Fetch data từ vnstock
                    df = self.fetch_historical_data(symbol, days_back)
                    
                    if df is not None and not df.empty:
                        # Save vào database
                        if self.process_and_save_data(symbol, df):
                            success_count += 1
                            logger.info(f"✅ {symbol} completed successfully!")
                        else:
                            error_count += 1
                            logger.error(f"❌ Failed to save {symbol} to database")
                    else:
                        error_count += 1
                        logger.warning(f"⚠️ No data available for {symbol}")
                    
                    # Delay để tránh bị rate limit - lịch sự với API!
                    if i < len(self.vn30_symbols):
                        time.sleep(2)  # 2 giây delay
                        
                except Exception as symbol_error:
                    error_count += 1
                    logger.error(f"❌ Error processing {symbol}: {symbol_error}")
                    continue
            
            # Summary báo cáo
            logger.info("🎉 VN30 crawling completed!")
            logger.info(f"📊 Summary: {success_count} successful, {error_count} errors")
            
            return success_count > 0
            
        finally:
            self.close_database_connection()

def main():
    """
    Main function - entry point của toàn bộ chương trình! 🎬
    """
    print("🚀 VNStock Historical Data Crawler")
    print("="*50)
    
    # Database configuration - thay đổi theo config của mày!
    db_config = {
        'host': 'localhost',
        'port': 3306,
        'user': 'root',
        'password': '123',  # Thay password của mày vào đây!
        'database': 'stock',  # Tên database của mày
        'charset': 'utf8mb4',
        'autocommit': False
    }
    
    try:
        # Khởi tạo crawler
        crawler = VNStockCrawler(db_config)
        
        # Crawl VN30 data - 120 ngày gần nhất (4 tháng đầy đủ!)
        success = crawler.crawl_all_vn30(days_back=500)
        
        if success:
            print("🎉 Crawling completed successfully!")
            print("💾 Historical data has been saved to database")
        else:
            print("❌ Crawling failed or no data saved")
            
    except KeyboardInterrupt:
        print("\n⏹️ Crawling interrupted by user")
    except Exception as e:
        logger.error(f"💥 Unexpected error: {e}")
        print(f"💥 Error: {e}")

if __name__ == "__main__":
    main()