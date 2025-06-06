// Predictions page JavaScript - Complete version
console.log('🔮 Predictions page loaded and ready!');

// Page-specific variables
let currentFilters = {
    algorithm: '',
    timeRange: '7',
    stock: ''
};
let predictionData = [];
let isRefreshing = false;

// Initialize predictions page
document.addEventListener('DOMContentLoaded', function() {
    console.log('🎯 Initializing predictions page...');
    
    // Load initial data
    loadAlgorithmPerformance();
    loadPredictionSummary();
    loadAllPredictions();
    loadSuccessfulPredictions();
    loadAccuracyChart();
    
    // Setup event listeners
    setupPredictionsEventListeners();
    
    // Auto-refresh every 2 minutes
    setInterval(() => {
        if (!isLoading && !isRefreshing) {
            refreshPredictionsData();
        }
    }, 120000);
    
    console.log('✅ Predictions page initialized successfully!');
});

// Setup event listeners
function setupPredictionsEventListeners() {
    console.log('🎛️ Setting up event listeners...');
    
    // Filter change handlers
    const algorithmFilter = document.getElementById('algorithmFilter');
    const timeRangeFilter = document.getElementById('timeRangeFilter');
    const stockFilter = document.getElementById('stockFilter');
    
    if (algorithmFilter) {
        algorithmFilter.addEventListener('change', applyPredictionFilters);
    }
    if (timeRangeFilter) {
        timeRangeFilter.addEventListener('change', applyPredictionFilters);
    }
    if (stockFilter) {
        stockFilter.addEventListener('change', applyPredictionFilters);
    }
    
    // Populate stock filter dropdown
    populateStockFilter();
}

// Populate stock filter dropdown
function populateStockFilter() {
    const stockFilter = document.getElementById('stockFilter');
    if (!stockFilter) return;
    
    const vn30Stocks = [
        'VCB', 'VIC', 'FPT', 'VNM', 'HPG', 'MBB', 'GAS', 'TCB', 'BID', 'VRE',
        'ACB', 'BCM', 'BVH', 'CTG', 'GVR', 'HDB', 'MSN', 'MWG', 'PLX', 'POW',
        'SAB', 'SHB', 'SSB', 'SSI', 'STB', 'TPB', 'VJC', 'VPB', 'VTI'
    ];
    
    vn30Stocks.forEach(symbol => {
        const option = document.createElement('option');
        option.value = symbol;
        option.textContent = `${symbol} - VN30 Stock`;
        stockFilter.appendChild(option);
    });
}

// Load algorithm performance
async function loadAlgorithmPerformance() {
    try {
        console.log('🏆 Loading algorithm performance...');
        
        // Try API call first
        const response = await apiCall('/algorithms/comparison?days=30');
        
        if (response.success && response.data && response.data.algorithms) {
            renderAlgorithmPerformance(response.data.algorithms);
        } else {
            throw new Error('API not available or invalid response');
        }
        
    } catch (error) {
        console.error('❌ API not available, using sample data:', error);
        renderSampleAlgorithmPerformance();
    }
}

// Render algorithm performance
function renderAlgorithmPerformance(algorithms) {
    const container = document.getElementById('algoPerformance');
    if (!container) {
        console.warn('Algorithm performance container not found');
        return;
    }
    
    container.innerHTML = algorithms.map(algo => `
        <div class="algo-card" onclick="showAlgorithmDetails('${algo.name}')">
            <h3>${formatAlgorithmName(algo.name)}</h3>
            <div class="performance-stats">
                <div class="stat">
                    <span class="label">Accuracy:</span>
                    <span class="value">${parseFloat(algo.accuracy_rate || 0).toFixed(1)}%</span>
                </div>
                <div class="stat">
                    <span class="label">Predictions:</span>
                    <span class="value">${algo.total_predictions || 0}</span>
                </div>
                <div class="stat">
                    <span class="label">Avg Error:</span>
                    <span class="value">${parseFloat(algo.avg_error || 0).toFixed(1)}%</span>
                </div>
            </div>
            <div class="trend ${getTrendClass(algo.trend || 'stable')}">
                ${getTrendIcon(algo.trend || 'stable')} ${formatTrendText(algo.trend || 'stable')}
            </div>
        </div>
    `).join('');
}

