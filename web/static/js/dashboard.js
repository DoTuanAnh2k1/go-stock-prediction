// VN Stock Dashboard JavaScript
console.log('🚀 VN Stock Dashboard loaded!');

// Global variables
let isLoading = false;
const API_BASE = '/api';
let refreshInterval;

// Initialize dashboard when DOM is ready
document.addEventListener('DOMContentLoaded', function() {
    console.log('📊 Initializing dashboard...');
    
    // Initial load
    updateMarketStatus();
    loadWatchlist();
    loadPredictions();
    updateStats();
    
    // Setup auto-refresh
    setupAutoRefresh();
    
    // Setup event listeners
    setupEventListeners();
    
    console.log('✅ Dashboard initialized successfully!');
});

// Setup auto-refresh intervals
function setupAutoRefresh() {
    // Update market status every minute
    setInterval(updateMarketStatus, 60000);
    
    // Update last update time every minute
    setInterval(updateLastUpdateTime, 60000);
    
    // Auto-refresh data every 30 seconds (when not manually loading)
    refreshInterval = setInterval(() => {
        if (!isLoading) {
            console.log('🔄 Auto-refreshing data...');
            loadWatchlist(false); // Silent refresh
            loadPredictions(false); // Silent refresh
            updateStats();
        }
    }, 30000);
}

// Setup event listeners
function setupEventListeners() {
    // Handle visibility change to pause/resume auto-refresh
    document.addEventListener('visibilitychange', function() {
        if (document.hidden) {
            console.log('⏸️ Page hidden, pausing auto-refresh');
            clearInterval(refreshInterval);
        } else {
            console.log('▶️ Page visible, resuming auto-refresh');
            setupAutoRefresh();
        }
    });
    
    // Handle online/offline events
    window.addEventListener('online', function() {
        console.log('🌐 Connection restored, refreshing data');
        loadWatchlist();
        loadPredictions();
    });
    
    window.addEventListener('offline', function() {
        console.log('📡 Connection lost, showing cached data');
        showConnectionError();
    });
}

// Update market status based on Vietnam market hours
function updateMarketStatus() {
    const now = new Date();
    const hour = now.getHours();
    const minute = now.getMinutes();
    const day = now.getDay(); // 0 = Sunday, 6 = Saturday
    
    const statusElement = document.getElementById('marketStatus');
    const statusText = document.getElementById('marketStatusText');
    
    // Check if it's weekend
    if (day === 0 || day === 6) {
        statusElement.className = 'market-status closed';
        statusText.textContent = 'Market Closed (Weekend)';
        return;
    }
    
    // Vietnam market hours: 9:00-11:30 (morning), 13:00-15:00 (afternoon)
    const isOpen = (hour >= 9 && hour < 11) || 
                  (hour === 11 && minute <= 30) || 
                  (hour >= 13 && hour < 15);
    
    if (isOpen) {
        statusElement.className = 'market-status open';
        statusText.textContent = 'Market Open';
    } else if (hour < 9) {
        statusElement.className = 'market-status closed';
        statusText.textContent = 'Pre-Market';
    } else if ((hour === 11 && minute > 30) || hour === 12) {
        statusElement.className = 'market-status closed';
        statusText.textContent = 'Lunch Break';
    } else {
        statusElement.className = 'market-status closed';
        statusText.textContent = 'Market Closed';
    }
}

