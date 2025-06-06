// Stocks page JavaScript
console.log('📈 Stocks page loaded!');

// Page-specific variables
let currentPage = 1;
let pageSize = 20;
let currentFilters = {};
let currentSort = { by: 'symbol', order: 'asc' };

// Initialize stocks page
document.addEventListener('DOMContentLoaded', function() {
    console.log('📊 Initializing stocks page...');
    
    // Load initial data
    loadVN30Overview();
    loadAllStocks();
    loadTopMovers();
    loadMostActive();
    
    // Setup event listeners
    setupStocksEventListeners();
    
    // Auto-refresh every 60 seconds
    setInterval(() => {
        if (!isLoading) {
            refreshAllData();
        }
    }, 60000);
    
    console.log('✅ Stocks page initialized!');
});

// Setup event listeners
function setupStocksEventListeners() {
    // Search functionality
    const searchInput = document.getElementById('stockSearch');
    if (searchInput) {
        searchInput.addEventListener('input', debounce(searchStocks, 300));
        searchInput.addEventListener('keypress', function(e) {
            if (e.key === 'Enter') {
                searchStocks();
            }
        });
    }
    
    // Sort dropdown
    const sortBy = document.getElementById('sortBy');
    if (sortBy) {
        sortBy.addEventListener('change', function() {
            currentSort.by = this.value;
            loadAllStocks();
        });
    }
}

// Load VN30 overview
async function loadVN30Overview() {
    try {
        console.log('📈 Loading VN30 overview...');
        
        // Mock data since API might not be ready
        const mockData = {
            index: 1234.56,
            change: 12.34,
            changePercent: 1.02,
            volume: 125600000,
            value: 3200000000000
        };
        
        updateVN30Display(mockData);
        
    } catch (error) {
        console.error('❌ Error loading VN30 overview:', error);
        showNotification('Failed to load VN30 overview', 'error');
    }
}

// Update VN30 display
function updateVN30Display(data) {
    const indexElement = document.getElementById('vn30Index');
    const changeElement = document.getElementById('vn30Change');
    const volumeElement = document.getElementById('totalVolume');
    const valueElement = document.getElementById('totalValue');
    
    if (indexElement) {
        indexElement.textContent = data.index.toLocaleString('vi-VN', {
            minimumFractionDigits: 2,
            maximumFractionDigits: 2
        });
    }
    
    if (changeElement) {
        const changeClass = data.change > 0 ? 'positive' : data.change < 0 ? 'negative' : 'neutral';
        changeElement.className = `change ${changeClass}`;
        changeElement.textContent = `${data.change > 0 ? '+' : ''}${data.change.toFixed(2)} (${data.changePercent > 0 ? '+' : ''}${data.changePercent.toFixed(2)}%)`;
    }
    
    if (volumeElement) {
        volumeElement.textContent = formatVolume(data.volume);
    }
    
    if (valueElement) {
        valueElement.textContent = formatValue(data.value);
    }
}

// Load all stocks
async function loadAllStocks() {
    showLoading('stocksLoading');
    
    try {
        console.log('📊 Loading all stocks...');
        
        // Try to call API first
        const response = await apiCall('/market/overview');
        
        if (response.success && response.data) {
            renderStocksTable(response.data.stocks || []);
        } else {
            throw new Error('API returned no data');
        }
        
    } catch (error) {
        console.error('❌ Error loading stocks:', error);
        renderSampleStocks();
    } finally {
        hideLoading('stocksLoading');
    }
}

// Render stocks table
function renderStocksTable(stocks) {
    const tbody = document.getElementById('stocksTableBody');
    if (!tbody) return;
    
    if (!stocks || stocks.length === 0) {
        tbody.innerHTML = '<tr><td colspan="10" class="no-data">📊 No stocks data available</td></tr>';
        return;
    }
    
    tbody.innerHTML = stocks.map(stock => `
        <tr onclick="showStockDetail('${stock.symbol}')" style="cursor: pointer;">
            <td><strong>${stock.symbol}</strong></td>
            <td>${stock.company_name}</td>
            <td>${stock.sector || 'N/A'}</td>
            <td>${formatPrice(stock.current_price)}</td>
            <td class="change ${getChangeClass(stock.change)}">${formatPrice(stock.change)}</td>
            <td class="change ${getChangeClass(stock.change_percent)}">${stock.change_percent?.toFixed(2)}%</td>
            <td>${formatVolume(stock.volume)}</td>
            <td>${formatPrice(stock.value)}</td>
            <td>${stock.is_vn30 ? '✅' : '❌'}</td>
            <td>
                <button class="action-btn small" onclick="event.stopPropagation(); addToWatchlist('${stock.symbol}')">
                    ⭐ Watch
                </button>
            </td>
        </tr>
    `).join('');
}