// Render sample algorithm performance
function renderSampleAlgorithmPerformance() {
    const sampleData = [
        { 
            name: 'lstm_nn', 
            accuracy_rate: 93.2, 
            total_predictions: 156, 
            avg_error: 2.1, 
            trend: 'improving',
            last_prediction: '2024-01-15T10:30:00Z'
        },
        { 
            name: 'arima_garch', 
            accuracy_rate: 80.1, 
            total_predictions: 142, 
            avg_error: 3.2, 
            trend: 'stable',
            last_prediction: '2024-01-15T10:25:00Z'
        },
        { 
            name: 'moving_average', 
            accuracy_rate: 75.5, 
            total_predictions: 148, 
            avg_error: 4.1, 
            trend: 'declining',
            last_prediction: '2024-01-15T10:20:00Z'
        }
    ];
    
    renderAlgorithmPerformance(sampleData);
}

// Load prediction summary
async function loadPredictionSummary() {
    try {
        console.log('📊 Loading prediction summary...');
        
        // Try API first, fallback to mock data
        const response = await apiCall('/predictions/summary');
        
        if (response.success && response.data) {
            updatePredictionSummary(response.data);
        } else {
            throw new Error('API not available');
        }
        
    } catch (error) {
        console.error('❌ Using mock summary data:', error);
        
        // Mock summary data
        const summaryData = {
            totalPredictions: 446,
            overallAccuracy: 83.2,
            confirmed: 127,
            avgError: 2.8
        };
        
        updatePredictionSummary(summaryData);
    }
}

// Update prediction summary display
function updatePredictionSummary(data) {
    const summaryContainer = document.querySelector('.prediction-summary');
    if (!summaryContainer) {
        console.warn('Prediction summary container not found');
        return;
    }
    
    summaryContainer.innerHTML = `
        <div class="summary-stat">
            <div class="number">${data.totalPredictions || 0}</div>
            <div class="label">Total Predictions</div>
            <div class="sublabel">Last 30 days</div>
        </div>
        <div class="summary-stat">
            <div class="number">${parseFloat(data.overallAccuracy || 0).toFixed(1)}%</div>
            <div class="label">Overall Accuracy</div>
            <div class="sublabel">All algorithms</div>
        </div>
        <div class="summary-stat">
            <div class="number">${data.confirmed || 0}</div>
            <div class="label">Confirmed</div>
            <div class="sublabel">With actual prices</div>
        </div>
        <div class="summary-stat">
            <div class="number">${parseFloat(data.avgError || 0).toFixed(1)}%</div>
            <div class="label">Avg Error</div>
            <div class="sublabel">Price difference</div>
        </div>
    `;
}

// Load all predictions
async function loadAllPredictions() {
    showLoading('predictionsLoading');
    
    try {
        console.log('🔮 Loading all predictions...');
        
        const response = await apiCall('/predictions?limit=50');
        
        if (response.success && response.data && response.data.predictions) {
            predictionData = response.data.predictions;
            renderPredictionsTable(predictionData);
            updatePaginationInfo(response.data);
        } else {
            throw new Error('API not available or invalid response');
        }
        
    } catch (error) {
        console.error('❌ API not available, using sample data:', error);
        renderSamplePredictions();
    } finally {
        hideLoading('predictionsLoading');
    }
}