// Load watchlist data
async function loadWatchlist(showSpinner = true) {
    const symbols = 'VCB,VIC,FPT,VNM,HPG,MBB,GAS,TCB,BID,VRE';
    
    if (showSpinner) {
        showLoading('watchlistLoading');
    }
    
    try {
        console.log('📈 Loading watchlist data...');
        const response = await fetch(`${API_BASE}/stocks/watchlist?symbols=${symbols}`, {
            headers: {
                'Accept': 'application/json',
                'Cache-Control': 'no-cache'
            }
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        const data = await response.json();
        console.log("data:", data)
        // if (data.success && data.data && data.data.stocks) {
        if (data && data.stocks) {
            console.log(`✅ Loaded ${data.stocks.length} stocks`);
            renderWatchlist(data.stocks);
        } else {
            console.warn('⚠️ API returned unexpected format, using sample data');
            renderWatchlistError();
        }
    } catch (error) {
        console.error('❌ Error loading watchlist:', error);
        renderWatchlistError();
    } finally {
        if (showSpinner) {
            hideLoading('watchlistLoading');
        }
    }
}

// Render watchlist stocks
function renderWatchlist(stocks) {
    const grid = document.getElementById('watchlistGrid');
    
    if (!stocks || stocks.length === 0) {
        grid.innerHTML = '<div class="no-data">📊 No stock data available</div>';
        return;
    }
    
    grid.innerHTML = stocks.map(stock => `
        <div class="stock-item" data-symbol="${stock.stock.symbol}">
            <div class="stock-info">
                <h3>${stock.stock.symbol}</h3>
                <p>${stock.stock.company_name}</p>
            </div>
            <div class="stock-price">
                <div class="price">${formatPrice(stock.current_price)}</div>
                <div class="change ${getChangeClass(stock.change)}">
                    ${formatChange(stock.change, stock.change_percent)}
                </div>
            </div>
            <div class="status completed">Live</div>
        </div>
    `).join('');
    
    // Add click handlers for stock items
    grid.querySelectorAll('.stock-item').forEach(item => {
        item.addEventListener('click', function() {
            const symbol = this.dataset.symbol;
            showStockDetails(symbol);
        });
    });
}

// Render watchlist error with sample data
function renderWatchlistError() {
    const grid = document.getElementById('watchlistGrid');
    grid.innerHTML = `
        <div class="error-message">
            <p style="color: #e53e3e; margin-bottom: 15px;">📡 API not connected - Using sample data</p>
            ${renderSampleWatchlist()}
        </div>
    `;
}

// Render sample watchlist data
function renderSampleWatchlist() {
    const sampleStocks = [
        { symbol: 'VCB', name: 'Vietcombank', price: 87500, change: 1250, changePercent: 1.45 },
        { symbol: 'VIC', name: 'Vingroup', price: 58900, change: -800, changePercent: -1.34 },
        { symbol: 'FPT', name: 'FPT Corporation', price: 125000, change: 2500, changePercent: 2.04 },
        { symbol: 'VNM', name: 'Vinamilk', price: 56200, change: 300, changePercent: 0.54 },
        { symbol: 'HPG', name: 'Hoa Phat Group', price: 23450, change: -150, changePercent: -0.64 }
    ];
    
    return sampleStocks.map(stock => `
        <div class="stock-item" data-symbol="${stock.symbol}">
            <div class="stock-info">
                <h3>${stock.symbol}</h3>
                <p>${stock.name}</p>
            </div>
            <div class="stock-price">
                <div class="price">${formatPrice(stock.price)}</div>
                <div class="change ${getChangeClass(stock.change)}">
                    ${formatChange(stock.change, stock.changePercent)}
                </div>
            </div>
            <div class="status completed">Sample</div>
        </div>
    `).join('');
}

// Load predictions data
async function loadPredictions(showSpinner = true) {
    if (showSpinner) {
        showLoading('predictionsLoading');
    }
    
    try {
        console.log('🔮 Loading predictions data...');
        const response = await fetch(`${API_BASE}/predictions?limit=10`, {
            headers: {
                'Accept': 'application/json',
                'Cache-Control': 'no-cache'
            }
        });
        
        if (!response.ok) {
            throw new Error(`HTTP ${response.status}: ${response.statusText}`);
        }
        
        const data = await response.json();
        
        if (data.success && data.data && data.data.predictions) {
            console.log(`✅ Loaded ${data.data.predictions.length} predictions`);
            renderPredictions(data.data.predictions);
        } else {
            console.warn('⚠️ API returned unexpected format, using sample data');
            renderPredictionsError();
        }
    } catch (error) {
        console.error('❌ Error loading predictions:', error);
        renderPredictionsError();
    } finally {
        if (showSpinner) {
            hideLoading('predictionsLoading');
        }
    }
}

// Render predictions table
function renderPredictions(predictions) {
    const tbody = document.getElementById('predictionsTableBody');
    
    if (!predictions || predictions.length === 0) {
        tbody.innerHTML = '<tr><td colspan="7" class="no-data">🔮 No predictions available</td></tr>';
        return;
    }
    
    tbody.innerHTML = predictions.map(pred => `
        <tr data-symbol="${pred.stock.symbol}">
            <td><strong>${pred.stock.symbol}</strong></td>
            <td>${formatPrice(pred.stock.current_price || 0)}</td>
            <td>${formatPrice(pred.predicted_price)}</td>
            <td class="change ${getChangeClass(pred.predicted_price - (pred.stock.current_price || 0))}">
                ${formatChange(pred.predicted_price - (pred.stock.current_price || 0))}
            </td>
            <td><span class="algorithm-tag ${pred.algorithm_name}">${formatAlgorithmName(pred.algorithm_name)}</span></td>
            <td>${formatConfidence(pred.confidence)}</td>
            <td>${formatDate(pred.target_date)}</td>
        </tr>
    `).join('');
    
    // Add click handlers for prediction rows
    tbody.querySelectorAll('tr').forEach(row => {
        if (row.dataset.symbol) {
            row.addEventListener('click', function() {
                showPredictionDetails(this.dataset.symbol);
            });
        }
    });
}

// Render predictions error with sample data
function renderPredictionsError() {
    const tbody = document.getElementById('predictionsTableBody');
    tbody.innerHTML = `
        <tr><td colspan="7" class="error-message" style="text-align: center; color: #e53e3e;">
            📡 API not connected - Using sample data
        </td></tr>
        ${renderSamplePredictions()}
    `;
}

// Render sample predictions
function renderSamplePredictions() {
    const samplePredictions = [
        { symbol: 'VCB', current: 87500, predicted: 89200, algorithm: 'lstm_nn', confidence: 93 },
        { symbol: 'VIC', current: 58900, predicted: 57800, algorithm: 'arima_garch', confidence: 81 },
        { symbol: 'FPT', current: 125000, predicted: 128500, algorithm: 'moving_average', confidence: 76 },
        { symbol: 'VNM', current: 56200, predicted: 57100, algorithm: 'lstm_nn', confidence: 88 },
        { symbol: 'HPG', current: 23450, predicted: 24200, algorithm: 'arima_garch', confidence: 79 }
    ];
    
    return samplePredictions.map(pred => `
        <tr data-symbol="${pred.symbol}">
            <td><strong>${pred.symbol}</strong></td>
            <td>${formatPrice(pred.current)}</td>
            <td>${formatPrice(pred.predicted)}</td>
            <td class="change ${getChangeClass(pred.predicted - pred.current)}">
                ${formatChange(pred.predicted - pred.current)}
            </td>
            <td><span class="algorithm-tag ${pred.algorithm}">${formatAlgorithmName(pred.algorithm)}</span></td>
            <td>${pred.confidence}%</td>
            <td>Tomorrow</td>
        </tr>
    `).join('');
}

// Update dashboard statistics
async function updateStats() {
    try {
        console.log('📊 Updating statistics...');
        const response = await fetch(`${API_BASE}/training/status`);
        
        if (response.ok) {
            const data = await response.json();
            if (data.success) {
                // Update stats from API if available
                console.log('✅ Stats updated from API');
                // You can update actual stats here when API is ready
            }
        }
    } catch (error) {
        console.log('📊 Using mock stats (API not available)');
    }
    
    // Mock dynamic updates for demo
    updateMockStats();
}

// Update mock statistics for demo
function updateMockStats() {
    const totalPredictions = document.getElementById('totalPredictions');
    const avgAccuracy = document.getElementById('avgAccuracy');
    
    if (totalPredictions) {
        const current = parseInt(totalPredictions.textContent);
        totalPredictions.textContent = Math.max(200, current + Math.floor(Math.random() * 5));
    }
    
    if (avgAccuracy) {
        const variations = ['87.3%', '87.8%', '86.9%', '88.1%', '87.6%'];
        avgAccuracy.textContent = variations[Math.floor(Math.random() * variations.length)];
    }
}

// Utility Functions

// Format price in Vietnamese currency
function formatPrice(price) {
    if (typeof price === 'string') {
        price = parseFloat(price);
    }
    
    return new Intl.NumberFormat('vi-VN', {
        style: 'currency',
        currency: 'VND',
        minimumFractionDigits: 0,
        maximumFractionDigits: 0
    }).format(price || 0);
}

// Format price change with percentage
function formatChange(change, percent = null) {
    if (typeof change === 'string') {
        change = parseFloat(change);
    }
    
    const changeStr = change > 0 ? `+${formatPrice(Math.abs(change))}` : `-${formatPrice(Math.abs(change))}`;
    
    // Fix cái percent.toFixed ở đây nè!
    let percentStr = '';
    if (percent !== null && percent !== undefined) {
        // Convert percent to number nếu nó là string
        const percentNum = typeof percent === 'string' ? parseFloat(percent) : percent;
        
        // Check xem có phải valid number không
        if (!isNaN(percentNum) && isFinite(percentNum)) {
            percentStr = ` (${percentNum > 0 ? '+' : ''}${percentNum.toFixed(2)}%)`;
        }
    }
    
    return changeStr + percentStr;
}

// Get CSS class for price change
function getChangeClass(change) {
    if (typeof change === 'string') {
        change = parseFloat(change);
    }
    
    if (change > 0) return 'positive';
    if (change < 0) return 'negative';
    return 'neutral';
}

// Format algorithm name for display
function formatAlgorithmName(algorithm) {
    const nameMap = {
        'lstm_nn': 'LSTM',
        'arima_garch': 'ARIMA',
        'moving_average': 'MA',
        'lstm': 'LSTM',
        'arima': 'ARIMA',
        'ma': 'MA'
    };
    return nameMap[algorithm] || algorithm.toUpperCase();
}

// Format confidence percentage
function formatConfidence(confidence) {
    if (typeof confidence === 'string') {
        confidence = parseFloat(confidence);
    }
    
    // If confidence is between 0 and 1, convert to percentage
    if (confidence <= 1) {
        confidence *= 100;
    }
    
    return `${Math.round(confidence)}%`;
}

// Format date for display
function formatDate(dateStr) {
    if (!dateStr) return 'Tomorrow';
    
    try {
        const date = new Date(dateStr);
        const today = new Date();
        const tomorrow = new Date(today);
        tomorrow.setDate(tomorrow.getDate() + 1);
        
        if (date.toDateString() === today.toDateString()) {
            return 'Today';
        } else if (date.toDateString() === tomorrow.toDateString()) {
            return 'Tomorrow';
        } else {
            return date.toLocaleDateString('vi-VN', {
                day: '2-digit',
                month: '2-digit',
                year: 'numeric'
            });
        }
    } catch (error) {
        return 'Tomorrow';
    }
}

// Update last update time display
function updateLastUpdateTime() {
    const lastUpdate = document.getElementById('lastUpdate');
    const nextUpdate = document.getElementById('nextUpdate');
    
    if (lastUpdate) {
        const minutes = Math.floor(Math.random() * 5) + 1;
        lastUpdate.textContent = `${minutes}m ago`;
    }
    
    if (nextUpdate) {
        const nextMinutes = Math.floor(Math.random() * 3) + 1;
        nextUpdate.textContent = `Next in ${nextMinutes}m`;
    }
}

// Show loading spinner
function showLoading(elementId) {
    const element = document.getElementById(elementId);
    if (element) {
        element.classList.add('show');
        isLoading = true;
    }
}

// Hide loading spinner
function hideLoading(elementId) {
    const element = document.getElementById(elementId);
    if (element) {
        element.classList.remove('show');
        isLoading = false;
    }
}

// Show connection error
function showConnectionError() {
    const errorMsg = document.createElement('div');
    errorMsg.className = 'connection-error';
    errorMsg.innerHTML = `
        <div style="background: #fed7d7; color: #e53e3e; padding: 10px; border-radius: 8px; margin: 10px 0; text-align: center;">
            📡 Connection lost. Showing cached data.
        </div>
    `;
    
    const container = document.querySelector('.container');
    if (container && !container.querySelector('.connection-error')) {
        container.insertBefore(errorMsg, container.firstChild);
        
        // Remove error message after 5 seconds
        setTimeout(() => {
            if (errorMsg.parentNode) {
                errorMsg.parentNode.removeChild(errorMsg);
            }
        }, 5000);
    }
}

// Manual refresh functions (called by buttons)
function refreshWatchlist() {
    console.log('🔄 Manual watchlist refresh');
    loadWatchlist(true);
}

function refreshPredictions() {
    console.log('🔄 Manual predictions refresh');
    loadPredictions(true);
}

// Show stock details (placeholder for future feature)
function showStockDetails(symbol) {
    console.log(`📈 Show details for ${symbol}`);
    // Future: Open modal or navigate to stock detail page
    alert(`Stock details for ${symbol} - Coming soon!`);
}

// Show prediction details (placeholder for future feature)
function showPredictionDetails(symbol) {
    console.log(`🔮 Show prediction details for ${symbol}`);
    // Future: Open modal with detailed prediction analysis
    alert(`Prediction details for ${symbol} - Coming soon!`);
}

// Export functions for global access
window.refreshWatchlist = refreshWatchlist;
window.refreshPredictions = refreshPredictions;

// Performance monitoring
const performanceObserver = new PerformanceObserver((list) => {
    for (const entry of list.getEntries()) {
        if (entry.entryType === 'navigation') {
            console.log(`⚡ Page load time: ${entry.loadEventEnd - entry.loadEventStart}ms`);
        }
    }
});

try {
    performanceObserver.observe({ entryTypes: ['navigation'] });
} catch (e) {
    console.log('Performance Observer not supported');
}

console.log('🎉 Dashboard JavaScript loaded successfully!');