// Render sample stocks
function renderSampleStocks() {
    const sampleStocks = [
        { symbol: 'VCB', company_name: 'Vietcombank', sector: 'Banking', current_price: 87500, change: 1250, change_percent: 1.45, volume: 2500000, value: 218750000000, is_vn30: true },
        { symbol: 'VIC', company_name: 'Vingroup', sector: 'Real Estate', current_price: 58900, change: -800, change_percent: -1.34, volume: 3200000, value: 188480000000, is_vn30: true },
        { symbol: 'FPT', company_name: 'FPT Corporation', sector: 'Technology', current_price: 125000, change: 2500, change_percent: 2.04, volume: 1800000, value: 225000000000, is_vn30: true },
        { symbol: 'VNM', company_name: 'Vinamilk', sector: 'Consumer Goods', current_price: 56200, change: 300, change_percent: 0.54, volume: 1500000, value: 84300000000, is_vn30: true },
        { symbol: 'HPG', company_name: 'Hoa Phat Group', sector: 'Industrial', current_price: 23450, change: -150, change_percent: -0.64, volume: 4200000, value: 98490000000, is_vn30: true },
        { symbol: 'GAS', company_name: 'PetroVietnam Gas', sector: 'Energy', current_price: 67800, change: 800, change_percent: 1.19, volume: 800000, value: 54240000000, is_vn30: true },
        { symbol: 'MBB', company_name: 'Military Bank', sector: 'Banking', current_price: 25600, change: 200, change_percent: 0.79, volume: 3100000, value: 79360000000, is_vn30: true },
        { symbol: 'TCB', company_name: 'Techcombank', sector: 'Banking', current_price: 48900, change: -300, change_percent: -0.61, volume: 2800000, value: 136920000000, is_vn30: true }
    ];
    
    renderStocksTable(sampleStocks);
}

// Load top movers (gainers and losers)
async function loadTopMovers() {
    showLoading('gainersLoading');
    showLoading('losersLoading');
    
    try {
        console.log('🚀 Loading top movers...');
        
        // Sample data for top gainers
        const topGainers = [
            { symbol: 'FPT', change_percent: 2.04, current_price: 125000 },
            { symbol: 'VCB', change_percent: 1.45, current_price: 87500 },
            { symbol: 'GAS', change_percent: 1.19, current_price: 67800 },
            { symbol: 'MBB', change_percent: 0.79, current_price: 25600 },
            { symbol: 'VNM', change_percent: 0.54, current_price: 56200 }
        ];
        
        // Sample data for top losers
        const topLosers = [
            { symbol: 'VIC', change_percent: -1.34, current_price: 58900 },
            { symbol: 'HPG', change_percent: -0.64, current_price: 23450 },
            { symbol: 'TCB', change_percent: -0.61, current_price: 48900 },
            { symbol: 'BID', change_percent: -0.45, current_price: 42300 },
            { symbol: 'SSI', change_percent: -0.32, current_price: 18700 }
        ];
        
        renderTopMovers('topGainers', topGainers, 'gainer');
        renderTopMovers('topLosers', topLosers, 'loser');
        
    } catch (error) {
        console.error('❌ Error loading top movers:', error);
    } finally {
        hideLoading('gainersLoading');
        hideLoading('losersLoading');
    }
}

// Render top movers
function renderTopMovers(containerId, stocks, type) {
    const container = document.getElementById(containerId);
    if (!container) return;
    
    container.innerHTML = stocks.map(stock => `
        <div class="stock-item" onclick="showStockDetail('${stock.symbol}')">
            <div class="stock-info">
                <h3>${stock.symbol}</h3>
                <p>${formatPrice(stock.current_price)}</p>
            </div>
            <div class="stock-price">
                <div class="change ${type === 'gainer' ? 'positive' : 'negative'}">
                    ${stock.change_percent > 0 ? '+' : ''}${stock.change_percent.toFixed(2)}%
                </div>
            </div>
        </div>
    `).join('');
}

// Load most active stocks
async function loadMostActive() {
    showLoading('activeLoading');
    
    try {
        console.log('🔥 Loading most active stocks...');
        
        const mostActive = [
            { symbol: 'HPG', volume: 4200000, current_price: 23450, value: 98490000000 },
            { symbol: 'VIC', volume: 3200000, current_price: 58900, value: 188480000000 },
            { symbol: 'MBB', volume: 3100000, current_price: 25600, value: 79360000000 },
            { symbol: 'TCB', volume: 2800000, current_price: 48900, value: 136920000000 },
            { symbol: 'VCB', volume: 2500000, current_price: 87500, value: 218750000000 },
            { symbol: 'FPT', volume: 1800000, current_price: 125000, value: 225000000000 }
        ];
        
        renderMostActive(mostActive);
        
    } catch (error) {
        console.error('❌ Error loading most active:', error);
    } finally {
        hideLoading('activeLoading');
    }
}