// Render predictions table
function renderPredictionsTable(predictions) {
    const tbody = document.getElementById('predictionsTableBody');
    if (!tbody) {
        console.warn('Predictions table body not found');
        return;
    }
    
    if (!predictions || predictions.length === 0) {
        tbody.innerHTML = '<tr><td colspan="10" class="no-data">🔮 No predictions available</td></tr>';
        return;
    }
    
    tbody.innerHTML = predictions.map((pred, index) => `
        <tr onclick="showPredictionDetail(${pred.id || index})" style="cursor: pointer;" title="Click to view details">
            <td><strong>${pred.stock?.symbol || pred.symbol || 'N/A'}</strong></td>
            <td><span class="algorithm-tag ${pred.algorithm_name || 'unknown'}">${formatAlgorithmName(pred.algorithm_name || 'unknown')}</span></td>
            <td>${formatPrice(pred.predicted_price || 0)}</td>
            <td>${formatPrice(pred.current_price || 0)}</td>
            <td class="change ${getChangeClass((pred.predicted_price || 0) - (pred.current_price || 0))}">
                ${formatChange((pred.predicted_price || 0) - (pred.current_price || 0))}
            </td>
            <td>${formatConfidence(pred.confidence || 0)}</td>
            <td>${formatDate(pred.target_date)}</td>
            <td>${pred.actual_price ? formatPrice(pred.actual_price) : '-'}</td>
            <td>${pred.accuracy ? formatConfidence(pred.accuracy) : '-'}</td>
            <td><span class="status ${getPredictionStatusClass(pred)}">${getPredictionStatusText(pred)}</span></td>
        </tr>
    `).join('');
}

// Render sample predictions
function renderSamplePredictions() {
    const samplePredictions = [
        { 
            id: 1, 
            symbol: 'VCB', 
            algorithm_name: 'lstm_nn', 
            predicted_price: 89200, 
            current_price: 87500, 
            confidence: 0.93, 
            target_date: '2024-01-16T15:00:00Z', 
            actual_price: 88900, 
            accuracy: 0.91,
            prediction_date: '2024-01-15T09:00:00Z'
        },
        { 
            id: 2, 
            symbol: 'VIC', 
            algorithm_name: 'arima_garch', 
            predicted_price: 57800, 
            current_price: 58900, 
            confidence: 0.81, 
            target_date: '2024-01-16T15:00:00Z', 
            actual_price: null, 
            accuracy: null,
            prediction_date: '2024-01-15T09:00:00Z'
        },
        { 
            id: 3, 
            symbol: 'FPT', 
            algorithm_name: 'moving_average', 
            predicted_price: 128500, 
            current_price: 125000, 
            confidence: 0.76, 
            target_date: '2024-01-16T15:00:00Z', 
            actual_price: null, 
            accuracy: null,
            prediction_date: '2024-01-15T09:00:00Z'
        },
        { 
            id: 4, 
            symbol: 'VNM', 
            algorithm_name: 'lstm_nn', 
            predicted_price: 57100, 
            current_price: 56200, 
            confidence: 0.88, 
            target_date: '2024-01-16T15:00:00Z', 
            actual_price: 56950, 
            accuracy: 0.85,
            prediction_date: '2024-01-15T09:00:00Z'
        },
        { 
            id: 5, 
            symbol: 'HPG', 
            algorithm_name: 'arima_garch', 
            predicted_price: 24200, 
            current_price: 23450, 
            confidence: 0.79, 
            target_date: '2024-01-16T15:00:00Z', 
            actual_price: null, 
            accuracy: null,
            prediction_date: '2024-01-15T09:00:00Z'
        }
    ];
    
    predictionData = samplePredictions;
    renderPredictionsTable(samplePredictions);
}

// Load successful predictions
async function loadSuccessfulPredictions() {
    showLoading('successfulLoading');
    
    try {
        console.log('✅ Loading successful predictions...');
        
        const response = await apiCall('/predictions?status=confirmed&limit=10');
        
        if (response.success && response.data) {
            renderSuccessfulPredictions(response.data.predictions || []);
        } else {
            throw new Error('API not available');
        }
        
    } catch (error) {
        console.error('❌ Using sample successful predictions:', error);
        
        const successfulPreds = [
            { symbol: 'VCB', predicted: 89200, actual: 88900, accuracy: 91, algorithm: 'lstm_nn', date: '2024-01-15' },
            { symbol: 'VNM', predicted: 57100, actual: 56950, accuracy: 85, algorithm: 'lstm_nn', date: '2024-01-15' },
            { symbol: 'GAS', predicted: 68500, actual: 68200, accuracy: 87, algorithm: 'arima_garch', date: '2024-01-14' },
            { symbol: 'MBB', predicted: 25800, actual: 25650, accuracy: 82, algorithm: 'moving_average', date: '2024-01-14' },
            { symbol: 'TCB', predicted: 49200, actual: 49050, accuracy: 89, algorithm: 'lstm_nn', date: '2024-01-13' }
        ];
        
        renderSuccessfulPredictions(successfulPreds);
    } finally {
        hideLoading('successfulLoading');
    }
}

// Render successful predictions
function renderSuccessfulPredictions(predictions) {
    const container = document.getElementById('successfulPredictions');
    if (!container) {
        console.warn('Successful predictions container not found');
        return;
    }
    
    if (!predictions || predictions.length === 0) {
        container.innerHTML = '<div class="no-data">✅ No successful predictions found</div>';
        return;
    }
    
    container.innerHTML = predictions.map(pred => `
        <div class="success-prediction-item" onclick="showPredictionDetail('${pred.symbol}')">
            <div class="pred-header">
                <h4>${pred.symbol}</h4>
                <span class="algorithm-tag ${pred.algorithm}">${formatAlgorithmName(pred.algorithm)}</span>
            </div>
            <div class="pred-details">
                <div class="pred-stat">
                    <span class="label">Predicted:</span>
                    <span class="value">${formatPrice(pred.predicted)}</span>
                </div>
                <div class="pred-stat">
                    <span class="label">Actual:</span>
                    <span class="value">${formatPrice(pred.actual)}</span>
                </div>
                <div class="pred-stat">
                    <span class="label">Accuracy:</span>
                    <span class="value accuracy">${pred.accuracy}%</span>
                </div>
            </div>
            <div class="pred-date">${formatDate(pred.date)}</div>
        </div>
    `).join('');
}

// Load accuracy chart
function loadAccuracyChart() {
    console.log('📈 Loading accuracy chart...');
    
    const chartCanvas = document.getElementById('accuracyChart');
    if (!chartCanvas) {
        console.warn('Accuracy chart canvas not found');
        return;
    }
    
    // Mock chart data for now - in real app, you'd use Chart.js or similar
    const ctx = chartCanvas.getContext('2d');
    
    // Simple chart simulation
    ctx.fillStyle = '#4299e1';
    ctx.fillRect(50, 50, 100, 200);
    ctx.fillText('LSTM: 93%', 60, 40);
    
    ctx.fillStyle = '#d69e2e';
    ctx.fillRect(200, 100, 100, 150);
    ctx.fillText('ARIMA: 80%', 210, 90);
    
    ctx.fillStyle = '#38a169';
    ctx.fillRect(350, 120, 100, 130);
    ctx.fillText('MA: 75%', 360, 110);
}

// Apply prediction filters
function applyPredictionFilters() {
    console.log('🎯 Applying prediction filters...');
    
    updateFilters();
    
    // Filter existing data
    let filteredData = [...predictionData];
    
    if (currentFilters.algorithm) {
        filteredData = filteredData.filter(pred => pred.algorithm_name === currentFilters.algorithm);
    }
    
    if (currentFilters.stock) {
        filteredData = filteredData.filter(pred => 
            (pred.stock?.symbol || pred.symbol) === currentFilters.stock
        );
    }
    
    // Time range filter would be handled by API in real app
    // For now, just show notification
    
    renderPredictionsTable(filteredData);
    
    showNotification(`Filters applied: ${filteredData.length} predictions shown`, 'info');
}

// Update filters from form
function updateFilters() {
    currentFilters.algorithm = document.getElementById('algorithmFilter')?.value || '';
    currentFilters.timeRange = document.getElementById('timeRangeFilter')?.value || '7';
    currentFilters.stock = document.getElementById('stockFilter')?.value || '';
    
    console.log('Updated filters:', currentFilters);
}