// Render most active stocks
function renderMostActive(stocks) {
    const container = document.getElementById('mostActive');
    if (!container) return;
    
    container.innerHTML = `
        <div class="active-stocks-grid">
            ${stocks.map(stock => `
                <div class="stock-item active-stock" onclick="showStockDetail('${stock.symbol}')">
                    <div class="stock-info">
                        <h3>${stock.symbol}</h3>
                        <p>Volume: ${formatVolume(stock.volume)}</p>
                    </div>
                    <div class="stock-price">
                        <div class="price">${formatPrice(stock.current_price)}</div>
                        <div class="value">Value: ${formatValue(stock.value)}</div>
                    </div>
                </div>
            `).join('')}
        </div>
    `;
}

// Search stocks function
function searchStocks() {
    const searchTerm = document.getElementById('stockSearch')?.value.toLowerCase();
    
    if (!searchTerm) {
        loadAllStocks();
        return;
    }
    
    console.log('🔍 Searching for:', searchTerm);
    
    // Filter existing table rows
    const tableRows = document.querySelectorAll('#stocksTableBody tr');
    let visibleCount = 0;
    
    tableRows.forEach(row => {
        const symbol = row.querySelector('td')?.textContent.toLowerCase();
        const company = row.querySelectorAll('td')[1]?.textContent.toLowerCase();
        
        if (symbol?.includes(searchTerm) || company?.includes(searchTerm)) {
            row.style.display = '';
            visibleCount++;
        } else {
            row.style.display = 'none';
        }
    });
    
    if (visibleCount === 0) {
        document.getElementById('stocksTableBody').innerHTML = 
            '<tr><td colspan="10" class="no-data">🔍 No stocks found matching your search</td></tr>';
    }
}

// Apply filters function
function applyFilters() {
    const sectorFilter = document.getElementById('sectorFilter')?.value;
    const exchangeFilter = document.getElementById('exchangeFilter')?.value;
    
    console.log('🎯 Applying filters:', { sector: sectorFilter, exchange: exchangeFilter });
    
    currentFilters = {
        sector: sectorFilter,
        exchange: exchangeFilter
    };
    
    // In a real app, this would call the API with filters
    // For now, just show notification
    showNotification(`Filters applied: ${sectorFilter || 'All sectors'}, ${exchangeFilter || 'All exchanges'}`, 'info');
    
    // Reload data with filters
    loadAllStocks();
}

// Refresh all data
function refreshAllData() {
    console.log('🔄 Refreshing all stocks data...');
    loadVN30Overview();
    loadAllStocks();
    loadTopMovers();
    loadMostActive();
}

// Manual refresh function (called by button)
function refreshStocks() {
    showNotification('Refreshing stocks data...', 'info');
    refreshAllData();
}

// Show stock detail (placeholder)
function showStockDetail(symbol) {
    console.log('📈 Show stock detail for:', symbol);
    showNotification(`Stock details for ${symbol} - Coming soon!`, 'info');
    // TODO: Implement stock detail modal or page
}

// Add to watchlist (placeholder)
function addToWatchlist(symbol) {
    console.log('⭐ Add to watchlist:', symbol);
    showNotification(`${symbol} added to watchlist!`, 'success');
    // TODO: Implement watchlist functionality
}

// Utility functions
function formatVolume(volume) {
    if (volume >= 1000000) {
        return (volume / 1000000).toFixed(1) + 'M';
    } else if (volume >= 1000) {
        return (volume / 1000).toFixed(0) + 'K';
    }
    return volume.toLocaleString();
}

function formatValue(value) {
    if (value >= 1000000000000) {
        return (value / 1000000000000).toFixed(1) + 'T VND';
    } else if (value >= 1000000000) {
        return (value / 1000000000).toFixed(1) + 'B VND';
    } else if (value >= 1000000) {
        return (value / 1000000).toFixed(1) + 'M VND';
    }
    return formatPrice(value);
}

// Debounce function for search
function debounce(func, wait) {
    let timeout;
    return function executedFunction(...args) {
        const later = () => {
            clearTimeout(timeout);
            func(...args);
        };
        clearTimeout(timeout);
        timeout = setTimeout(later, wait);
    };
}

// Export functions for global access
window.refreshStocks = refreshStocks;
window.searchStocks = searchStocks;
window.applyFilters = applyFilters;
window.showStockDetail = showStockDetail;
window.addToWatchlist = addToWatchlist;

console.log('🎉 Stocks JavaScript ready!');