// Export predictions to CSV
function exportPredictions() {
    console.log('📁 Exporting predictions to CSV...');
    
    if (!predictionData || predictionData.length === 0) {
        showNotification('No predictions to export!', 'warning');
        return;
    }
    
    try {
        const headers = ['Stock', 'Algorithm', 'Predicted Price', 'Current Price', 'Expected Change', 'Confidence', 'Target Date', 'Actual Price', 'Accuracy', 'Status'];
        const csvData = predictionData.map(pred => [
            pred.stock?.symbol || pred.symbol || 'N/A',
            pred.algorithm_name || 'unknown',
            pred.predicted_price || 0,
            pred.current_price || 0,
            (pred.predicted_price || 0) - (pred.current_price || 0),
            pred.confidence || 0,
            pred.target_date || '',
            pred.actual_price || '',
            pred.accuracy || '',
            getPredictionStatusText(pred)
        ]);
        
        const csv = [headers, ...csvData].map(row => row.join(',')).join('\n');
        
        const blob = new Blob([csv], { type: 'text/csv' });
        const url = window.URL.createObjectURL(blob);
        const a = document.createElement('a');
        a.href = url;
        a.download = `predictions_${new Date().toISOString().split('T')[0]}.csv`;
        document.body.appendChild(a);
        a.click();
        document.body.removeChild(a);
        window.URL.revokeObjectURL(url);
        
        showNotification('Predictions exported to CSV successfully!', 'success');
    } catch (error) {
        console.error('❌ Export failed:', error);
        showNotification('Failed to export predictions!', 'error');
    }
}

// Update pagination info
function updatePaginationInfo(data) {
    const pagination = document.getElementById('predictionsPagination');
    if (!pagination) return;
    
    pagination.innerHTML = `
        <div class="pagination-info">
            Showing ${data.predictions?.length || 0} of ${data.total || 0} predictions
        </div>
    `;
}

// Refresh predictions data
function refreshPredictionsData() {
    if (isRefreshing) return;
    
    isRefreshing = true;
    console.log('🔄 Refreshing predictions data...');
    
    Promise.all([
        loadAlgorithmPerformance(),
        loadPredictionSummary(),
        loadAllPredictions(),
        loadSuccessfulPredictions()
    ]).finally(() => {
        isRefreshing = false;
    });
}

// Manual refresh function (called by button)
function refreshPredictions() {
    showNotification('Refreshing predictions data...', 'info');
    refreshPredictionsData();
}

// Show prediction detail (placeholder)
function showPredictionDetail(predictionId) {
    console.log('🔮 Show prediction detail for ID:', predictionId);
    showNotification(`Prediction details for ID ${predictionId} - Coming soon!`, 'info');
    // TODO: Implement prediction detail modal with charts and analysis
}

// Show algorithm details (placeholder)
function showAlgorithmDetails(algorithmName) {
    console.log('🤖 Show algorithm details for:', algorithmName);
    showNotification(`Algorithm details for ${formatAlgorithmName(algorithmName)} - Coming soon!`, 'info');
    // TODO: Implement algorithm detail view
}

// Utility functions
function formatAlgorithmName(algorithm) {
    const nameMap = {
        'lstm_nn': 'LSTM',
        'arima_garch': 'ARIMA',
        'moving_average': 'MA',
        'lstm': 'LSTM',
        'arima': 'ARIMA',
        'ma': 'MA',
        'unknown': 'Unknown'
    };
    return nameMap[algorithm] || algorithm.toUpperCase();
}

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

function getTrendClass(trend) {
    switch(trend?.toLowerCase()) {
        case 'improving': return 'positive';
        case 'declining': return 'negative';
        default: return 'neutral';
    }
}

function getTrendIcon(trend) {
    switch(trend?.toLowerCase()) {
        case 'improving': return '↗';
        case 'declining': return '↘';
        default: return '→';
    }
}

function formatTrendText(trend) {
    return trend ? trend.charAt(0).toUpperCase() + trend.slice(1) : 'Stable';
}

function getPredictionStatusClass(prediction) {
    if (prediction.actual_price) {
        return 'confirmed';
    }
    
    const targetDate = new Date(prediction.target_date);
    const now = new Date();
    
    if (now > targetDate) {
        return 'pending';
    }
    
    return 'active';
}

function getPredictionStatusText(prediction) {
    const status = getPredictionStatusClass(prediction);
    switch(status) {
        case 'confirmed': return 'Confirmed';
        case 'pending': return 'Pending';
        case 'active': return 'Active';
        default: return 'Unknown';
    }
}

// Export functions for global access
window.refreshPredictions = refreshPredictions;
window.applyPredictionFilters = applyPredictionFilters;
window.exportPredictions = exportPredictions;
window.showPredictionDetail = showPredictionDetail;
window.showAlgorithmDetails = showAlgorithmDetails;

console.log('🎉 Predictions JavaScript ready and complete!');